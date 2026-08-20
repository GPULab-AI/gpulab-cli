package commands

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	serverlessName             string
	serverlessTemplate         string
	serverlessGPUType          string
	serverlessRegion           string
	serverlessVolume           string
	serverlessGPUCount         int
	serverlessMemory           int
	serverlessPort             int
	serverlessHealthPath       string
	serverlessMinReplicas      int
	serverlessMaxReplicas      int
	serverlessConcurrency      int
	serverlessOverflow         bool
	serverlessAutoscaling      bool
	serverlessAutoscalingTpl   string
	serverlessPolicy           string
	serverlessPolicyFile       string
	serverlessMetricsJSON      string
	serverlessMetricsFile      string
	serverlessIdleTimeout      int
	serverlessColdStartTimeout int
	serverlessRequestTimeout   int
	serverlessPullPolicy       string
	serverlessCUDA             string
	serverlessEnv              []string
	serverlessEnvFile          string
	serverlessCommand          string
	serverlessNotes            string
	serverlessEnable           bool
	serverlessDisable          bool
	serverlessForceDelete      bool
	serverlessPage             int
	serverlessPerPage          int
	serverlessAllPages         bool
	serverlessDetails          bool
	serverlessReplica          string
	serverlessDeployLogs       bool
	serverlessFollowLogs       bool
	serverlessLogTail          int
	serverlessLogTimestamps    bool
	serverlessLogSince         string
	serverlessLogGrep          string
	serverlessLogDetect        bool
	serverlessInvokeMethod     string
	serverlessInvokeData       string
	serverlessInvokeDataFile   string
	serverlessInvokeHeader     []string
	serverlessInvokeWait       bool
	serverlessInvokeTimeout    int
	serverlessWatchStatus      bool
	serverlessReplicaForce     bool
	serverlessRestartAll       bool
	serverlessRestartRolling   bool
	serverlessRestartDrain     bool
	serverlessRestartWait      int
	serverlessReplicaDrain     bool
	serverlessListVolume       string
	serverlessListWithPolicy   bool
	serverlessReplicasStale    bool
	serverlessStaleDepth       int
	serverlessStaleCodePath    string
)

func init() {
	rootCmd.AddCommand(serverlessCmd)

	serverlessCmd.AddCommand(serverlessListCmd)
	serverlessCmd.AddCommand(serverlessCreateCmd)
	serverlessCmd.AddCommand(serverlessInspectCmd)
	serverlessCmd.AddCommand(serverlessUpdateCmd)
	serverlessCmd.AddCommand(serverlessDeleteCmd)
	serverlessCmd.AddCommand(serverlessOptionsCmd)
	serverlessCmd.AddCommand(serverlessReplicasCmd)
	serverlessCmd.AddCommand(serverlessRestartCmd)
	serverlessCmd.AddCommand(serverlessDeleteReplicaCmd)
	serverlessCmd.AddCommand(serverlessRequestsCmd)
	serverlessCmd.AddCommand(serverlessAutoscalingLogsCmd)
	serverlessCmd.AddCommand(serverlessLogsCmd)
	serverlessCmd.AddCommand(serverlessCancelRequestCmd)
	serverlessCmd.AddCommand(serverlessValidatePolicyCmd)
	serverlessCmd.AddCommand(serverlessStatusCmd)
	serverlessCmd.AddCommand(serverlessInvokeCmd)

	addServerlessWriteFlags(serverlessCreateCmd, true)
	addServerlessWriteFlags(serverlessUpdateCmd, false)

	serverlessListCmd.Flags().StringVar(&serverlessListVolume, "volume", "", "Only show endpoints using this network volume (UUID, name, or ID)")
	serverlessListCmd.Flags().BoolVar(&serverlessListWithPolicy, "with-policy", false, "Load each endpoint's autoscaling policy, which the list API does not return")

	serverlessReplicasCmd.Flags().BoolVar(&serverlessReplicasStale, "stale", false, "Flag replicas started before the volume's newest code file (running old code)")
	serverlessReplicasCmd.Flags().IntVar(&serverlessStaleDepth, "depth", 2, "Directory depth to scan for the volume's newest file when --stale")
	serverlessReplicasCmd.Flags().StringVar(&serverlessStaleCodePath, "code-path", "", "Scope the --stale scan to this volume subdirectory (e.g. server)")

	// Reverse lookup lives under `volumes` for discoverability, but shares the
	// serverless volume-matching logic defined here.
	volumesCmd.AddCommand(volumesEndpointsCmd)

	serverlessDeleteCmd.Flags().BoolVar(&serverlessForceDelete, "force", false, "Skip confirmation")

	serverlessRequestsCmd.Flags().IntVar(&serverlessPage, "page", 1, "Page number")
	serverlessRequestsCmd.Flags().IntVar(&serverlessPerPage, "per-page", 25, "Rows per page")
	serverlessRequestsCmd.Flags().BoolVar(&serverlessAllPages, "all", false, "Fetch every page instead of just one")
	serverlessRequestsCmd.Flags().BoolVar(&serverlessDetails, "details", false, "Print request/response bodies in human output")

	serverlessAutoscalingLogsCmd.Flags().IntVar(&serverlessPage, "page", 1, "Page number")
	serverlessAutoscalingLogsCmd.Flags().IntVar(&serverlessPerPage, "per-page", 25, "Rows per page")
	serverlessAutoscalingLogsCmd.Flags().BoolVar(&serverlessAllPages, "all", false, "Fetch every page instead of just one")
	serverlessAutoscalingLogsCmd.Flags().BoolVar(&serverlessDetails, "details", false, "Print metrics and context in human output")

	serverlessRestartCmd.Flags().BoolVar(&serverlessRestartAll, "all", false, "Restart every replica")
	serverlessRestartCmd.Flags().BoolVar(&serverlessRestartRolling, "rolling", false, "Restart one replica at a time, waiting for each replacement (needs min-replicas >= 1)")
	serverlessRestartCmd.Flags().BoolVar(&serverlessRestartDrain, "drain", false, "Finish in-flight requests before tearing each replica down")
	serverlessRestartCmd.Flags().IntVar(&serverlessRestartWait, "wait-timeout", 300, "Seconds to wait for each replacement replica when --rolling")

	serverlessDeleteReplicaCmd.Flags().BoolVar(&serverlessReplicaForce, "force", false, "Skip confirmation")
	serverlessDeleteReplicaCmd.Flags().BoolVar(&serverlessReplicaDrain, "drain", false, "Finish in-flight requests before tearing the replica down")

	serverlessLogsCmd.Flags().StringVar(&serverlessReplica, "replica", "", "Replica UUID or prefix (default: first running replica, use 'all' for every replica)")
	serverlessLogsCmd.Flags().BoolVar(&serverlessDeployLogs, "deploy", false, "Show deployment logs instead of runtime logs")
	serverlessLogsCmd.Flags().BoolVarP(&serverlessFollowLogs, "follow", "f", false, "Follow logs")
	serverlessLogsCmd.Flags().IntVarP(&serverlessLogTail, "tail", "n", 100, "Number of runtime log lines from the end")
	serverlessLogsCmd.Flags().BoolVarP(&serverlessLogTimestamps, "timestamps", "t", false, "Show runtime log timestamps")
	serverlessLogsCmd.Flags().StringVar(&serverlessLogSince, "since", "", "Only show logs since a relative time (10m, 1h, 2d) or Unix timestamp")
	serverlessLogsCmd.Flags().StringVar(&serverlessLogGrep, "grep", "", "Only show log lines matching this regular expression")
	serverlessLogsCmd.Flags().BoolVar(&serverlessLogDetect, "detect", false, "After the logs, summarize detected tracebacks/OOM/errors")

	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessAutoscalingTpl, "template", "", "Autoscaling template key")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessPolicy, "policy", "", "Autoscaling policy code")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessPolicyFile, "policy-file", "", "Read autoscaling policy code from file")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessMetricsJSON, "metrics-json", "", "Autoscaling metrics JSON")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessMetricsFile, "metrics-file", "", "Read autoscaling metrics JSON from file")

	serverlessStatusCmd.Flags().BoolVarP(&serverlessWatchStatus, "watch", "w", false, "Poll until the request is complete")

	serverlessInvokeCmd.Flags().StringVarP(&serverlessInvokeMethod, "method", "X", "POST", "HTTP method")
	serverlessInvokeCmd.Flags().StringVarP(&serverlessInvokeData, "data", "d", "", "Request body")
	serverlessInvokeCmd.Flags().StringVar(&serverlessInvokeDataFile, "data-file", "", "Read request body from file")
	serverlessInvokeCmd.Flags().StringArrayVarP(&serverlessInvokeHeader, "header", "H", nil, "Request header (repeatable, e.g. -H 'Content-Type: application/json')")
	serverlessInvokeCmd.Flags().BoolVar(&serverlessInvokeWait, "wait", false, "Poll queued async requests until complete")
	serverlessInvokeCmd.Flags().IntVar(&serverlessInvokeTimeout, "timeout", 600, "Wait timeout in seconds")
}

