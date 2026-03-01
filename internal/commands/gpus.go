package commands

import (
	"fmt"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(gpusCmd)
	gpusCmd.AddCommand(gpusTypesCmd)
	gpusCmd.AddCommand(gpusAvailableCmd)
}

var gpusCmd = &cobra.Command{
	Use:   "gpus",
	Short: "GPU information",
	RunE:  gpusTypesCmd.RunE,
}

var gpusTypesCmd = &cobra.Command{
	Use:   "types",
	Short: "List available GPU types",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		types, err := client.ListGPUTypes()
		if err != nil {
			return err
		}

		switch getOutputFormat() {
		case output.FormatJSON:
			output.PrintJSON(types)
		default:
			if len(types) == 0 {
				fmt.Println("No GPU types found.")
				return nil
			}
			headers := []string{"GPU", "MEMORY", "PRICE", "AVAILABLE"}
			rows := make([][]string, len(types))
			for i, t := range types {
				price := "-"
				if pf, err := t.GPUPrice.Float64(); err == nil && pf > 0 {
					price = fmt.Sprintf("$%.2f/hr", pf)
				}
				rows[i] = []string{
					t.GPUName,
					fmt.Sprintf("%d MB", t.GPUTotalMemory),
					price,
					fmt.Sprintf("%d", t.AvailableCount),
				}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

var gpusAvailableCmd = &cobra.Command{
	Use:   "available",
	Short: "List available GPUs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		resp, err := client.GetAvailableGPUs()
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}

		fmt.Printf("Total available: %d\n\n", resp.TotalAvailable)
		if len(resp.Summary) > 0 {
			headers := []string{"GPU", "COUNT", "MEMORY"}
			rows := make([][]string, len(resp.Summary))
			for i, s := range resp.Summary {
				rows[i] = []string{
					s.GPUName,
					fmt.Sprintf("%d", s.Count),
					fmt.Sprintf("%d MB", s.MemoryMB),
				}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}
