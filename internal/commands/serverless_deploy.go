package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	deployTo       string
	deployDest     string
	deployDrain    bool
	deployRolling  bool
	deployHealth   string
	deployNoVerify bool
	deployNoSmoke  bool
	deployYes      bool
	slDeployWait   int
)

func init() {
	serverlessCmd.AddCommand(serverlessDeployCmd)
	serverlessDeployCmd.Flags().StringVar(&deployTo, "to", "", "Destination path in the volume for a single file (e.g. server/tasks.py)")
	serverlessDeployCmd.Flags().StringVar(&deployDest, "dest", "", "Destination directory in the volume for multiple files")
	serverlessDeployCmd.Flags().BoolVar(&deployDrain, "drain", false, "Finish in-flight requests before tearing each replica down")
	serverlessDeployCmd.Flags().BoolVar(&deployRolling, "rolling", true, "Restart replicas one at a time, waiting for each replacement (needs min-replicas >= 1)")
	serverlessDeployCmd.Flags().StringVar(&deployHealth, "health-path", "", "Health path for the smoke test (default: the endpoint's health check path)")
	serverlessDeployCmd.Flags().BoolVar(&deployNoVerify, "no-verify", false, "Skip byte-identical verification of uploaded files")
	serverlessDeployCmd.Flags().BoolVar(&deployNoSmoke, "no-smoke", false, "Skip the post-restart smoke test")
	serverlessDeployCmd.Flags().BoolVarP(&deployYes, "yes", "y", false, "Skip confirmation before restarting replicas")
	serverlessDeployCmd.Flags().IntVar(&slDeployWait, "wait-timeout", 300, "Seconds to wait for replacements and the smoke test")
}

type deployPlanItem struct {
	Local  string
	Remote string // full path inside the volume
	Dir    string // destination directory
	Name   string // stored filename
}