func addServerlessWriteFlags(cmd *cobra.Command, create bool) {
	cmd.Flags().StringVar(&serverlessName, "name", "", "Endpoint name")
	cmd.Flags().StringVar(&serverlessTemplate, "template", "", "Template ID, UUID, or name")
	cmd.Flags().StringVar(&serverlessGPUType, "gpu-type", "", "GPU type ID or name")
	cmd.Flags().StringVar(&serverlessRegion, "region", "", "Region ID, slug, or name")
	cmd.Flags().StringVar(&serverlessVolume, "volume", "", "Network volume ID, UUID, or name")
	cmd.Flags().IntVar(&serverlessGPUCount, "gpu-count", 1, "GPUs per replica")
	cmd.Flags().IntVar(&serverlessMemory, "memory", 32, "Memory per replica in GB")
	cmd.Flags().IntVar(&serverlessPort, "port", 8000, "Replica endpoint port")
	cmd.Flags().StringVar(&serverlessHealthPath, "health-path", "/health", "Health check path")
	cmd.Flags().IntVar(&serverlessMinReplicas, "min-replicas", 0, "Minimum warm replicas")
	cmd.Flags().IntVar(&serverlessMaxReplicas, "max-replicas", 1, "Maximum replicas")
	cmd.Flags().IntVar(&serverlessConcurrency, "concurrency", 1, "Max concurrent requests per replica")
	cmd.Flags().BoolVar(&serverlessOverflow, "overflow", false, "Allow overflow routing to another warm replica on 5xx")
	cmd.Flags().BoolVar(&serverlessAutoscaling, "autoscaling", false, "Enable custom autoscaling")
	cmd.Flags().StringVar(&serverlessAutoscalingTpl, "autoscaling-template", "", "Autoscaling template key")
	cmd.Flags().StringVar(&serverlessPolicy, "policy", "", "Autoscaling policy code")
	cmd.Flags().StringVar(&serverlessPolicyFile, "policy-file", "", "Read autoscaling policy code from file")
	cmd.Flags().StringVar(&serverlessMetricsJSON, "metrics-json", "", "Autoscaling metrics JSON")
	cmd.Flags().StringVar(&serverlessMetricsFile, "metrics-file", "", "Read autoscaling metrics JSON from file")
	cmd.Flags().IntVar(&serverlessIdleTimeout, "idle-timeout", 60, "Idle timeout in seconds")
	cmd.Flags().IntVar(&serverlessColdStartTimeout, "cold-start-timeout", 300, "Cold start timeout in seconds")
	cmd.Flags().IntVar(&serverlessRequestTimeout, "request-timeout", 600, "Request timeout in seconds")
	cmd.Flags().StringVar(&serverlessPullPolicy, "pull-policy", "cached", "Image pull policy (cached|latest)")
	cmd.Flags().StringVar(&serverlessCUDA, "cuda", "", "CUDA version label")
	cmd.Flags().StringArrayVarP(&serverlessEnv, "env", "e", nil, "Environment variable (KEY=VALUE or KEY to inherit from host)")
	cmd.Flags().StringVar(&serverlessEnvFile, "env-file", "", "Read environment variables from file")
	cmd.Flags().StringVar(&serverlessCommand, "command", "", "Replica startup command")
	cmd.Flags().StringVar(&serverlessNotes, "notes", "", "Endpoint notes")
	cmd.Flags().BoolVar(&serverlessEnable, "enable", false, "Enable endpoint")
	cmd.Flags().BoolVar(&serverlessDisable, "disable", false, "Disable endpoint")

	if create {
		cmd.MarkFlagRequired("name")
		cmd.MarkFlagRequired("template")
		cmd.MarkFlagRequired("gpu-type")
	}
}

var serverlessCmd = &cobra.Command{
	Use:     "serverless",
	Aliases: []string{"sl"},
	Short:   "Manage serverless GPU endpoints",
	RunE:    serverlessListCmd.RunE,
}

var serverlessListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List serverless GPU endpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		services, err := client.ListServerlessServices()
		if err != nil {
			return err
		}
		if serverlessListVolume != "" {
			services = filterServicesByVolume(services, serverlessListVolume)
		}
		if serverlessListWithPolicy {
			services = hydrateServerlessPolicies(client, services)
		}
		printServerlessServices(services)
		return nil
	},
}

// serviceUsesVolume reports whether a serverless endpoint uses the given volume,
// matched by UUID (exact or prefix), name (case-insensitive), or numeric ID.
func serviceUsesVolume(service api.ServerlessService, selector string) bool {
	if selector == "" {
		return false
	}
	if service.NetworkVolumeUUID == selector ||
		strings.HasPrefix(service.NetworkVolumeUUID, selector) ||
		strings.EqualFold(service.NetworkVolumeName, selector) {
		return true
	}
	if service.NetworkVolumeID != nil && strconv.Itoa(*service.NetworkVolumeID) == selector {
		return true
	}
	return strings.Contains(strings.ToLower(service.NetworkVolumeName), strings.ToLower(selector))
}

func filterServicesByVolume(services []api.ServerlessService, selector string) []api.ServerlessService {
	var out []api.ServerlessService
	for _, service := range services {
		if serviceUsesVolume(service, selector) {
			out = append(out, service)
		}
	}
	return out
}

