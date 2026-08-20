package commands

import (
	"fmt"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
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

// placeableNote explains a gap between free and placeable GPUs. Reporting free
// GPUs as if they were usable capacity sent an operator to reallocate GPUs that
// no endpoint could ever be scheduled onto.
func placeableNote(gap int) string {
	return output.Dim(fmt.Sprintf(
		"FREE counts every unassigned online GPU. PLACEABLE counts only those on a host the "+
			"scheduler will use, which is what an endpoint can actually get.\n"+
			"%d free GPU(s) sit on hosts that take no work. An endpoint's own region and CUDA "+
			"filters can lower PLACEABLE further.", gap))
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

		// A failure here is not fatal: the free count is still worth showing,
		// as long as it is not labelled as capacity the scheduler can use.
		placeable, placeableErr := client.PlaceableGPUCounts()
		if placeableErr == nil {
			for i := range types {
				if count, ok := placeable[api.GPUTypeKey{Name: types[i].GPUName, MemoryMB: types[i].GPUTotalMemory}]; ok {
					types[i].PlaceableCount = &count
				}
			}
		}

		switch getOutputFormat() {
		case output.FormatJSON:
			output.PrintJSON(types)
		default:
			if len(types) == 0 {
				fmt.Println("No GPU types found.")
				return nil
			}
			headers := []string{"GPU", "MEMORY", "PRICE", "FREE", "PLACEABLE"}
			rows := make([][]string, len(types))
			gap := 0
			for i, t := range types {
				price := "-"
				if pf, ok := t.GPUPrice.Float64(); ok && pf > 0 {
					price = fmt.Sprintf("$%.2f/hr", pf)
				}
				placeableCell := "?"
				if t.PlaceableCount != nil {
					placeableCell = fmt.Sprintf("%d", *t.PlaceableCount)
					if *t.PlaceableCount < t.AvailableCount {
						gap += t.AvailableCount - *t.PlaceableCount
						placeableCell = output.Warn(placeableCell)
					}
				}
				rows[i] = []string{
					t.GPUName,
					fmt.Sprintf("%d MB", t.GPUTotalMemory),
					price,
					fmt.Sprintf("%d", t.AvailableCount),
					placeableCell,
				}
			}
			output.PrintTable(headers, rows)
			switch {
			case placeableErr != nil:
				fmt.Printf("\n%s\n", output.Dim(
					"PLACEABLE unknown: could not read serverless options. FREE counts GPUs on hosts "+
						"the scheduler may refuse, so treat it as an upper bound."))
			case gap > 0:
				fmt.Printf("\n%s\n", placeableNote(gap))
			}
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

		// The API's own summary merges GPU sizes under one label; regroup it
		// before anything is counted or joined against it.
		if regrouped := resp.SummaryByType(); regrouped != nil {
			resp.Summary = regrouped
		}

		placeable, placeableErr := client.PlaceableGPUCounts()
		if placeableErr == nil {
			total := 0
			for i := range resp.Summary {
				if count, ok := placeable[api.GPUTypeKey{Name: resp.Summary[i].GPUName, MemoryMB: resp.Summary[i].MemoryMB}]; ok {
					resp.Summary[i].PlaceableCount = &count
					total += count
				}
			}
			resp.TotalPlaceable = &total
		}

		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}

		fmt.Printf("Total free: %d\n", resp.TotalAvailable)
		if resp.TotalPlaceable != nil {
			fmt.Printf("Total placeable: %d\n", *resp.TotalPlaceable)
		}
		fmt.Println()
		if len(resp.Summary) > 0 {
			headers := []string{"GPU", "MEMORY", "FREE", "PLACEABLE"}
			rows := make([][]string, len(resp.Summary))
			gap := 0
			for i, s := range resp.Summary {
				placeableCell := "?"
				if s.PlaceableCount != nil {
					placeableCell = fmt.Sprintf("%d", *s.PlaceableCount)
					if *s.PlaceableCount < s.Count {
						gap += s.Count - *s.PlaceableCount
						placeableCell = output.Warn(placeableCell)
					}
				}
				rows[i] = []string{
					s.GPUName,
					fmt.Sprintf("%d MB", s.MemoryMB),
					fmt.Sprintf("%d", s.Count),
					placeableCell,
				}
			}
			output.PrintTable(headers, rows)
			if gap > 0 {
				fmt.Printf("\n%s\n", placeableNote(gap))
			}
		}
		return nil
	},
}
