package commands

import (
	"fmt"
	"time"

	"github.com/gpulab/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	logsFollow     bool
	logsTail       int
	logsDeploy     bool
	logsTimestamps bool
)

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
	logsCmd.Flags().IntVarP(&logsTail, "tail", "n", 100, "Number of lines from the end")
	logsCmd.Flags().BoolVar(&logsDeploy, "deploy", false, "Show deployment logs instead")
	logsCmd.Flags().BoolVarP(&logsTimestamps, "timestamps", "t", false, "Show timestamps")

	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs [UUID]",
	Short: "Get container logs",
	Args:  cobra.ExactArgs(1),
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
				fmt.Print(logs)
				if logs != "" && logs[len(logs)-1] != '\n' {
					fmt.Println()
				}
			}
			return nil
		}

		// Get runtime logs
		logs, err := client.GetContainerLogs(uuid, logsTail, "", logsTimestamps)
		if err != nil {
			return err
		}

		if flagJSON && !logsFollow {
			output.PrintJSON(map[string]string{"logs": logs})
			return nil
		}

		fmt.Print(logs)
		if logs != "" && logs[len(logs)-1] != '\n' {
			fmt.Println()
		}

		// Follow mode: poll every 2 seconds
		if logsFollow {
			since := fmt.Sprintf("%d", time.Now().Unix())
			for {
				time.Sleep(2 * time.Second)
				newLogs, err := client.GetContainerLogs(uuid, 0, since, logsTimestamps)
				if err != nil {
					continue
				}
				since = fmt.Sprintf("%d", time.Now().Unix())
				if newLogs != "" {
					fmt.Print(newLogs)
					if newLogs[len(newLogs)-1] != '\n' {
						fmt.Println()
					}
				}
			}
		}

		return nil
	},
}