// volumesEndpointsCmd answers "which serverless endpoints use volume X?".
var volumesEndpointsCmd = &cobra.Command{
	Use:     "endpoints [VOLUME]",
	Aliases: []string{"used-by", "usage"},
	Short:   "List serverless endpoints that use a network volume",
	Long: `Reverse lookup: given a network volume, show every serverless endpoint that
mounts it. VOLUME may be a full UUID, a UUID prefix, or the volume name.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		selectors := []string{args[0]}
		if uuid, err := client.ResolveVolumeUUID(args[0]); err == nil && uuid != args[0] {
			selectors = append(selectors, uuid)
		}

		services, err := client.ListServerlessServices()
		if err != nil {
			return err
		}
		var matches []api.ServerlessService
		for _, service := range services {
			for _, sel := range selectors {
				if serviceUsesVolume(service, sel) {
					matches = append(matches, service)
					break
				}
			}
		}

		switch getOutputFormat() {
		case output.FormatJSON:
			output.PrintJSON(matches)
		case output.FormatQuiet:
			ids := make([]string, len(matches))
			for i, m := range matches {
				ids[i] = m.UUID
			}
			output.PrintQuiet(ids)
		default:
			if len(matches) == 0 {
				fmt.Printf("No serverless endpoints use volume %q.\n", args[0])
				return nil
			}
			printServerlessServices(matches)
		}
		return nil
	},
}

var serverlessCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a serverless GPU endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		req, err := buildServerlessRequest(cmd, client, nil)
		if err != nil {
			return err
		}
		resp, err := client.CreateServerlessService(req)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless endpoint created: %s", resp.Data.UUID))
		fmt.Printf("Name:         %s\n", resp.Data.Name)
		fmt.Printf("Endpoint key: %s\n", resp.Data.EndpointKey)
		fmt.Printf("URL:          %s\n", resp.Data.EndpointURL)
		return nil
	},
}

var serverlessInspectCmd = &cobra.Command{
	Use:   "inspect [ENDPOINT]",
	Short: "Show serverless endpoint details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(detail)
			return nil
		}
		printServerlessDetail(detail)
		return nil
	},
}

var serverlessUpdateCmd = &cobra.Command{
	Use:   "update [ENDPOINT]",
	Short: "Update a serverless GPU endpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		req, err := buildServerlessRequest(cmd, client, &detail.Data)
		if err != nil {
			return err
		}
		resp, err := client.UpdateServerlessService(identifier, req)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless endpoint updated: %s", resp.Data.UUID))
		fmt.Printf("Name: %s\n", resp.Data.Name)
		fmt.Printf("URL:  %s\n", resp.Data.EndpointURL)
		return nil
	},
}

var serverlessDeleteCmd = &cobra.Command{
	Use:     "delete [ENDPOINT]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a serverless GPU endpoint and its replicas",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		if !serverlessForceDelete {
			fmt.Printf("Delete serverless endpoint %s and all warm replicas? [y/N] ", shortID(identifier))
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := client.DeleteServerlessService(identifier); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "uuid": identifier})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless endpoint deleted: %s", shortID(identifier)))
		return nil
	},
}

var serverlessOptionsCmd = &cobra.Command{
	Use:   "options",
	Short: "List serverless templates, GPU types, regions, volumes, and autoscaling templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		options, err := client.GetServerlessOptions()
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(options)
			return nil
		}
		printServerlessOptions(options)
		return nil
	},
}

var serverlessReplicasCmd = &cobra.Command{
	Use:   "replicas [ENDPOINT]",
	Short: "List replicas for a serverless endpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}

		var stale map[string]*staleResult
		if serverlessReplicasStale {
			stale, err = computeReplicaStaleness(client, detail.Data.NetworkVolumeUUID, serverlessStaleCodePath, detail.Replicas, serverlessStaleDepth)
			if err != nil {
				return fmt.Errorf("stale check failed: %w", err)
			}
		}

		if flagJSON {
			output.PrintJSON(enrichReplicasWithStale(detail.Replicas, stale))
			return nil
		}
		printServerlessReplicas(detail.Replicas, stale)
		return nil
	},
}

var serverlessRestartCmd = &cobra.Command{
	Use:     "restart [ENDPOINT] [REPLICA]",
	Aliases: []string{"restart-replica"},
	Short:   "Restart serverless replicas (single, rolling, or all)",
	Long: `Restart one or more replicas of a serverless endpoint.

Each restart tears the replica down and brings a fresh one back up to the
warm-pool minimum, so it reloads the latest volume code. (This is different from
the container-level /restart, which does NOT reload code.) Pass a REPLICA to
restart just that one, or --all to restart every replica. With --rolling the
replicas are restarted one at a time, waiting for each replacement to come up
before the next teardown, so there's no full outage when min-replicas >= 1.

REPLICA may be a full UUID, a UUID prefix, or a server name (see 'serverless
replicas <endpoint>').`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}

		var targets []api.ServerlessReplica
		if serverlessRestartAll {
			if len(args) == 2 {
				return fmt.Errorf("pass either a REPLICA or --all, not both")
			}
			targets = detail.Replicas
		} else {
			if len(args) < 2 {
				return fmt.Errorf("specify a replica to restart, or pass --all to restart every replica\n\n%s", replicaListHint(detail.Replicas))
			}
			replicaUUID, err := resolveServerlessReplicaUUID(client, identifier, args[1])
			if err != nil {
				return err
			}
			for _, r := range detail.Replicas {
				if r.UUID == replicaUUID {
					targets = append(targets, r)
					break
				}
			}
		}

		if len(targets) == 0 {
			if flagJSON {
				output.PrintJSON(map[string]interface{}{"endpoint": identifier, "restarted": []interface{}{}})
				return nil
			}
			fmt.Println("No replicas to restart.")
			return nil
		}

		return restartServerlessReplicas(client, identifier, targets, detail.Data.MinReplicas, serverlessRestartRolling, serverlessRestartDrain, serverlessRestartWait)
	},
}

// replicaListHint renders a short list of replicas for error messages.
func replicaListHint(replicas []api.ServerlessReplica) string {
	if len(replicas) == 0 {
		return "This endpoint has no replicas right now."
	}
	var b strings.Builder
	b.WriteString("replicas:\n")
	for _, r := range replicas {
		fmt.Fprintf(&b, "  %s  %s  %s\n", shortID(r.UUID), r.ServerName, r.Status)
	}
	return b.String()
}

// restartServerlessReplicas restarts the given replicas, optionally rolling
// (waiting for each replacement to come up) and/or draining in-flight requests.
func restartServerlessReplicas(client *api.Client, identifier string, targets []api.ServerlessReplica, minReplicas int, rolling, drain bool, waitTimeout int) error {
	// Pre-compute the batch's UUIDs so rolling waits detect a genuinely new
	// replacement replica rather than a sibling that is still up.
	batch := make(map[string]bool, len(targets))
	for _, r := range targets {
		batch[r.UUID] = true
	}

	results := make([]map[string]interface{}, 0, len(targets))
	for i, r := range targets {
		if !flagJSON {
			label := r.ServerName
			if label == "" {
				label = shortID(r.UUID)
			}
			fmt.Fprintf(os.Stderr, "Restarting replica %d/%d: %s (%s)...\n", i+1, len(targets), label, shortID(r.UUID))
		}
		resp, err := client.RestartServerlessReplica(identifier, r.UUID, drain)
		if err != nil {
			if flagJSON {
				results = append(results, map[string]interface{}{"uuid": r.UUID, "status": "error", "error": err.Error()})
				continue
			}
			return fmt.Errorf("failed to restart replica %s: %w", shortID(r.UUID), err)
		}
		results = append(results, map[string]interface{}{
			"uuid":                    r.UUID,
			"status":                  "restarted",
			"replacement_provisioned": resp.ReplacementProvisioned,
		})

		// Roll: wait for a replacement before tearing down the next one. Only
		// meaningful when the warm pool re-provisions (min-replicas >= 1).
		if rolling && minReplicas > 0 && i < len(targets)-1 {
			if !flagJSON {
				fmt.Fprintf(os.Stderr, "  waiting for a replacement replica...\n")
			}
			if err := waitForNewRunningReplica(client, identifier, batch, waitTimeout); err != nil && !flagJSON {
				fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			}
		}
	}

	if flagJSON {
		output.PrintJSON(map[string]interface{}{"endpoint": identifier, "restarted": results})
		return nil
	}
	output.PrintSuccess(fmt.Sprintf("Restarted %d replica(s) of %s", len(targets), shortID(identifier)))
	return nil
}

// waitForNewRunningReplica polls until a running replica appears whose UUID is
// not in exclude (the restart batch), i.e. a freshly provisioned replacement.
func waitForNewRunningReplica(client *api.Client, identifier string, exclude map[string]bool, timeoutSecs int) error {
	deadline := time.Now().Add(time.Duration(timeoutSecs) * time.Second)
	for {
		detail, err := client.GetServerlessService(identifier)
		if err == nil {
			for _, r := range detail.Replicas {
				if exclude[r.UUID] {
					continue
				}
				switch strings.ToLower(r.Status) {
				case "running", "healthy":
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %ds waiting for a replacement replica", timeoutSecs)
		}
		time.Sleep(3 * time.Second)
	}
}

var serverlessDeleteReplicaCmd = &cobra.Command{
	Use:     "delete-replica [ENDPOINT] [REPLICA]",
	Aliases: []string{"rm-replica"},
	Short:   "Delete a single replica of a serverless endpoint",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		replicaUUID, err := resolveServerlessReplicaUUID(client, identifier, args[1])
		if err != nil {
			return err
		}
		if !serverlessReplicaForce && !flagJSON && !flagQuiet {
			fmt.Printf("Delete replica %s of %s? [y/N] ", shortID(replicaUUID), shortID(identifier))
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := client.DeleteServerlessReplica(identifier, replicaUUID, serverlessReplicaDrain); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "uuid": replicaUUID})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Replica deleted: %s", shortID(replicaUUID)))
		return nil
	},
}

var serverlessRequestsCmd = &cobra.Command{
	Use:     "requests [ENDPOINT]",
	Aliases: []string{"request-logs"},
	Short:   "Show serverless request logs",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		var page *api.ServerlessPage[api.ServerlessRequestLog]
		if serverlessAllPages {
			page, err = fetchAllServerlessPages(func(p int) (*api.ServerlessPage[api.ServerlessRequestLog], error) {
				return client.ListServerlessRequests(identifier, p, serverlessPerPage)
			})
		} else {
			page, err = client.ListServerlessRequests(identifier, serverlessPage, serverlessPerPage)
		}
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(page)
			return nil
		}
		printServerlessRequests(page, serverlessDetails)
		return nil
	},
}

var serverlessAutoscalingLogsCmd = &cobra.Command{
	Use:     "autoscaling-logs [ENDPOINT]",
	Aliases: []string{"autoscaling-history", "history"},
	Short:   "Show serverless autoscaling logs and history",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		var page *api.ServerlessPage[api.ServerlessAutoscalingLog]
		if serverlessAllPages {
			page, err = fetchAllServerlessPages(func(p int) (*api.ServerlessPage[api.ServerlessAutoscalingLog], error) {
				return client.ListServerlessAutoscalingLogs(identifier, p, serverlessPerPage)
			})
		} else {
			page, err = client.ListServerlessAutoscalingLogs(identifier, serverlessPage, serverlessPerPage)
		}
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(page)
			return nil
		}
		printAutoscalingSummary(computeAutoscalingStats(page))
		printServerlessAutoscalingLogs(page, serverlessDetails)
		return nil
	},
}

