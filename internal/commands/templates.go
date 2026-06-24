package commands

import (
	"fmt"
	"strings"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	tplName        string
	tplImage       string
	tplDescription string
	tplVisibility  string
	tplType        string
	tplCategory    int
	tplCredentials int
	tplMountPath   string
	tplPorts       string
	tplEnv         []string
	tplEnvFile     string
	tplCommand     string
	tplDisk        int
	tplVolumeDisk  int
	tplMemory      int
	tplPullPolicy  string
	tplAuthorName  string
	tplAuthorURL   string
	tplThumbnail   string
	tplNotes       string
	tplDeleteForce bool
)

func init() {
	rootCmd.AddCommand(templatesCmd)
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesInfoCmd)
	templatesCmd.AddCommand(templatesCategoriesCmd)
	templatesCmd.AddCommand(templatesCreateCmd)
	templatesCmd.AddCommand(templatesEditCmd)
	templatesCmd.AddCommand(templatesDeleteCmd)

	addTemplateWriteFlags(templatesCreateCmd)
	addTemplateWriteFlags(templatesEditCmd)
	templatesCreateCmd.MarkFlagRequired("name")
	templatesCreateCmd.MarkFlagRequired("image")

	templatesDeleteCmd.Flags().BoolVar(&tplDeleteForce, "force", false, "Skip confirmation")
}

// addTemplateWriteFlags registers the flags shared by create and edit. Defaults
// are left empty so the edit command can detect which fields actually changed;
// the API fills in defaults (visibility=private, type=gpu, disk=10) on create.
func addTemplateWriteFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&tplName, "name", "", "Template name")
	f.StringVar(&tplImage, "image", "", "Docker image (e.g. repository/image:tag)")
	f.StringVar(&tplDescription, "description", "", "Short description")
	f.StringVar(&tplVisibility, "visibility", "", "Visibility: public|private (default private)")
	f.StringVar(&tplType, "type", "", "Container type: gpu|cpu (default gpu)")
	f.IntVar(&tplCategory, "category", 0, "Category ID (see 'gpulab templates categories')")
	f.IntVar(&tplCredentials, "credentials", 0, "Docker credentials ID (for private images)")
	f.StringVar(&tplMountPath, "mount-path", "", "Volume mount path inside the container")
	f.StringVar(&tplPorts, "ports", "", "Exposed ports, comma-separated (e.g. 80,443,8080-8085)")
	f.StringArrayVarP(&tplEnv, "env", "e", nil, "Environment variable KEY=VALUE (repeatable; bare KEY inherits from host)")
	f.StringVar(&tplEnvFile, "env-file", "", "Read environment variables from a .env file")
	f.StringVar(&tplCommand, "command", "", "Container start command")
	f.IntVar(&tplDisk, "disk", 0, "Container disk size in GB (1-300, default 10)")
	f.IntVar(&tplVolumeDisk, "volume-disk", 0, "Volume disk size in GB (1-300)")
	f.IntVar(&tplMemory, "memory", 0, "Memory limit in GB (1-40)")
	f.StringVar(&tplPullPolicy, "pull-policy", "", "Image pull policy: cached|latest|digest")
	f.StringVar(&tplAuthorName, "author-name", "", "Author name")
	f.StringVar(&tplAuthorURL, "author-url", "", "Author URL")
	f.StringVar(&tplThumbnail, "thumbnail", "", "Thumbnail URL")
	f.StringVar(&tplNotes, "notes", "", "Free-form notes")
}

var templateWriteFlagNames = []string{
	"name", "image", "description", "visibility", "type", "category", "credentials",
	"mount-path", "ports", "env", "env-file", "command", "disk", "volume-disk",
	"memory", "pull-policy", "author-name", "author-url", "thumbnail", "notes",
}

