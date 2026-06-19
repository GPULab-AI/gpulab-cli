package commands

import (
	"fmt"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	logsFollow     bool
	logsTail       int
	logsDeploy     bool
	logsTimestamps bool
	logsSince      string
)

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Stream new log output until interrupted")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "n", 100, "Number of lines from the end (0 = all)")
	logsCmd.Flags().BoolVar(&logsDeploy, "deploy", false, "Show deployment logs instead of runtime logs")
	logsCmd.Flags().BoolVarP(&logsTimestamps, "timestamps", "t", false, "Prefix each line with a timestamp")
	logsCmd.Flags().StringVar(&logsSince, "since", "", "Only show logs since a Unix timestamp or relative time (e.g. 10m, 1h)")

	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs [CONTAINER]",
	Short: "Get container logs",
	Long: `Fetch runtime (or deployment) logs for a container.

CONTAINER may be a full UUID or a unique UUID prefix. Use --deploy to inspect
why a container failed to start, and --follow to stream new output live.`,
	Example: `  gpulab logs ab12cd34                 # last 100 runtime log lines
  gpulab logs ab12cd34 -n 500          # last 500 lines
  gpulab logs ab12cd34 -f              # follow live output
  gpulab logs ab12cd34 --since 15m     # logs from the last 15 minutes
  gpulab logs ab12cd34 --deploy        # deployment/startup logs
  gpulab logs ab12cd34 --json          # machine-readable output`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		uuid, err := client.ResolveContainerUUID(args[0])
		if err != nil {
			return err
		}

		if logsDeploy {
			logs, err := client.GetDeploymentLogs(uuid)
			if err != nil {
				return err
			}
			if flagJSON {
				output.PrintJSON(map[string]string{"logs": logs})
			} else {
				printLogChunk(logs)
			}
			return nil
		}

		// Get runtime logs
		logs, err := client.GetContainerLogs(uuid, logsTail, logsSince, logsTimestamps)
		if err != nil {
			return err
		}

		if flagJSON && !logsFollow {
			output.PrintJSON(map[string]string{"logs": logs})
			return nil
		}

		printLogChunk(logs)

		// Follow mode: poll for new output. Advance `since` to the moment just
		// before each request (captured up front) so the next poll resumes from
		// there with no gap; only advance on a successful fetch.
		if logsFollow {
			since := fmt.Sprintf("%d", time.Now().Unix())
			for {
				time.Sleep(2 * time.Second)
				next := fmt.Sprintf("%d", time.Now().Unix())
				newLogs, err := client.GetContainerLogs(uuid, 0, since, logsTimestamps)
				if err != nil {
					continue
				}
				since = next
				printLogChunk(newLogs)
			}
		}

		return nil
	},
}

// printLogChunk writes a log blob and ensures it ends with a newline.
func printLogChunk(logs string) {
	if logs == "" {
		return
	}
	fmt.Print(logs)
	if logs[len(logs)-1] != '\n' {
		fmt.Println()
	}
}