var serverlessLogsCmd = &cobra.Command{
	Use:   "logs [ENDPOINT]",
	Short: "Show serverless replica container logs",
	Long: `Show container logs for a serverless endpoint's replicas.

By default the first running replica is used. Pass --replica <uuid|prefix|name>
to target a specific replica, or --replica all to aggregate every replica's
logs (each block is prefixed with its replica). Logs are de-framed and cleaned
automatically, so no piping through 'strings' is needed.

Examples:
  gpulab serverless logs my-endpoint                 # first running replica
  gpulab serverless logs my-endpoint --replica all   # every replica
  gpulab serverless logs my-endpoint --since 10m     # last 10 minutes
  gpulab serverless logs my-endpoint --grep Traceback
  gpulab serverless logs my-endpoint -f              # follow`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		replicas, err := selectServerlessLogReplicas(detail.Replicas, serverlessReplica, serverlessDeployLogs)
		if err != nil {
			return err
		}
		sinceTS, err := output.ParseSince(serverlessLogSince)
		if err != nil {
			return err
		}
		if !serverlessFollowLogs {
			return printServerlessReplicaLogs(client, replicas, serverlessDeployLogs, serverlessLogTail, sinceTS, serverlessLogTimestamps, serverlessLogGrep, serverlessLogDetect)
		}
		since := sinceTS
		if since == "" {
			since = fmt.Sprintf("%d", time.Now().Unix())
		}
		for {
			if err := printServerlessReplicaLogs(client, replicas, serverlessDeployLogs, 0, since, serverlessLogTimestamps, serverlessLogGrep, false); err != nil && flagDebug {
				fmt.Fprintf(os.Stderr, "[DEBUG] log poll failed: %v\n", err)
			}
			since = fmt.Sprintf("%d", time.Now().Unix())
			time.Sleep(2 * time.Second)
		}
	},
}

var serverlessCancelRequestCmd = &cobra.Command{
	Use:   "cancel-request [REQUEST_UUID]",
	Short: "Cancel a queued or provisioning serverless request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		if err := client.CancelServerlessRequest(args[0]); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "request_uuid": args[0]})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless request cancelled: %s", shortID(args[0])))
		return nil
	},
}

var serverlessValidatePolicyCmd = &cobra.Command{
	Use:   "validate-policy",
	Short: "Validate serverless autoscaling policy code",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		policy, metrics, err := resolveServerlessPolicyInputs(client, serverlessAutoscalingTpl)
		if err != nil {
			return err
		}
		resp, err := client.ValidateServerlessPolicy(&api.ServerlessPolicyValidationRequest{
			AutoscalingTemplateKey:   serverlessAutoscalingTpl,
			AutoscalingPolicyCode:    policy,
			AutoscalingMetricsConfig: metrics,
		})
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}
		output.PrintSuccess(resp.Message)
		fmt.Printf("Policy hash:   %s\n", resp.PolicyHash)
		fmt.Printf("Metric count:  %d\n", resp.MetricsCount)
		return nil
	},
}

var serverlessStatusCmd = &cobra.Command{
	Use:   "status [REQUEST_UUID]",
	Short: "Show public serverless request status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		for {
			resp, err := client.GetServerlessRequestStatus(args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				output.PrintJSON(resp)
			} else {
				printServerlessStatus(resp)
			}
			if !serverlessWatchStatus || !resp.Processing {
				return nil
			}
			time.Sleep(3 * time.Second)
		}
	},
}

var serverlessInvokeCmd = &cobra.Command{
	Use:   "invoke [ENDPOINT] [PATH]",
	Short: "Invoke a serverless endpoint",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		target, err := resolveInvokeURL(client, args)
		if err != nil {
			return err
		}
		resp, body, err := invokeServerlessURL(cmd.Context(), target)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if flagJSON {
			output.PrintJSON(map[string]interface{}{
				"status_code": resp.StatusCode,
				"headers":     resp.Header,
				"body":        string(body),
			})
		} else {
			fmt.Print(string(body))
			if len(body) > 0 && body[len(body)-1] != '\n' {
				fmt.Println()
			}
		}

		if serverlessInvokeWait && resp.StatusCode == http.StatusAccepted {
			requestID := parseRequestID(body)
			if requestID == "" {
				return nil
			}
			return waitServerlessRequest(client, requestID, time.Duration(serverlessInvokeTimeout)*time.Second)
		}
		return nil
	},
}

func buildServerlessRequest(cmd *cobra.Command, client *api.Client, existing *api.ServerlessService) (*api.ServerlessServiceRequest, error) {
	req := &api.ServerlessServiceRequest{
		IsEnabled:               true,
		GPUCount:                1,
		Memory:                  32,
		EndpointPort:            8000,
		HealthCheckPath:         "/health",
		MinReplicas:             0,
		MaxReplicas:             1,
		MaxConcurrentRequests:   1,
		IdleTimeoutSeconds:      60,
		ColdStartTimeoutSeconds: 300,
		RequestTimeoutSeconds:   600,
		ImagePullPolicy:         "cached",
	}

	if existing != nil {
		fromExisting, err := serverlessRequestFromService(*existing)
		if err != nil {
			return nil, err
		}
		req = fromExisting
	}

	setStringFlag(cmd, "name", &req.Name, serverlessName)
	setStringFlag(cmd, "template", &req.Template, serverlessTemplate)
	setStringFlag(cmd, "gpu-type", &req.GPUType, serverlessGPUType)
	setStringFlag(cmd, "region", &req.Region, serverlessRegion)
	setStringFlag(cmd, "volume", &req.NetworkVolume, serverlessVolume)
	setIntFlag(cmd, "gpu-count", &req.GPUCount, serverlessGPUCount)
	setIntFlag(cmd, "memory", &req.Memory, serverlessMemory)
	setIntFlag(cmd, "port", &req.EndpointPort, serverlessPort)
	setStringFlag(cmd, "health-path", &req.HealthCheckPath, serverlessHealthPath)
	setIntFlag(cmd, "min-replicas", &req.MinReplicas, serverlessMinReplicas)
	setIntFlag(cmd, "max-replicas", &req.MaxReplicas, serverlessMaxReplicas)
	setIntFlag(cmd, "concurrency", &req.MaxConcurrentRequests, serverlessConcurrency)
	setBoolFlag(cmd, "overflow", &req.AllowOverflowRequests, serverlessOverflow)
	setBoolFlag(cmd, "autoscaling", &req.AutoscalingEnabled, serverlessAutoscaling)
	setStringFlag(cmd, "autoscaling-template", &req.AutoscalingTemplateKey, serverlessAutoscalingTpl)
	setIntFlag(cmd, "idle-timeout", &req.IdleTimeoutSeconds, serverlessIdleTimeout)
	setIntFlag(cmd, "cold-start-timeout", &req.ColdStartTimeoutSeconds, serverlessColdStartTimeout)
	setIntFlag(cmd, "request-timeout", &req.RequestTimeoutSeconds, serverlessRequestTimeout)
	setStringFlag(cmd, "pull-policy", &req.ImagePullPolicy, serverlessPullPolicy)
	setStringFlag(cmd, "cuda", &req.CUDAVersion, serverlessCUDA)
	setStringFlag(cmd, "command", &req.Command, serverlessCommand)
	setStringFlag(cmd, "notes", &req.Notes, serverlessNotes)

	if cmd.Flags().Changed("enable") {
		req.IsEnabled = true
	}
	if cmd.Flags().Changed("disable") {
		req.IsEnabled = false
	}
	if serverlessEnable && serverlessDisable {
		return nil, fmt.Errorf("--enable and --disable cannot be used together")
	}

	if req.Name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if req.Template == "" && req.TemplateID == 0 {
		return nil, fmt.Errorf("--template is required")
	}
	if req.GPUType == "" && req.GPUTypeID == 0 {
		return nil, fmt.Errorf("--gpu-type is required")
	}

	if req.Template != "" || req.GPUType != "" || req.Region != "" || req.NetworkVolume != "" {
		options, err := client.GetServerlessOptions()
		if err != nil {
			return nil, err
		}
		if req.Template != "" {
			template, err := resolveServerlessTemplateOption(options.Templates, req.Template)
			if err != nil {
				return nil, err
			}
			req.TemplateID = template.ID
			req.Template = ""
		}
		if req.GPUType != "" {
			gpuType, err := resolveServerlessGPUTypeOption(options.GPUTypes, req.GPUType)
			if err != nil {
				return nil, err
			}
			req.GPUTypeID = gpuType.ID
			req.GPUType = ""
		}
		if req.Region != "" {
			region, err := resolveServerlessRegionOption(options.Regions, req.Region)
			if err != nil {
				return nil, err
			}
			req.RegionID = region.ID
			req.Region = ""
		}
		if req.NetworkVolume != "" {
			volume, err := resolveServerlessVolumeOption(options.NetworkVolumes, req.NetworkVolume)
			if err != nil {
				return nil, err
			}
			id := volume.ID
			req.NetworkVolumeID = &id
			req.NetworkVolume = ""
		}
	}

	if cmd.Flags().Changed("env") || cmd.Flags().Changed("env-file") {
		envVars, err := buildEnvVars(serverlessEnvFile, serverlessEnv)
		if err != nil {
			return nil, err
		}
		req.EnvironmentVariables = envVars
	}

	if cmd.Flags().Changed("policy") || cmd.Flags().Changed("policy-file") || cmd.Flags().Changed("metrics-json") ||
		cmd.Flags().Changed("metrics-file") || cmd.Flags().Changed("autoscaling-template") {
		policy, metrics, err := resolveServerlessPolicyInputs(client, req.AutoscalingTemplateKey)
		if err != nil {
			return nil, err
		}
		req.AutoscalingPolicyCode = policy
		req.AutoscalingMetricsConfig = metrics
		if policy != "" {
			req.AutoscalingEnabled = true
		}
	}

	return req, nil
}

