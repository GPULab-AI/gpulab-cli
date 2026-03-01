package commands

import (
	"fmt"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(templatesCmd)
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesInfoCmd)
}

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage container templates",
	RunE:  templatesListCmd.RunE,
}

var templatesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		templates, err := client.ListTemplates()
		if err != nil {
			return err
		}

		switch getOutputFormat() {
		case output.FormatJSON:
			output.PrintJSON(templates)
		case output.FormatQuiet:
			uuids := make([]string, len(templates))
			for i, t := range templates {
				uuids[i] = t.TemplateUUID
			}
			output.PrintQuiet(uuids)
		default:
			if len(templates) == 0 {
				fmt.Println("No templates found.")
				return nil
			}
			headers := []string{"UUID", "NAME", "IMAGE", "TYPE", "VISIBILITY"}
			rows := make([][]string, len(templates))
			for i, t := range templates {
				uuid := t.TemplateUUID
				if len(uuid) > 12 {
					uuid = uuid[:12]
				}
				rows[i] = []string{uuid, t.Name, t.DockerImage, t.ContainerType, t.Visibility}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

var templatesInfoCmd = &cobra.Command{
	Use:   "info [UUID]",
	Short: "Show template details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		template, err := client.GetTemplate(args[0])
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(template)
			return nil
		}

		fmt.Printf("UUID:        %s\n", template.TemplateUUID)
		fmt.Printf("Name:        %s\n", template.Name)
		fmt.Printf("Image:       %s\n", template.DockerImage)
		fmt.Printf("Type:        %s\n", template.ContainerType)
		fmt.Printf("Visibility:  %s\n", template.Visibility)
		if template.Description != "" {
			fmt.Printf("Description: %s\n", template.Description)
		}
		if template.ExposedPorts != nil {
			fmt.Printf("Ports:       %s\n", *template.ExposedPorts)
		}
		if template.Command != nil {
			fmt.Printf("Command:     %s\n", *template.Command)
		}
		return nil
	},
}
