package commands

import (
	"fmt"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		if flagJSON {
			output.PrintJSON(map[string]string{
				"version": versionStr,
				"commit":  commitStr,
				"date":    dateStr,
			})
			return
		}
		fmt.Printf("gpulab version %s\n", versionStr)
		fmt.Printf("  commit: %s\n", commitStr)
		fmt.Printf("  built:  %s\n", dateStr)
	},
}