func resolveServerlessTemplateOption(options []api.ServerlessTemplateOption, value string) (*api.ServerlessTemplateOption, error) {
	var matches []api.ServerlessTemplateOption
	for _, option := range options {
		if strconv.Itoa(option.ID) == value || option.TemplateUUID == value || strings.EqualFold(option.Name, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.Name), strings.ToLower(value)) || strings.HasPrefix(option.TemplateUUID, value) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless template %q not found", value)
	}
	return nil, fmt.Errorf("serverless template %q is ambiguous", value)
}

func resolveServerlessGPUTypeOption(options []api.ServerlessGPUTypeOption, value string) (*api.ServerlessGPUTypeOption, error) {
	var matches []api.ServerlessGPUTypeOption
	for _, option := range options {
		if strconv.Itoa(option.ID) == value || strings.EqualFold(option.GPUName, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.GPUName), strings.ToLower(value)) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless GPU type %q not found", value)
	}
	return nil, fmt.Errorf("serverless GPU type %q is ambiguous", value)
}

func resolveServerlessRegionOption(options []api.ServerlessRegionOption, value string) (*api.ServerlessRegionOption, error) {
	var matches []api.ServerlessRegionOption
	for _, option := range options {
		if option.ID == value || option.Slug == value || strings.EqualFold(option.Name, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.Name), strings.ToLower(value)) || strings.HasPrefix(option.ID, value) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless region %q not found", value)
	}
	return nil, fmt.Errorf("serverless region %q is ambiguous", value)
}

func resolveServerlessVolumeOption(options []api.ServerlessNetworkVolumeOption, value string) (*api.ServerlessNetworkVolumeOption, error) {
	var matches []api.ServerlessNetworkVolumeOption
	for _, option := range options {
		if strconv.Itoa(option.ID) == value || option.VolumeUUID == value || strings.EqualFold(option.VolumeName, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.VolumeName), strings.ToLower(value)) || strings.HasPrefix(option.VolumeUUID, value) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless network volume %q not found", value)
	}
	return nil, fmt.Errorf("serverless network volume %q is ambiguous", value)
}

// serverlessRequestFromService turns an existing endpoint into a full update
// request, so that `serverless update` preserves every field the caller did not
// pass a flag for. It requires a service loaded from the detail endpoint: the
// list payload leaves the configuration block out, and copying those blanks
// into an update would erase the endpoint's autoscaling policy, command, and
// notes.
func serverlessRequestFromService(service api.ServerlessService) (*api.ServerlessServiceRequest, error) {
	if !service.AutoscalingPolicyCode.Known() {
		return nil, fmt.Errorf("internal error: endpoint %q was loaded without its configuration; "+
			"updating from it would erase the autoscaling policy", service.Name)
	}
	req := &api.ServerlessServiceRequest{
		Name:                     service.Name,
		IsEnabled:                service.IsEnabled,
		TemplateID:               service.TemplateID,
		GPUTypeID:                service.GPUTypeID,
		RegionID:                 service.RegionID,
		GPUCount:                 service.GPUCount,
		Memory:                   service.Memory,
		EndpointPort:             service.EndpointPort,
		HealthCheckPath:          service.HealthCheckPath,
		MinReplicas:              service.MinReplicas,
		MaxReplicas:              service.MaxReplicas,
		MaxConcurrentRequests:    service.MaxConcurrentRequests,
		AllowOverflowRequests:    service.AllowOverflowRequests,
		AutoscalingEnabled:       service.AutoscalingEnabled,
		AutoscalingTemplateKey:   service.AutoscalingTemplateKey.String(),
		AutoscalingPolicyCode:    service.AutoscalingPolicyCode.String(),
		AutoscalingMetricsConfig: service.AutoscalingMetricsConfig.String(),
		IdleTimeoutSeconds:       service.IdleTimeoutSeconds,
		ColdStartTimeoutSeconds:  service.ColdStartTimeoutSeconds,
		RequestTimeoutSeconds:    service.RequestTimeoutSeconds,
		ImagePullPolicy:          service.ImagePullPolicy,
		CUDAVersion:              service.CUDAVersion.String(),
		EnvironmentVariables:     service.EnvironmentVariables.String(),
		Command:                  service.Command.String(),
		Notes:                    service.Notes.String(),
	}
	if service.NetworkVolumeID != nil {
		v := *service.NetworkVolumeID
		req.NetworkVolumeID = &v
	}
	return req, nil
}

func resolveServerlessPolicyInputs(client *api.Client, templateKey string) (string, interface{}, error) {
	policy := serverlessPolicy
	if serverlessPolicyFile != "" {
		data, err := os.ReadFile(serverlessPolicyFile)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read policy file: %w", err)
		}
		policy = string(data)
	}

	var metrics interface{} = serverlessMetricsJSON
	if serverlessMetricsFile != "" {
		data, err := os.ReadFile(serverlessMetricsFile)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read metrics file: %w", err)
		}
		metrics = string(data)
	}

	if templateKey != "" && (policy == "" || metrics == "") {
		options, err := client.GetServerlessOptions()
		if err != nil {
			return "", nil, err
		}
		for _, template := range options.AutoscalingTemplates {
			if template.Key == templateKey {
				if policy == "" {
					policy = template.PolicyCode
				}
				if metrics == "" {
					metrics = template.MetricsConfig
				}
				break
			}
		}
	}

	if policy == "" {
		return "", nil, fmt.Errorf("autoscaling policy is required (use --policy, --policy-file, or --autoscaling-template)")
	}
	return policy, metrics, nil
}

// fetchAllServerlessPages walks every page of a paginated serverless endpoint
// and returns a single page containing all rows, so `--all` shows everything.
func fetchAllServerlessPages[T any](fetch func(page int) (*api.ServerlessPage[T], error)) (*api.ServerlessPage[T], error) {
	first, err := fetch(1)
	if err != nil {
		return nil, err
	}
	for p := 2; p <= first.LastPage; p++ {
		next, err := fetch(p)
		if err != nil {
			return nil, err
		}
		first.Data = append(first.Data, next.Data...)
	}
	first.CurrentPage = 1
	first.LastPage = 1
	first.HasNextPage = false
	first.HasPrevPage = false
	return first, nil
}

// hydrateServerlessPolicies fills in the configuration block that the list API
// leaves out, by loading each endpoint's detail concurrently. An endpoint whose
// detail fails to load keeps its unknown fields, so the caller still reports it
// as unknown rather than as having no policy.
func hydrateServerlessPolicies(client *api.Client, services []api.ServerlessService) []api.ServerlessService {
	const workers = 8

	out := make([]api.ServerlessService, len(services))
	copy(out, services)

	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				detail, err := client.GetServerlessService(out[i].UUID)
				if err != nil {
					continue
				}
				out[i] = detail.Data
			}
		}()
	}
	for i := range out {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}

