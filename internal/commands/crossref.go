package commands

import (
	"fmt"
	"strings"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
)

// resolveContainerUUIDWithHint resolves a container UUID and, on failure, checks
// whether the identifier is actually a serverless endpoint or replica. Serverless
// replicas are not standalone containers, so a bare "no container found" is
// misleading; this points the user at the right serverless command instead.
//
// suggest builds the recommended command from the owning endpoint and (when the
// identifier resolved to a replica) the replica UUID.
func resolveContainerUUIDWithHint(client *api.Client, id string, suggest func(endpoint, replica string) string) (string, error) {
	uuid, err := client.ResolveContainerUUID(id)
	if err == nil {
		return uuid, nil
	}

	// Is it a serverless endpoint (UUID, key, or name)?
	if endpoint, epErr := client.ResolveServerlessService(id); epErr == nil {
		return "", fmt.Errorf("%q is a serverless endpoint, not a container.\nServerless endpoints run replicas — try:\n  %s", id, suggest(endpoint, ""))
	}

	// Is it a serverless replica of some endpoint?
	if endpoint, replica, found := findServerlessReplicaOwner(client, id); found {
		return "", fmt.Errorf("%q is a serverless replica of endpoint %s, not a standalone container.\nTry:\n  %s", id, shortID(endpoint), suggest(endpoint, replica))
	}

	return "", err
}

// findServerlessReplicaOwner scans the caller's serverless endpoints for a
// replica matching id (full UUID, UUID prefix, or server name). It is only
// invoked on the error path, so the extra per-endpoint lookups are acceptable.
func findServerlessReplicaOwner(client *api.Client, id string) (endpoint, replica string, found bool) {
	services, err := client.ListServerlessServices()
	if err != nil {
		return "", "", false
	}
	for _, svc := range services {
		detail, err := client.GetServerlessService(svc.UUID)
		if err != nil {
			continue
		}
		for _, r := range detail.Replicas {
			if r.UUID == id ||
				strings.HasPrefix(r.UUID, id) ||
				strings.EqualFold(r.ServerName, id) ||
				strings.HasPrefix(r.ServerName, id) {
				return svc.UUID, r.UUID, true
			}
		}
	}
	return "", "", false
}

// serverlessRestartSuggestion is the suggestion builder for restart/stop/start/
// redeploy container actions.
func serverlessRestartSuggestion(endpoint, replica string) string {
	if replica == "" {
		return fmt.Sprintf("gpulab serverless restart %s --all", shortID(endpoint))
	}
	return fmt.Sprintf("gpulab serverless restart %s %s", shortID(endpoint), shortID(replica))
}

// serverlessLogsSuggestion is the suggestion builder for the logs command.
func serverlessLogsSuggestion(endpoint, replica string) string {
	if replica == "" {
		return fmt.Sprintf("gpulab serverless logs %s", shortID(endpoint))
	}
	return fmt.Sprintf("gpulab serverless logs %s --replica %s", shortID(endpoint), shortID(replica))
}