// buildTemplateRequest assembles a create/update payload from the flags the user
// actually set, so edit performs a true partial update.
func buildTemplateRequest(cmd *cobra.Command) (*api.TemplateRequest, error) {
	f := cmd.Flags()
	req := &api.TemplateRequest{}

	if f.Changed("name") {
		req.Name = tplName
	}
	if f.Changed("image") {
		req.DockerImage = tplImage
	}
	if f.Changed("visibility") {
		req.Visibility = tplVisibility
	}
	if f.Changed("type") {
		req.ContainerType = tplType
	}
	if f.Changed("pull-policy") {
		req.ImagePullPolicy = tplPullPolicy
	}
	if f.Changed("description") {
		v := tplDescription
		req.Description = &v
	}
	if f.Changed("author-name") {
		v := tplAuthorName
		req.AuthorName = &v
	}
	if f.Changed("author-url") {
		v := tplAuthorURL
		req.AuthorURL = &v
	}
	if f.Changed("thumbnail") {
		v := tplThumbnail
		req.Thumbnail = &v
	}
	if f.Changed("mount-path") {
		v := tplMountPath
		req.VolumeMountPath = &v
	}
	if f.Changed("ports") {
		v := tplPorts
		req.ExposedPorts = &v
	}
	if f.Changed("command") {
		v := tplCommand
		req.Command = &v
	}
	if f.Changed("notes") {
		v := tplNotes
		req.Notes = &v
	}
	if f.Changed("category") {
		v := tplCategory
		req.CategoryID = &v
	}
	if f.Changed("credentials") {
		v := tplCredentials
		req.CredentialsID = &v
	}
	if f.Changed("disk") {
		v := tplDisk
		req.ContainerDiskSize = &v
	}
	if f.Changed("volume-disk") {
		v := tplVolumeDisk
		req.VolumeDiskSize = &v
	}
	if f.Changed("memory") {
		v := tplMemory
		req.MemoryLimit = &v
	}
	if f.Changed("env") || f.Changed("env-file") {
		env, err := buildEnvVars(tplEnvFile, tplEnv)
		if err != nil {
			return nil, err
		}
		if env != nil {
			req.EnvironmentVariables = env
		}
	}

	return req, nil
}