// scalingCell states an endpoint's autoscaling policy in one table cell. It
// never renders an unreported policy as "none": an operator who reads "none"
// goes and attaches a policy that is already attached.
func scalingCell(service api.ServerlessService) string {
	if !service.AutoscalingEnabled {
		return "off"
	}
	switch {
	case service.AutoscalingTemplateKey.Set():
		return "on:" + service.AutoscalingTemplateKey.String()
	case service.AutoscalingPolicyCode.Set():
		return "on:custom"
	case service.AutoscalingPolicyCode.Known() || service.AutoscalingTemplateKey.Known():
		return output.Warn("on:no-policy")
	default:
		return "on:?"
	}
}

func printServerlessServices(services []api.ServerlessService) {
	switch getOutputFormat() {
	case output.FormatJSON:
		output.PrintJSON(services)
	case output.FormatQuiet:
		values := make([]string, len(services))
		for i, service := range services {
			values[i] = service.UUID
		}
		output.PrintQuiet(values)
	default:
		if len(services) == 0 {
			fmt.Println("No serverless endpoints found.")
			return
		}
		headers := []string{"UUID", "NAME", "STATUS", "GPU", "VOLUME", "REPLICAS", "SCALING", "REQUESTS", "URL"}
		rows := make([][]string, len(services))
		unknownPolicies := 0
		for i, service := range services {
			status := "paused"
			if service.IsEnabled {
				status = "active"
			}
			volume := service.NetworkVolumeName
			if volume == "" {
				volume = "-"
			}
			if service.AutoscalingEnabled && !service.AutoscalingPolicyCode.Known() {
				unknownPolicies++
			}
			rows[i] = []string{
				shortID(service.UUID),
				service.Name,
				output.StatusColor(status),
				service.GPUTypeName,
				volume,
				fmt.Sprintf("%d/%d", service.ActiveReplicasCount, service.MaxReplicas),
				scalingCell(service),
				fmt.Sprintf("%d", service.TotalRequestsCount),
				service.EndpointURL,
			}
		}
		output.PrintTable(headers, rows)
		if unknownPolicies > 0 {
			fmt.Printf("\n%s\n", output.Dim(fmt.Sprintf(
				"SCALING on:? — the list API does not return the policy for %d endpoint(s). Add --with-policy to load it.",
				unknownPolicies)))
		}
	}
}

func printServerlessDetail(detail *api.ServerlessDetail) {
	service := detail.Data
	fmt.Printf("UUID:          %s\n", service.UUID)
	fmt.Printf("Name:          %s\n", service.Name)
	fmt.Printf("Status:        %s\n", boolStatus(service.IsEnabled, "active", "paused"))
	fmt.Printf("Endpoint key:  %s\n", service.EndpointKey)
	fmt.Printf("URL:           %s\n", service.EndpointURL)
	fmt.Printf("Template:      %s\n", service.TemplateName)
	if service.RegionName != "" {
		fmt.Printf("Region:        %s\n", service.RegionName)
	}

	fmt.Printf("\nResources:\n")
	fmt.Printf("  GPU:         %s x%d\n", service.GPUTypeName, service.GPUCount)
	fmt.Printf("  Memory:      %d GB\n", service.Memory)
	fmt.Printf("  Port:        %d\n", service.EndpointPort)
	fmt.Printf("  Health:      %s\n", service.HealthCheckPath)
	fmt.Printf("  Pull policy: %s\n", service.ImagePullPolicy)
	if service.CUDAVersion.Set() {
		fmt.Printf("  CUDA:        %s\n", service.CUDAVersion)
	}

	fmt.Printf("\nNetwork Volume:\n")
	if service.NetworkVolumeName != "" || service.NetworkVolumeUUID != "" {
		fmt.Printf("  Name:        %s\n", service.NetworkVolumeName)
		if service.NetworkVolumeUUID != "" {
			fmt.Printf("  UUID:        %s\n", service.NetworkVolumeUUID)
		}
		if service.NetworkVolumeMaxSize != nil {
			fmt.Printf("  Size:        %d GB\n", *service.NetworkVolumeMaxSize)
		}
		if service.TemplateVolumeMountPath.Set() {
			fmt.Printf("  Mount path:  %s\n", service.TemplateVolumeMountPath)
		}
	} else {
		fmt.Printf("  (none)\n")
	}

	fmt.Printf("\nScaling:\n")
	fmt.Printf("  Replicas:    %d active, %d provisioning (min %d, max %d)\n", service.ActiveReplicasCount, service.ProvisioningReplicasCount, service.MinReplicas, service.MaxReplicas)
	fmt.Printf("  Concurrency: %d per replica\n", service.MaxConcurrentRequests)
	fmt.Printf("  Overflow:    %s\n", boolStatus(service.AllowOverflowRequests, "enabled", "disabled"))
	fmt.Printf("  Autoscaling: %s\n", boolStatus(service.AutoscalingEnabled, "enabled", "disabled"))
	printAutoscalingPolicy(service, "  ")

	fmt.Printf("\nTimeouts:\n")
	fmt.Printf("  Idle:        %ds\n", service.IdleTimeoutSeconds)
	fmt.Printf("  Cold start:  %ds\n", service.ColdStartTimeoutSeconds)
	fmt.Printf("  Request:     %ds\n", service.RequestTimeoutSeconds)

	fmt.Printf("\nTraffic:\n")
	fmt.Printf("  Requests:    %d total, %d queued\n", service.TotalRequestsCount, service.QueuedRequestsCount)
	if service.LastInvokedAt != "" {
		fmt.Printf("  Last invoke: %s\n", shortTime(service.LastInvokedAt))
	}
	if service.LastScaledAt != "" {
		fmt.Printf("  Last scaled: %s\n", shortTime(service.LastScaledAt))
	}

	if len(service.EnvironmentVariablesMap) > 0 {
		fmt.Printf("\nEnvironment:   %d variable(s)\n", len(service.EnvironmentVariablesMap))
	}
	if service.Command.Set() {
		fmt.Printf("Command:       %s\n", service.Command)
	}
	if service.Notes.Set() {
		fmt.Printf("Notes:         %s\n", service.Notes)
	}

	fmt.Printf("\nReplicas:\n")
	printServerlessReplicas(detail.Replicas, nil)

	if len(detail.AutoscalingLogs.Data) > 0 {
		fmt.Printf("\nRecent autoscaling logs:\n")
		printServerlessAutoscalingLogs(&detail.AutoscalingLogs, false)
	}
}

// printAutoscalingPolicy describes the policy attached to an endpoint. An
// endpoint with autoscaling on and no policy behaves nothing like one whose
// policy this payload simply does not carry, so neither case prints a blank:
// the first is called out as a real misconfiguration, the second points at the
// command that does return the policy.
func printAutoscalingPolicy(service api.ServerlessService, indent string) {
	if !service.AutoscalingPolicyCode.Known() && !service.AutoscalingTemplateKey.Known() {
		if service.AutoscalingEnabled {
			fmt.Printf("%sPolicy:      %s\n", indent,
				output.Warn("not reported by this command — run 'gpulab serverless inspect <endpoint>'"))
		}
		return
	}
	if service.AutoscalingEnabled && !service.AutoscalingTemplateKey.Set() && !service.AutoscalingPolicyCode.Set() {
		fmt.Printf("%sPolicy:      %s\n", indent,
			output.Warn("none attached — autoscaling is on with nothing to evaluate"))
		return
	}
	if service.AutoscalingTemplateKey.Set() {
		fmt.Printf("%sTemplate:    %s\n", indent, service.AutoscalingTemplateKey)
	}
	if service.AutoscalingPolicyCode.Set() {
		fmt.Printf("%sPolicy:      %s\n", indent, summarizeText(service.AutoscalingPolicyCode.String()))
	}
	if service.AutoscalingMetricsConfig.Set() {
		fmt.Printf("%sMetrics:     %s\n", indent, summarizeText(service.AutoscalingMetricsConfig.String()))
	}
	if service.AutoscalingLastEvaluatedAt.Set() {
		fmt.Printf("%sLast eval:   %s\n", indent, shortTime(service.AutoscalingLastEvaluatedAt.String()))
	}
	if service.AutoscalingLastErrorAt.Set() {
		fmt.Printf("%sLast error:  %s\n", indent, shortTime(service.AutoscalingLastErrorAt.String()))
	}
}

// summarizeText collapses a possibly-multiline config value to a single line for
// summary display; the full value is available via --json.
func summarizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(none)"
	}
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return fmt.Sprintf("%s … (%d lines)", strings.TrimSpace(s[:idx]), strings.Count(s, "\n")+1)
	}
	if len(s) > 70 {
		return s[:67] + "..."
	}
	return s
}