var serverlessDeployCmd = &cobra.Command{
	Use:   "deploy [ENDPOINT] [LOCAL_FILE...]",
	Short: "Deploy code to an endpoint: upload → verify → rolling restart → smoke test",
	Long: `Atomically deploy code to a serverless endpoint. This replaces the manual
upload-then-restart dance and prevents the stale-replica trap where a restart
doesn't actually reload the new code.

Steps:
  1. Upload the file(s) to the endpoint's network volume.
  2. Verify each uploaded file is byte-identical to the local copy.
  3. Rolling-restart the replicas so they reload the new code.
  4. Smoke-test the endpoint's health path.

Use --to for a single file's exact destination, or --dest for a directory when
uploading several files.

Examples:
  gpulab serverless deploy my-endpoint ./tasks.py --to server/tasks.py
  gpulab serverless deploy my-endpoint a.py b.py --dest server --drain
  gpulab serverless deploy my-endpoint ./tasks.py --to server/tasks.py --no-smoke -y`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth().WithTimeout(30 * time.Minute)
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		svc := detail.Data
		if svc.NetworkVolumeUUID == "" {
			return fmt.Errorf("endpoint %s has no network volume to deploy to", shortID(identifier))
		}

		locals := args[1:]
		for _, p := range locals {
			fi, statErr := os.Stat(p)
			if statErr != nil {
				return fmt.Errorf("cannot read %s: %w", p, statErr)
			}
			if fi.IsDir() {
				return fmt.Errorf("%s is a directory; pass individual files or use 'gpulab volumes sync'", p)
			}
		}

		plan, err := buildDeployPlan(locals)
		if err != nil {
			return err
		}

		if !deployYes && !flagJSON {
			fmt.Printf("Deploy %d file(s) to volume %s and restart %d replica(s) of %s? [y/N] ",
				len(plan), shortID(svc.NetworkVolumeUUID), len(detail.Replicas), shortID(identifier))
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		steps := &deploySteps{json: flagJSON}

		// Step 1: upload, grouped by destination directory.
		steps.begin("Uploading %d file(s) to volume %s", len(plan), shortID(svc.NetworkVolumeUUID))
		uploads := map[string][]api.UploadItem{}
		for _, it := range plan {
			uploads[it.Dir] = append(uploads[it.Dir], api.UploadItem{LocalPath: it.Local, DestName: it.Name})
		}
		for dir, items := range uploads {
			if _, err := client.UploadVolumeFiles(svc.NetworkVolumeUUID, dir, items); err != nil {
				return fmt.Errorf("upload failed: %w", err)
			}
		}
		steps.ok("uploaded %d file(s)", len(plan))

		// Step 2: verify byte-identical before we restart into it.
		if !deployNoVerify {
			steps.begin("Verifying uploads are byte-identical")
			for _, it := range plan {
				if err := verifyReadback(client, svc.NetworkVolumeUUID, it.Remote, it.Local); err != nil {
					return fmt.Errorf("%w — aborting before restart", err)
				}
			}
			steps.ok("all %d file(s) byte-identical", len(plan))
		}

		// Step 3: rolling restart so replicas reload the new code.
		restarted := 0
		if len(detail.Replicas) == 0 {
			steps.skip("no running replicas; new code loads on next cold start")
		} else {
			mode := "restarting"
			if deployRolling {
				mode = "rolling-restarting"
			}
			steps.begin("%s %d replica(s)", mode, len(detail.Replicas))
			if err := restartServerlessReplicas(client, identifier, detail.Replicas, svc.MinReplicas, deployRolling, deployDrain, slDeployWait); err != nil {
				return err
			}
			restarted = len(detail.Replicas)
		}

		// Step 4: prove the restart actually recycled the replicas onto the new
		// code. A liveness smoke test alone can't catch a restart that didn't
		// reload code, so we re-check staleness: every running replica must have
		// started after the code we just deployed.
		var recycleErr error
		if restarted > 0 && !deployNoVerify && svc.NetworkVolumeUUID != "" {
			steps.begin("Verifying replicas recycled onto the new code")
			stuck, err := assertReplicasRecycled(client, identifier, svc.NetworkVolumeUUID, deployCodePath(plan), slDeployWait)
			if err != nil {
				steps.fail("could not confirm recycle: %v", err)
			} else if len(stuck) > 0 {
				recycleErr = fmt.Errorf("replicas did not reload the new code (still predate it): %s", strings.Join(stuck, ", "))
				steps.fail("%v", recycleErr)
			} else {
				steps.ok("all running replicas started after the deployed code")
			}
		}

		// Step 5: smoke test the health path.
		smokeCode := 0
		var smokeErr error
		healthPath := firstNonEmpty(deployHealth, svc.HealthCheckPath, "/")
		if !deployNoSmoke && svc.EndpointURL != "" {
			steps.begin("Smoke test GET %s", healthPath)
			smokeCode, smokeErr = smokeTestEndpoint(cmd.Context(), svc.EndpointURL, healthPath, slDeployWait)
			if smokeErr != nil {
				steps.fail("smoke test failed: %v", smokeErr)
			} else {
				steps.ok("health check returned %d", smokeCode)
			}
		}

		overallErr := recycleErr
		if overallErr == nil {
			overallErr = smokeErr
		}

		if flagJSON {
			output.PrintJSON(map[string]interface{}{
				"endpoint":     identifier,
				"volume":       svc.NetworkVolumeUUID,
				"files":        len(plan),
				"verified":     !deployNoVerify,
				"replicas":     restarted,
				"recycled":     recycleErr == nil,
				"smoke_status": smokeCode,
				"smoke_ok":     smokeErr == nil,
				"status":       boolStatus(overallErr == nil, "success", "failed"),
			})
		}
		if recycleErr != nil {
			return fmt.Errorf("deploy: %w", recycleErr)
		}
		if smokeErr != nil {
			return fmt.Errorf("deploy applied but smoke test failed: %w", smokeErr)
		}
		if !flagJSON {
			output.PrintSuccess(fmt.Sprintf("Deployed %d file(s) to %s", len(plan), shortID(identifier)))
		}
		return nil
	},
}

// assertReplicasRecycled waits for running replicas to come up after a restart
// and returns the UUIDs of any that still predate the volume's newest code file
// (i.e. the restart did not reload the new code). An empty result means all
// running replicas are on the new code.
func assertReplicasRecycled(client *api.Client, identifier, volumeUUID, codePath string, timeoutSecs int) ([]string, error) {
	deadline := time.Now().Add(time.Duration(timeoutSecs) * time.Second)
	for {
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return nil, err
		}
		running := runningReplicas(detail.Replicas)
		if len(running) > 0 {
			stale, err := computeReplicaStaleness(client, volumeUUID, codePath, running, 2)
			if err != nil {
				return nil, err
			}
			var stuck []string
			for _, r := range running {
				if res := stale[r.UUID]; res != nil && res.Stale {
					stuck = append(stuck, shortID(r.UUID))
				}
			}
			return stuck, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("no running replicas came up within %ds", timeoutSecs)
		}
		time.Sleep(3 * time.Second)
	}
}

