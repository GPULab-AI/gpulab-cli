package commands

import (
	"fmt"
	"strings"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(inspectCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(redeployCmd)

	rmCmd.Flags().BoolVar(&forceDelete, "force", false, "Skip confirmation")
	rootCmd.AddCommand(rmCmd)
}

var forceDelete bool

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list", "ps"},
	Short:   "List all containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		containers, err := client.ListContainers()
		if err != nil {
			return err
		}

		format := getOutputFormat()

		switch format {
		case output.FormatJSON:
			output.PrintJSON(containers)
		case output.FormatQuiet:
			uuids := make([]string, len(containers))
			for i, c := range containers {
				uuids[i] = c.UUID
			}
			output.PrintQuiet(uuids)
		default:
			if len(containers) == 0 {
				fmt.Println("No containers found.")
				return nil
			}
			headers := []string{"UUID", "NAME", "STATUS", "TYPE", "GPUS", "CREATED"}
			rows := make([][]string, len(containers))
			for i, c := range containers {
				gpuInfo := "-"
				if c.GPUCount != nil && *c.GPUCount > 0 {
					gpuInfo = fmt.Sprintf("%d", *c.GPUCount)
					if len(c.GPUs) > 0 && c.GPUs[0].Type != nil {
						if gpuType, ok := c.GPUs[0].Type.(map[string]interface{}); ok {
							if name, ok := gpuType["gpu_name"].(string); ok {
								gpuInfo = name
							}
						}
					}
				}
				// Shorten UUID for display
				shortUUID := c.UUID
				if len(shortUUID) > 12 {
					shortUUID = shortUUID[:12]
				}
				created := c.CreatedAt
				if len(created) > 16 {
					created = created[:16] // YYYY-MM-DDTHH:MM
				}
				rows[i] = []string{
					shortUUID,
					c.ServerName,
					output.StatusColor(c.Status),
					c.Type,
					gpuInfo,
					created,
				}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

var inspectCmd = &cobra.Command{
	Use:   "inspect [UUID]",
	Short: "Show detailed container information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		uuid, err := client.ResolveContainerUUID(args[0])
		if err != nil {
			return err
		}

		container, err := client.GetContainer(uuid)
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(container)
			return nil
		}

		fmt.Printf("UUID:        %s\n", container.UUID)
		fmt.Printf("Name:        %s\n", container.ServerName)
		fmt.Printf("Status:      %s\n", output.StatusColor(container.Status))
		fmt.Printf("Type:        %s\n", container.Type)
		if container.Memory != nil {
			fmt.Printf("Memory:      %d GB\n", *container.Memory)
		}
		if container.AllocatedCPUCores != nil {
			fmt.Printf("CPU Cores:   %d\n", *container.AllocatedCPUCores)
		}
		if container.OpenedPorts != nil && *container.OpenedPorts != "" {
			fmt.Printf("Ports:       %s\n", *container.OpenedPorts)
		}
		if container.VolumeMountPath != nil && *container.VolumeMountPath != "" {
			fmt.Printf("Volume:      %s\n", *container.VolumeMountPath)
		}
		if container.Command != nil && *container.Command != "" {
			fmt.Printf("Command:     %s\n", *container.Command)
		}
		if container.PublicURLs != nil && *container.PublicURLs != "" {
			fmt.Printf("URLs:        %s\n", *container.PublicURLs)
		}

		if container.WebTerminal != nil {
			if wt, ok := container.WebTerminal.(map[string]interface{}); ok {
				if url, ok := wt["terminal_url"].(string); ok {
					fmt.Printf("Terminal:    %s\n", url)
				}
			}
		}

		// GPU info
		if len(container.GPUs) > 0 {
			fmt.Printf("\nGPUs:\n")
			for _, gpu := range container.GPUs {
				name := "Unknown"
				if gpu.Type != nil {
					if gpuType, ok := gpu.Type.(map[string]interface{}); ok {
						if n, ok := gpuType["gpu_name"].(string); ok {
							name = n
						}
					}
				}
				mem := ""
				if m, err := gpu.TotalMemory.Int64(); err == nil && m > 0 {
					mem = fmt.Sprintf(" (%d MB)", m)
				}
				fmt.Printf("  [%s] %s%s - %s\n", gpu.GPUIndex, name, mem, gpu.GPUStatus)
			}
		}

		// Template info
		if container.Template != nil {
			if tmpl, ok := container.Template.(map[string]interface{}); ok {
				fmt.Printf("\nTemplate:\n")
				if name, ok := tmpl["name"].(string); ok {
					fmt.Printf("  Name:      %s\n", name)
				}
				if image, ok := tmpl["docker_image"].(string); ok {
					fmt.Printf("  Image:     %s\n", image)
				}
			}
		}

		fmt.Printf("\nCreated:     %s\n", container.CreatedAt)
		fmt.Printf("Updated:     %s\n", container.UpdatedAt)

		return nil
	},
}

func containerActionCmd(use, short, action string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " [UUID]",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := requireAuth()

			uuid, err := client.ResolveContainerUUID(args[0])
			if err != nil {
				return err
			}

			var actionErr error
			switch action {
			case "stop":
				actionErr = client.StopContainer(uuid)
			case "start":
				actionErr = client.StartContainer(uuid)
			case "restart":
				actionErr = client.RestartContainer(uuid)
			case "redeploy":
				actionErr = client.RedeployContainer(uuid)
			}

			if actionErr != nil {
				return actionErr
			}

			if flagJSON {
				output.PrintJSON(map[string]string{"status": "success", "action": action, "uuid": uuid})
			} else {
				pastTense := map[string]string{
					"stop": "stopped", "start": "started", "restart": "restarted", "redeploy": "redeployed",
				}[action]
				output.PrintSuccess(fmt.Sprintf("Container %s: %s", pastTense, uuid[:12]))
			}
			return nil
		},
	}
}

var stopCmd = containerActionCmd("stop", "Stop a running container", "stop")
var startCmd = containerActionCmd("start", "Start a stopped container", "start")
var restartCmd = containerActionCmd("restart", "Restart a container", "restart")
var redeployCmd = containerActionCmd("redeploy", "Redeploy a container", "redeploy")

var rmCmd = &cobra.Command{
	Use:     "rm [UUID]",
	Aliases: []string{"delete", "remove"},
	Short:   "Delete a container",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		uuid, err := client.ResolveContainerUUID(args[0])
		if err != nil {
			return err
		}

		if !forceDelete {
			fmt.Printf("Delete container %s? [y/N] ", uuid[:12])
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := client.DeleteContainer(uuid); err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "uuid": uuid})
		} else {
			output.PrintSuccess(fmt.Sprintf("Container deleted: %s", uuid[:12]))
		}
		return nil
	},
}
