package commands

import (
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
)

// staleResult records whether a replica started before the volume's newest
// (code) file, i.e. it may be running code older than what's on disk — the
// classic stale-replica trap where a /restart that didn't reload code leaves an
// old worker crashing while the volume already has the fix.
type staleResult struct {
	Checked    bool
	Stale      bool
	NewestFile string
	NewestTime time.Time
}

// noiseFileSuffixes are runtime-written artifacts that should not count as
// "newer code" — they change constantly and would make every replica look stale.
var noiseFileSuffixes = []string{
	".log", ".pyc", ".pyo", ".pid", ".lock", ".tmp", ".temp",
	".swp", ".swo", ".cache", ".out", ".err", ".sqlite", ".db",
}

// noiseDirNames are directories skipped entirely during the newest-file scan.
var noiseDirNames = map[string]bool{
	"__pycache__": true, ".cache": true, ".git": true, "logs": true,
	"log": true, "tmp": true, "temp": true, "node_modules": true,
	".ipynb_checkpoints": true, "outputs": true, "output": true,
	"generations": true, "results": true, "samples": true, "uploads": true,
}

func isNoiseFile(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range noiseFileSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return lower == ".ds_store"
}

// newestVolumeFile walks a volume up to maxDepth directory levels and returns
// the most recently modified non-noise file and its mtime. The walk is bounded
// so this stays cheap for the common case (code in the root or a subdirectory).
func newestVolumeFile(client *api.Client, volumeUUID, dir string, maxDepth int) (newestPath string, newest time.Time, err error) {
	resp, err := client.ListVolumeFiles(volumeUUID, dir)
	if err != nil {
		return "", time.Time{}, err
	}
	for _, item := range resp.Items {
		if item.IsDirectory {
			if maxDepth > 0 && !noiseDirNames[strings.ToLower(item.Name)] {
				childPath, childTime, cerr := newestVolumeFile(client, volumeUUID, item.Path, maxDepth-1)
				if cerr == nil && childTime.After(newest) {
					newest, newestPath = childTime, childPath
				}
			}
			continue
		}
		if isNoiseFile(item.Name) {
			continue
		}
		if t, ok := output.ParseTimestamp(item.Modified); ok && t.After(newest) {
			newest, newestPath = t, item.Path
		}
	}
	return newestPath, newest, nil
}

// computeReplicaStaleness flags each replica that started before the volume's
// newest code file. codePath scopes the scan to a subdirectory (empty = whole
// volume). An empty volumeUUID (endpoint has no volume) returns an error so
// callers can explain why staleness can't be determined.
func computeReplicaStaleness(client *api.Client, volumeUUID, codePath string, replicas []api.ServerlessReplica, maxDepth int) (map[string]*staleResult, error) {
	if volumeUUID == "" {
		return nil, fmt.Errorf("endpoint has no network volume to compare against")
	}
	newestFile, newest, err := newestVolumeFile(client, volumeUUID, strings.Trim(codePath, "/"), maxDepth)
	if err != nil {
		return nil, err
	}
	out := make(map[string]*staleResult, len(replicas))
	for _, r := range replicas {
		res := &staleResult{Checked: true, NewestFile: newestFile, NewestTime: newest}
		if t, ok := output.ParseTimestamp(r.CreatedAt); ok && !newest.IsZero() {
			res.Stale = t.Before(newest)
		}
		out[r.UUID] = res
	}
	return out, nil
}

// shortVolumePath trims a long volume path to its last two segments for display.
func shortVolumePath(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return "(root)"
	}
	parts := strings.Split(p, "/")
	if len(parts) <= 2 {
		return p
	}
	return ".../" + path.Join(parts[len(parts)-2:]...)
}