// resolveServerlessReplicaUUID maps a full UUID, UUID prefix, or server name to
// a replica UUID within the given endpoint.
func resolveServerlessReplicaUUID(client *api.Client, identifier, partial string) (string, error) {
	detail, err := client.GetServerlessService(identifier)
	if err != nil {
		return "", err
	}
	var matches []api.ServerlessReplica
	for _, replica := range detail.Replicas {
		if replica.UUID == partial {
			return replica.UUID, nil
		}
		if strings.HasPrefix(replica.UUID, partial) || strings.HasPrefix(replica.ServerName, partial) {
			matches = append(matches, replica)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no replica found matching %q (run 'gpulab serverless replicas %s' to list)", partial, partial)
	case 1:
		return matches[0].UUID, nil
	default:
		msg := fmt.Sprintf("ambiguous replica %q matches %d replicas:\n", partial, len(matches))
		for _, m := range matches {
			msg += fmt.Sprintf("  %s  %s  %s\n", m.UUID, m.ServerName, m.Status)
		}
		return "", fmt.Errorf("%s", msg)
	}
}

// enrichReplica builds the machine-facing replica object with fields that are
// always populated, even when the raw API row leaves them blank: server, a
// non-empty status, created, and a derived age/uptime. This is the single
// source of truth for replica JSON; staleness (#13) and code version (#14)
// extend the map elsewhere.
func enrichReplica(r api.ServerlessReplica) map[string]interface{} {
	server := ""
	if r.Server != nil {
		server = r.Server["name"]
	}

	m := map[string]interface{}{
		"uuid":       r.UUID,
		"name":       r.ServerName,
		"server":     firstNonEmpty(server, r.ServerName, "-"),
		"status":     firstNonEmpty(r.Status, "unknown"),
		"created":    firstNonEmpty(r.CreatedAt, ""),
		"created_at": r.CreatedAt,
		"updated_at": r.UpdatedAt,
		"has_logs":   r.HasLogs,
		"uptime":     r.Uptime,
		"age":        "-",
	}

	if t, ok := output.ParseTimestamp(r.CreatedAt); ok {
		age := time.Since(t)
		m["age"] = output.HumanDuration(age)
		m["age_seconds"] = int64(age.Seconds())
		// The API often leaves uptime blank; derive it from created so machine
		// consumers always get a duration.
		if strings.TrimSpace(r.Uptime) == "" {
			m["uptime"] = output.HumanDuration(age)
		}
	}
	return m
}

func enrichReplicas(replicas []api.ServerlessReplica) []map[string]interface{} {
	return enrichReplicasWithStale(replicas, nil)
}

// enrichReplicasWithStale is enrichReplicas plus staleness fields when a stale
// map is provided (from --stale).
func enrichReplicasWithStale(replicas []api.ServerlessReplica, stale map[string]*staleResult) []map[string]interface{} {
	out := make([]map[string]interface{}, len(replicas))
	for i, r := range replicas {
		m := enrichReplica(r)
		if res, ok := stale[r.UUID]; ok && res != nil && res.Checked {
			if res.Unknown {
				m["stale"] = nil
				m["stale_unknown"] = true
			} else {
				m["stale"] = res.Stale
			}
			m["volume_newest_file"] = res.NewestFile
			if !res.NewestTime.IsZero() {
				m["volume_newest_mtime"] = res.NewestTime.UTC().Format(time.RFC3339)
			}
		}
		out[i] = m
	}
	return out
}

func printServerlessReplicas(replicas []api.ServerlessReplica, stale map[string]*staleResult) {
	if len(replicas) == 0 {
		fmt.Println("No replicas found.")
		return
	}
	headers := []string{"UUID", "NAME", "STATUS", "SERVER", "AGE", "CREATED"}
	if stale != nil {
		headers = append(headers, "CODE")
	}
	rows := make([][]string, len(replicas))
	for i, replica := range replicas {
		server := ""
		if replica.Server != nil {
			server = replica.Server["name"]
		}
		row := []string{
			shortID(replica.UUID),
			replica.ServerName,
			output.StatusColor(replica.Status),
			server,
			output.Age(replica.CreatedAt),
			shortTime(replica.CreatedAt),
		}
		if stale != nil {
			row = append(row, staleCell(stale[replica.UUID]))
		}
		rows[i] = row
	}
	output.PrintTable(headers, rows)

	if stale != nil {
		if sample := firstStaleResult(stale); sample != nil && sample.Checked {
			if sample.NewestTime.IsZero() {
				fmt.Printf("Volume newest file: %s\n", output.Warn("could not determine (no comparable code file found) — staleness unknown"))
			} else {
				fmt.Printf("Volume newest file: %s (%s)\n", sample.NewestFile, shortTime(sample.NewestTime.Format(time.RFC3339)))
			}
		}
	}
}

// staleCell renders the CODE column: whether a replica may be running code older
// than the volume.
func staleCell(res *staleResult) string {
	if res == nil || !res.Checked {
		return "-"
	}
	if res.Unknown {
		return output.Warn("? unknown")
	}
	if res.Stale {
		return output.Warn("⚠ stale")
	}
	return output.Good("ok")
}

func firstStaleResult(stale map[string]*staleResult) *staleResult {
	for _, v := range stale {
		return v
	}
	return nil
}

func printServerlessRequests(page *api.ServerlessPage[api.ServerlessRequestLog], details bool) {
	if len(page.Data) == 0 {
		fmt.Println("No serverless request logs found.")
		return
	}
	headers := []string{"UUID", "STATUS", "METHOD", "PATH", "CODE", "DURATION", "QUEUED"}
	rows := make([][]string, len(page.Data))
	for i, request := range page.Data {
		code := "-"
		if request.ResponseStatusCode != nil {
			code = strconv.Itoa(*request.ResponseStatusCode)
		}
		duration := "-"
		if request.DurationMS != nil {
			duration = fmt.Sprintf("%dms", *request.DurationMS)
		}
		rows[i] = []string{
			shortID(request.UUID),
			output.StatusColor(request.Status),
			request.Method,
			requestTarget(request.Path, request.QueryString),
			code,
			duration,
			shortTime(request.QueuedAt),
		}
	}
	output.PrintTable(headers, rows)
	fmt.Printf("Page %d of %d, %d total\n", page.CurrentPage, page.LastPage, page.Total)

	if details {
		for _, request := range page.Data {
			fmt.Printf("\n%s %s %s\n", request.UUID, request.Method, requestTarget(request.Path, request.QueryString))
			printBodyPayload("Request", request.RequestBody)
			printBodyPayload("Response", request.ResponseBody)
			if request.ErrorMessage != "" {
				fmt.Printf("Error: %s\n", request.ErrorMessage)
			}
		}
	}
}

func printServerlessAutoscalingLogs(page *api.ServerlessPage[api.ServerlessAutoscalingLog], details bool) {
	if len(page.Data) == 0 {
		fmt.Println("No autoscaling logs found.")
		return
	}
	headers := []string{"TIME", "LEVEL", "EVENT", "CURRENT", "DESIRED", "MESSAGE"}
	rows := make([][]string, len(page.Data))
	for i, log := range page.Data {
		current := "-"
		desired := "-"
		if log.CurrentReplicas != nil {
			current = strconv.Itoa(*log.CurrentReplicas)
		}
		if log.DesiredReplicas != nil {
			desired = strconv.Itoa(*log.DesiredReplicas)
		}
		rows[i] = []string{shortTime(log.CreatedAt), log.Level, log.Event, current, desired, log.Message}
	}
	output.PrintTable(headers, rows)
	fmt.Printf("Page %d of %d, %d total\n", page.CurrentPage, page.LastPage, page.Total)

	if details {
		for _, log := range page.Data {
			fmt.Printf("\n[%s] %s %s\n", shortTime(log.CreatedAt), log.Level, log.Event)
			printIndentedJSON("Metrics", log.Metrics)
			printIndentedJSON("Context", log.Context)
		}
	}
}

func printServerlessOptions(options *api.ServerlessOptions) {
	fmt.Println("Templates")
	templateRows := make([][]string, len(options.Templates))
	for i, template := range options.Templates {
		memory := "-"
		if template.MemoryLimit != nil {
			memory = fmt.Sprintf("%d GB", *template.MemoryLimit)
		}
		templateRows[i] = []string{strconv.Itoa(template.ID), template.Name, template.DockerImage, memory}
	}
	output.PrintTable([]string{"ID", "NAME", "IMAGE", "MEMORY"}, templateRows)

	fmt.Println("\nGPU Types")
	gpuRows := make([][]string, len(options.GPUTypes))
	for i, gpu := range options.GPUTypes {
		price := "-"
		if value, ok := gpu.GPUPrice.Float64(); ok && value > 0 {
			price = fmt.Sprintf("$%.2f/hr", value)
		}
		gpuRows[i] = []string{strconv.Itoa(gpu.ID), gpu.GPUName, fmt.Sprintf("%d MB", gpu.GPUTotalMemory), fmt.Sprintf("%d", gpu.FreeGPUsCount), price}
	}
	output.PrintTable([]string{"ID", "GPU", "MEMORY", "FREE", "PRICE"}, gpuRows)

	fmt.Println("\nRegions")
	regionRows := make([][]string, len(options.Regions))
	for i, region := range options.Regions {
		regionRows[i] = []string{region.ID, region.Name, region.Slug}
	}
	output.PrintTable([]string{"ID", "NAME", "SLUG"}, regionRows)

	fmt.Println("\nNetwork Volumes")
	volumeRows := make([][]string, len(options.NetworkVolumes))
	for i, volume := range options.NetworkVolumes {
		size := "-"
		if volume.MaxSize != nil {
			size = fmt.Sprintf("%d GB", *volume.MaxSize)
		}
		volumeRows[i] = []string{strconv.Itoa(volume.ID), volume.VolumeName, volume.RegionName, size}
	}
	output.PrintTable([]string{"ID", "NAME", "REGION", "SIZE"}, volumeRows)

	fmt.Println("\nAutoscaling Templates")
	autoRows := make([][]string, len(options.AutoscalingTemplates))
	for i, template := range options.AutoscalingTemplates {
		autoRows[i] = []string{template.Key, template.Name, template.Summary}
	}
	output.PrintTable([]string{"KEY", "NAME", "SUMMARY"}, autoRows)
}

func selectServerlessLogReplicas(replicas []api.ServerlessReplica, selector string, deployLogs bool) ([]api.ServerlessReplica, error) {
	if len(replicas) == 0 {
		return nil, fmt.Errorf("no replicas found for this serverless endpoint")
	}
	if selector == "all" {
		return replicas, nil
	}
	if selector != "" {
		var matches []api.ServerlessReplica
		for _, replica := range replicas {
			if strings.HasPrefix(replica.UUID, selector) || strings.HasPrefix(replica.ServerName, selector) {
				matches = append(matches, replica)
			}
		}
		if len(matches) == 1 {
			return matches, nil
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no replica found with prefix %q", selector)
		}
		return nil, fmt.Errorf("ambiguous replica prefix %q", selector)
	}

	if deployLogs {
		for _, replica := range replicas {
			if replica.HasLogs {
				return []api.ServerlessReplica{replica}, nil
			}
		}
	}
	for _, replica := range replicas {
		status := strings.ToLower(replica.Status)
		if status == "running" || status == "healthy" || status == "unhealthy" {
			return []api.ServerlessReplica{replica}, nil
		}
	}
	return []api.ServerlessReplica{replicas[0]}, nil
}

func printServerlessReplicaLogs(client *api.Client, replicas []api.ServerlessReplica, deploy bool, tail int, since string, timestamps bool, grep string, detect bool) error {
	results := make(map[string]string)
	for _, replica := range replicas {
		var logs string
		var err error
		if deploy {
			logs, err = client.GetDeploymentLogs(replica.UUID)
			logs = tailLines(logs, tail)
		} else {
			logs, err = client.GetContainerLogs(replica.UUID, tail, since, timestamps)
		}
		if err != nil {
			return err
		}
		if grep != "" {
			logs, err = grepLogs(logs, grep)
			if err != nil {
				return err
			}
		}
		results[replica.UUID] = logs
	}
	if flagJSON && since == "" {
		output.PrintJSON(results)
		return nil
	}
	for _, replica := range replicas {
		logs := results[replica.UUID]
		if len(replicas) > 1 && logs != "" {
			fmt.Printf("==> %s (%s) <==\n", replica.ServerName, shortID(replica.UUID))
		}
		fmt.Print(logs)
		if logs != "" && logs[len(logs)-1] != '\n' {
			fmt.Println()
		}
		if detect {
			if issues := detectLogIssues(logs); len(issues) > 0 {
				fmt.Fprintf(os.Stderr, "\n%s detected in %s:\n", output.Warn("Issues"), shortID(replica.UUID))
				for _, iss := range issues {
					fmt.Fprintf(os.Stderr, "  %s x%d  %s\n", iss.Category, iss.Count, iss.Sample)
				}
			}
		}
	}
	return nil
}

func resolveInvokeURL(client *api.Client, args []string) (string, error) {
	target := args[0]
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		if len(args) == 2 {
			return strings.TrimRight(target, "/") + "/" + strings.TrimLeft(args[1], "/"), nil
		}
		return target, nil
	}

	resolved, err := client.ResolveServerlessService(target)
	if err == nil {
		detail, err := client.GetServerlessService(resolved)
		if err != nil {
			return "", err
		}
		target = detail.Data.EndpointURL
	} else {
		target = strings.TrimRight(client.BaseURL, "/") + "/serverless/" + strings.TrimLeft(target, "/")
	}
	if len(args) == 2 {
		target = strings.TrimRight(target, "/") + "/" + strings.TrimLeft(args[1], "/")
	}
	return target, nil
}