func runningReplicas(replicas []api.ServerlessReplica) []api.ServerlessReplica {
	var out []api.ServerlessReplica
	for _, r := range replicas {
		switch strings.ToLower(r.Status) {
		case "running", "healthy", "unhealthy":
			out = append(out, r)
		}
	}
	return out
}

// deployCodePath returns the common destination directory of the plan (used to
// scope the recycle staleness check), or "" if the files span directories.
func deployCodePath(plan []deployPlanItem) string {
	if len(plan) == 0 {
		return ""
	}
	dir := plan[0].Dir
	for _, it := range plan[1:] {
		if it.Dir != dir {
			return ""
		}
	}
	return dir
}

func buildDeployPlan(locals []string) ([]deployPlanItem, error) {
	var plan []deployPlanItem
	if deployTo != "" {
		if len(locals) != 1 {
			return nil, fmt.Errorf("--to takes a single local file (got %d)", len(locals))
		}
		to := strings.TrimLeft(deployTo, "/")
		if strings.HasSuffix(to, "/") {
			// A trailing slash means "into this directory" — keep the local name.
			dir := strings.TrimRight(to, "/")
			name := filepath.Base(locals[0])
			remote := name
			if dir != "" {
				remote = dir + "/" + name
			}
			plan = append(plan, deployPlanItem{Local: locals[0], Remote: remote, Dir: dir, Name: name})
			return plan, nil
		}
		plan = append(plan, deployPlanItem{Local: locals[0], Remote: to, Dir: baseDir(to), Name: baseName(to)})
		return plan, nil
	}
	dir := strings.Trim(deployDest, "/")
	for _, p := range locals {
		name := filepath.Base(p)
		remote := name
		if dir != "" {
			remote = dir + "/" + name
		}
		plan = append(plan, deployPlanItem{Local: p, Remote: remote, Dir: dir, Name: name})
	}
	return plan, nil
}

// smokeTestEndpoint issues GET requests to the endpoint's health path until it
// returns a 2xx/3xx or the timeout elapses (cold starts take time). It returns
// the last status code and a non-nil error if it never became healthy.
func smokeTestEndpoint(ctx context.Context, endpointURL, healthPath string, timeoutSecs int) (int, error) {
	url := strings.TrimRight(endpointURL, "/") + "/" + strings.TrimLeft(healthPath, "/")
	deadline := time.Now().Add(time.Duration(timeoutSecs) * time.Second)
	httpClient := &http.Client{Timeout: 30 * time.Second}
	var lastCode int
	var lastErr error
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, err
		}
		resp, err := httpClient.Do(req)
		if err == nil {
			lastCode = resp.StatusCode
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return resp.StatusCode, nil
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastCode != 0 {
				return lastCode, fmt.Errorf("health check returned %d after %ds", lastCode, timeoutSecs)
			}
			return 0, fmt.Errorf("health check unreachable after %ds: %v", timeoutSecs, lastErr)
		}
		time.Sleep(3 * time.Second)
	}
}

// deploySteps renders numbered progress to stderr (suppressed in --json mode).
type deploySteps struct {
	n    int
	json bool
}

func (s *deploySteps) begin(format string, args ...interface{}) {
	s.n++
	if !s.json {
		fmt.Fprintf(os.Stderr, "[%d] %s...\n", s.n, fmt.Sprintf(format, args...))
	}
}

func (s *deploySteps) ok(format string, args ...interface{}) {
	if !s.json {
		fmt.Fprintf(os.Stderr, "    %s %s\n", output.Good("✓"), fmt.Sprintf(format, args...))
	}
}

func (s *deploySteps) skip(format string, args ...interface{}) {
	s.n++
	if !s.json {
		fmt.Fprintf(os.Stderr, "[%d] %s\n", s.n, fmt.Sprintf(format, args...))
	}
}

func (s *deploySteps) fail(format string, args ...interface{}) {
	if !s.json {
		fmt.Fprintf(os.Stderr, "    %s %s\n", output.Warn("✗"), fmt.Sprintf(format, args...))
	}
}
