package commands

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gpulab/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var statsWatch bool

func init() {
	statsCmd.Flags().BoolVarP(&statsWatch, "watch", "w", false, "Watch mode (refresh every 2s)")
	rootCmd.AddCommand(statsCmd)
}

var statsCmd = &cobra.Command{
	Use:   "stats [UUID]",
	Short: "Show container resource usage",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		uuid, err := client.ResolveContainerUUID(args[0])
		if err != nil {
			return err
		}

		printStats := func() error {
			stats, err := client.GetContainerStats(uuid)
			if err != nil {
				return err
			}

			if flagJSON {
				output.PrintJSON(stats)
				return nil
			}

			fmt.Printf("CPU:     %.1f%%\n", stats.CPUPercentage)
			fmt.Printf("Memory:  %s / %s (%.1f%%)\n",
				output.FormatBytes(stats.MemoryUsage),
				output.FormatBytes(stats.MemoryLimit),
				stats.MemoryPercentage)
			fmt.Printf("Net I/O: %s / %s\n",
				output.FormatBytes(stats.NetworkRx),
				output.FormatBytes(stats.NetworkTx))
			fmt.Printf("Disk:    %s / %s\n",
				output.FormatBytes(stats.BlockRead),
				output.FormatBytes(stats.BlockWrite))
			fmt.Printf("PIDs:    %d\n", stats.PIDs)
			return nil
		}

		if !statsWatch {
			return printStats()
		}

		// Watch mode
		for {
			clearScreen()
			fmt.Printf("Container: %s (every 2s, Ctrl+C to stop)\n\n", uuid[:12])
			if err := printStats(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			time.Sleep(2 * time.Second)
		}
	},
}

func clearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		fmt.Print("\033[H\033[2J")
	}
}