func anyTemplateWriteFlagChanged(cmd *cobra.Command) bool {
	for _, name := range templateWriteFlagNames {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage container templates (Docker images)",
	Long: `List, inspect, create, edit, and delete container templates (Docker images).

Running 'gpulab templates' with no subcommand lists every template.`,
	Example: `  gpulab templates                                  # list templates
  gpulab templates info my-template                 # show one template
  gpulab templates create --name web --image nginx:latest --ports 80
  gpulab templates edit my-template --image nginx:1.27
  gpulab templates delete my-template`,
	RunE: templatesListCmd.RunE,
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
				rows[i] = []string{t.TemplateUUID, t.Name, t.DockerImage, t.ContainerType, t.Visibility}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

var templatesCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List template categories (for --category on create/edit)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		categories, err := client.ListTemplateCategories()
		if err != nil {
			return err
		}

		switch getOutputFormat() {
		case output.FormatJSON:
			output.PrintJSON(categories)
		case output.FormatQuiet:
			ids := make([]string, len(categories))
			for i, c := range categories {
				ids[i] = fmt.Sprintf("%d", c.ID)
			}
			output.PrintQuiet(ids)
		default:
			if len(categories) == 0 {
				fmt.Println("No categories found.")
				return nil
			}
			headers := []string{"ID", "NAME", "DESCRIPTION"}
			rows := make([][]string, len(categories))
			for i, c := range categories {
				rows[i] = []string{fmt.Sprintf("%d", c.ID), c.Name, c.Description}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

var templatesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new template",
	Long: `Create a new container template (Docker image).

Only --name and --image are required; visibility defaults to private, container
type to gpu, and disk to 10GB unless overridden.`,
	Example: `  gpulab templates create --name web --image nginx:latest --ports 80
  gpulab templates create --name trainer --image pytorch/pytorch:latest \
    --type gpu --memory 24 --disk 50 -e WANDB_API_KEY=xxx --mount-path /workspace`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		req, err := buildTemplateRequest(cmd)
		if err != nil {
			return err
		}
		tpl, err := client.CreateTemplate(req)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(tpl)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Template created: %s", tpl.TemplateUUID))
		printTemplateSummary(tpl)
		return nil
	},
}

var templatesEditCmd = &cobra.Command{
	Use:     "edit [UUID|name]",
	Aliases: []string{"update"},
	Short:   "Edit an existing template",
	Long: `Update fields on an existing template. Only the flags you pass are changed.

The target may be a full UUID, a UUID prefix, or the template name.`,
	Example: `  gpulab templates edit my-template --image nginx:1.27
  gpulab templates edit pw6lrxJtvn --visibility public --ports 80,443`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		if !anyTemplateWriteFlagChanged(cmd) {
			return fmt.Errorf("no changes specified; pass at least one field to update (e.g. --image)")
		}
		uuid, err := client.ResolveTemplateUUID(args[0])
		if err != nil {
			return err
		}
		req, err := buildTemplateRequest(cmd)
		if err != nil {
			return err
		}
		tpl, err := client.UpdateTemplate(uuid, req)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(tpl)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Template updated: %s", tpl.TemplateUUID))
		printTemplateSummary(tpl)
		return nil
	},
}

var templatesDeleteCmd = &cobra.Command{
	Use:     "delete [UUID|name]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a template",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveTemplateUUID(args[0])
		if err != nil {
			return err
		}
		if !tplDeleteForce && !flagJSON && !flagQuiet {
			fmt.Printf("Delete template %s? [y/N] ", uuid)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := client.DeleteTemplate(uuid); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "uuid": uuid})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Template deleted: %s", uuid))
		return nil
	},
}

var templatesInfoCmd = &cobra.Command{
	Use:   "info [UUID|name]",
	Short: "Show template details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveTemplateUUID(args[0])
		if err != nil {
			return err
		}
		template, err := client.GetTemplate(uuid)
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
		if template.ExposedPorts != nil && *template.ExposedPorts != "" {
			fmt.Printf("Ports:       %s\n", *template.ExposedPorts)
		}
		if template.VolumeMountPath != "" {
			fmt.Printf("Mount Path:  %s\n", template.VolumeMountPath)
		}
		if template.ContainerDiskSize != "" {
			fmt.Printf("Disk:        %s GB\n", template.ContainerDiskSize)
		}
		if template.VolumeDiskSize != "" {
			fmt.Printf("Volume Disk: %s GB\n", template.VolumeDiskSize)
		}
		if template.MemoryLimit != nil {
			fmt.Printf("Memory:      %d GB\n", *template.MemoryLimit)
		}
		if template.Command != nil && *template.Command != "" {
			fmt.Printf("Command:     %s\n", *template.Command)
		}
		if len(template.EnvironmentVariables) > 0 {
			fmt.Printf("Environment: %d variable(s)\n", len(template.EnvironmentVariables))
			for k, v := range template.EnvironmentVariables {
				fmt.Printf("  %s=%s\n", k, v)
			}
		}
		if template.Notes != "" {
			fmt.Printf("Notes:       %s\n", template.Notes)
		}
		return nil
	},
}

// printTemplateSummary prints a short summary after a create/edit.
func printTemplateSummary(t *api.Template) {
	fmt.Printf("Name:       %s\n", t.Name)
	fmt.Printf("Image:      %s\n", t.DockerImage)
	fmt.Printf("Type:       %s\n", t.ContainerType)
	fmt.Printf("Visibility: %s\n", t.Visibility)
	if t.ExposedPorts != nil && *t.ExposedPorts != "" {
		fmt.Printf("Ports:      %s\n", *t.ExposedPorts)
	}
}
