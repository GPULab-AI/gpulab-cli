package commands

import (
	"fmt"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(volumesCmd)
	volumesCmd.AddCommand(volumesListCmd)
	volumesCmd.AddCommand(volumesInfoCmd)
}

var volumesCmd = &cobra.Command{
	Use:     "volumes",
	Aliases: []string{"vol"},
	Short:   "Manage network volumes and their files",
	Long: `List network volumes and manage the files stored on them.

Running 'gpulab volumes' with no subcommand lists every volume (all pages).
Use the 'files' subcommand to browse, upload, edit, and delete files.`,
	Example: `  gpulab volumes                       # list all volumes
  gpulab volumes info my-volume        # show one volume
  gpulab volumes files ls my-volume    # list files on a volume
  gpulab volumes files upload my-volume ./data.bin --dest datasets`,
	RunE: volumesListCmd.RunE,
}

var volumesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List volumes",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		volumes, err := client.ListVolumes()
		if err != nil {
			return err
		}

		switch getOutputFormat() {
		case output.FormatJSON:
			output.PrintJSON(volumes)
		case output.FormatQuiet:
			uuids := make([]string, len(volumes))
			for i, v := range volumes {
				uuids[i] = v.VolumeUUID
			}
			output.PrintQuiet(uuids)
		default:
			if len(volumes) == 0 {
				fmt.Println("No volumes found.")
				return nil
			}
			headers := []string{"UUID", "NAME", "SIZE", "STATUS"}
			rows := make([][]string, len(volumes))
			for i, v := range volumes {
				uuid := v.VolumeUUID
				if len(uuid) > 12 {
					uuid = uuid[:12]
				}
				size := "-"
				if v.MaxSize != nil {
					size = fmt.Sprintf("%d GB", *v.MaxSize)
				}
				rows[i] = []string{uuid, v.VolumeName, size, v.Status}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

var volumesInfoCmd = &cobra.Command{
	Use:   "info [UUID]",
	Short: "Show volume details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		volume, err := client.GetVolume(args[0])
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(volume)
			return nil
		}

		fmt.Printf("UUID:   %s\n", volume.VolumeUUID)
		fmt.Printf("Name:   %s\n", volume.VolumeName)
		fmt.Printf("Status: %s\n", volume.Status)
		if volume.MaxSize != nil {
			fmt.Printf("Size:   %d GB\n", *volume.MaxSize)
		}
		if volume.UsedSize != nil {
			fmt.Printf("Used:   %d GB\n", *volume.UsedSize)
		}
		return nil
	},
}
