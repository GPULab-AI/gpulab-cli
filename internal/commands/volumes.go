package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	volumeCreateName   string
	volumeCreateSize   int
	volumeCreateRegion string
	volumeCreateType   string
	volumeCreateDesc   string
	volumeDeleteForce  bool
)

func init() {
	rootCmd.AddCommand(volumesCmd)
	volumesCmd.AddCommand(volumesListCmd)
	volumesCmd.AddCommand(volumesInfoCmd)
	volumesCmd.AddCommand(volumesCreateCmd)
	volumesCmd.AddCommand(volumesDeleteCmd)

	volumesCreateCmd.Flags().StringVar(&volumeCreateName, "name", "", "Volume name (required)")
	volumesCreateCmd.Flags().IntVar(&volumeCreateSize, "size", 0, "Volume size in GB (required, 1-1000)")
	volumesCreateCmd.Flags().StringVar(&volumeCreateRegion, "region", "", "Region ID (defaults to the internal region)")
	volumesCreateCmd.Flags().StringVar(&volumeCreateType, "type", "", "Volume type (NVMe|HDD|NVMe_Shared)")
	volumesCreateCmd.Flags().StringVar(&volumeCreateDesc, "description", "", "Volume description")
	volumesCreateCmd.MarkFlagRequired("name")
	volumesCreateCmd.MarkFlagRequired("size")

	volumesDeleteCmd.Flags().BoolVar(&volumeDeleteForce, "force", false, "Skip confirmation")
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

var volumesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create (provision) a new network volume",
	Long: `Create and provision a new network volume.

The volume is provisioned on the default internal region unless --region is
given. Use this to make a scratch/test volume for the 'files' subcommands.`,
	Example: `  gpulab volumes create --name cli-test --size 10
  gpulab volumes create --name data --size 200 --description "training data"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth().WithTimeout(5 * time.Minute)
		volume, err := client.CreateVolume(&api.CreateVolumeRequest{
			VolumeName:  volumeCreateName,
			VolumeSpace: volumeCreateSize,
			RegionID:    volumeCreateRegion,
			VolumeType:  volumeCreateType,
			Description: volumeCreateDesc,
		})
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(volume)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Volume created: %s", volume.VolumeUUID))
		fmt.Printf("Name:   %s\n", volume.VolumeName)
		if volume.MaxSize != nil {
			fmt.Printf("Size:   %d GB\n", *volume.MaxSize)
		}
		fmt.Printf("Status: %s\n", volume.Status)
		return nil
	},
}

var volumesDeleteCmd = &cobra.Command{
	Use:     "delete [VOLUME]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete (deprovision) a network volume",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		if !volumeDeleteForce && !flagJSON && !flagQuiet {
			fmt.Printf("Delete volume %s? This deprovisions its storage. [y/N] ", uuid)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := client.DeleteVolume(uuid); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "uuid": uuid})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Volume deleted: %s", uuid))
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