func invokeServerlessURL(ctx context.Context, target string) (*http.Response, []byte, error) {
	var body io.Reader
	if serverlessInvokeDataFile != "" {
		data, err := os.ReadFile(serverlessInvokeDataFile)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(data)
	} else if serverlessInvokeData != "" {
		body = strings.NewReader(serverlessInvokeData)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(serverlessInvokeMethod), target, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	for _, header := range serverlessInvokeHeader {
		name, value, ok := strings.Cut(header, ":")
		if !ok {
			return nil, nil, fmt.Errorf("invalid header %q, expected Name: Value", header)
		}
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: time.Duration(serverlessInvokeTimeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, data, nil
}

func waitServerlessRequest(client *api.Client, requestID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := client.GetServerlessRequestStatus(requestID)
		if err != nil {
			return err
		}
		if !status.Processing {
			if flagJSON {
				output.PrintJSON(status)
			} else {
				printServerlessStatus(status)
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for serverless request %s", requestID)
		}
		time.Sleep(3 * time.Second)
	}
}

func printServerlessStatus(status *api.ServerlessStatusResponse) {
	fmt.Printf("Request:    %s\n", status.RequestID)
	fmt.Printf("Status:     %s\n", output.StatusColor(status.Status))
	fmt.Printf("Processing: %t\n", status.Processing)
	fmt.Printf("Message:    %s\n", status.Message)
	if status.Response != nil {
		if code, ok := status.Response["status_code"]; ok {
			fmt.Printf("HTTP Code:   %v\n", code)
		}
		if body, ok := status.Response["body"].(string); ok && body != "" {
			fmt.Printf("\n%s\n", body)
		}
	}
}

func parseRequestID(body []byte) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if value, ok := payload["request_id"].(string); ok {
		return value
	}
	return ""
}

func printBodyPayload(label string, body *api.ServerlessBodyPayload) {
	if body == nil {
		fmt.Printf("%s: <empty>\n", label)
		return
	}
	switch body.Encoding {
	case "utf8":
		fmt.Printf("%s:\n%s\n", label, body.Data)
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(body.Data)
		if err == nil {
			fmt.Printf("%s: <binary %d bytes>\n", label, len(decoded))
		} else {
			fmt.Printf("%s: <base64 %d bytes>\n", label, body.SizeBytes)
		}
	default:
		fmt.Printf("%s: <%s %d bytes>\n", label, body.Encoding, body.SizeBytes)
	}
}

func printIndentedJSON(label string, value interface{}) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil || string(data) == "{}" || string(data) == "null" {
		return
	}
	fmt.Printf("%s:\n%s\n", label, string(data))
}

func requestTarget(path, query string) string {
	if query == "" {
		return path
	}
	return path + "?" + query
}

func setStringFlag(cmd *cobra.Command, name string, target *string, value string) {
	if cmd.Flags().Changed(name) {
		*target = value
	}
}

func setIntFlag(cmd *cobra.Command, name string, target *int, value int) {
	if cmd.Flags().Changed(name) {
		*target = value
	}
}

func setBoolFlag(cmd *cobra.Command, name string, target *bool, value bool) {
	if cmd.Flags().Changed(name) {
		*target = value
	}
}

func boolStatus(enabled bool, trueValue, falseValue string) string {
	if enabled {
		return trueValue
	}
	return falseValue
}

func shortID(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

// shortTime renders an API timestamp in the active timezone (see --tz). It is a
// thin wrapper over output.FormatTime so every call site is timezone-consistent.
func shortTime(value string) string {
	return output.FormatTime(value)
}

func addQueryParam(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
