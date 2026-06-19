package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	filesUploadDest  string
	filesWriteValue  string
	filesWriteFile   string
	filesDownloadOut string
	filesForceDelete bool
)

func init() {
	volumesCmd.AddCommand(volumesFilesCmd)

	volumesFilesCmd.AddCommand(filesListCmd)
	volumesFilesCmd.AddCommand(filesCatCmd)
	volumesFilesCmd.AddCommand(filesDownloadCmd)
	volumesFilesCmd.AddCommand(filesUploadCmd)
	volumesFilesCmd.AddCommand(filesWriteCmd)
	volumesFilesCmd.AddCommand(filesMkdirCmd)
	volumesFilesCmd.AddCommand(filesRmCmd)
	volumesFilesCmd.AddCommand(filesMvCmd)
	volumesFilesCmd.AddCommand(filesCpCmd)
	volumesFilesCmd.AddCommand(filesSearchCmd)

	filesUploadCmd.Flags().StringVar(&filesUploadDest, "dest", "", "Destination directory inside the volume (default: root)")
	filesWriteCmd.Flags().StringVar(&filesWriteValue, "content", "", "File content (inline)")
	filesWriteCmd.Flags().StringVar(&filesWriteFile, "from-file", "", "Read content from a local file ('-' for stdin)")
	filesDownloadCmd.Flags().StringVarP(&filesDownloadOut, "output", "o", "", "Local output path (default: file basename, '-' for stdout)")
	filesRmCmd.Flags().BoolVar(&filesForceDelete, "force", false, "Skip confirmation")
}

var volumesFilesCmd = &cobra.Command{
	Use:     "files",
	Aliases: []string{"file", "fs"},
	Short:   "Browse and manage files on a network volume",
	Long: `Browse and manage files stored on a network volume.

Every subcommand takes a VOLUME identifier first: a full volume UUID, a UUID
prefix, or the volume name. Paths are relative to the volume root ("" is root).

Examples:
  gpulab volumes files ls my-volume
  gpulab volumes files ls my-volume models/
  gpulab volumes files cat my-volume config.yaml
  gpulab volumes files upload my-volume ./model.safetensors --dest models
  gpulab volumes files write my-volume notes.txt --content "hello"
  echo "data" | gpulab volumes files write my-volume notes.txt --from-file -
  gpulab volumes files rm my-volume old.bin --force`,
}

var filesListCmd = &cobra.Command{
	Use:     "ls [VOLUME] [PATH]",
	Aliases: []string{"list"},
	Short:   "List files in a volume directory",
	Args:    cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		path := ""
		if len(args) == 2 {
			path = args[1]
		}
		resp, err := client.ListVolumeFiles(uuid, path)
		if err != nil {
			return err
		}
		printVolumeFiles(resp)
		return nil
	},
}

var filesSearchCmd = &cobra.Command{
	Use:   "search [VOLUME] [QUERY] [PATH]",
	Short: "Search for files in a volume",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		path := ""
		if len(args) == 3 {
			path = args[2]
		}
		resp, err := client.SearchVolumeFiles(uuid, args[1], path)
		if err != nil {
			return err
		}
		printVolumeFiles(resp)
		return nil
	},
}

var filesCatCmd = &cobra.Command{
	Use:     "cat [VOLUME] [PATH]",
	Aliases: []string{"read"},
	Short:   "Print the contents of a text file",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		content, err := client.GetVolumeFileContent(uuid, args[1])
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"path": args[1], "content": content})
			return nil
		}
		fmt.Print(content)
		if content != "" && !strings.HasSuffix(content, "\n") {
			fmt.Println()
		}
		return nil
	},
}

var filesDownloadCmd = &cobra.Command{
	Use:     "download [VOLUME] [PATH]",
	Aliases: []string{"get"},
	Short:   "Download a file from a volume to local disk",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth().WithTimeout(10 * time.Minute)
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		data, err := client.DownloadVolumeFile(uuid, args[1])
		if err != nil {
			return err
		}

		out := filesDownloadOut
		if out == "" {
			out = baseName(args[1])
		}
		if out == "-" {
			os.Stdout.Write(data)
			return nil
		}
		if err := os.WriteFile(out, data, 0644); err != nil {
			return err
		}
		output.PrintSuccess(fmt.Sprintf("Downloaded %s (%s) -> %s", args[1], formatFileSize(int64(len(data))), out))
		return nil
	},
}

var filesUploadCmd = &cobra.Command{
	Use:     "upload [VOLUME] [LOCAL_FILE...]",
	Aliases: []string{"put"},
	Short:   "Upload local files to a volume directory",
	Args:    cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth().WithTimeout(10 * time.Minute)
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		locals := args[1:]
		for _, p := range locals {
			info, statErr := os.Stat(p)
			if statErr != nil {
				return fmt.Errorf("cannot upload %s: %w", p, statErr)
			}
			if info.IsDir() {
				return fmt.Errorf("cannot upload %s: directories are not supported (zip and use extract)", p)
			}
		}
		resp, err := client.UploadVolumeFiles(uuid, filesUploadDest, locals)
		if err != nil {
			return err
		}
		if flagJSON {
			os.Stdout.Write(resp)
			fmt.Println()
			return nil
		}
		dest := filesUploadDest
		if dest == "" {
			dest = "/"
		}
		output.PrintSuccess(fmt.Sprintf("Uploaded %d file(s) to %s", len(locals), dest))
		return nil
	},
}

var filesWriteCmd = &cobra.Command{
	Use:     "write [VOLUME] [PATH]",
	Aliases: []string{"edit", "save"},
	Short:   "Write/overwrite a text file on a volume",
	Long: `Write or overwrite a text file on a volume.

Content is taken from --content, then --from-file (use '-' for stdin), and
falls back to reading stdin when neither flag is given.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}

		content, err := resolveWriteContent(cmd)
		if err != nil {
			return err
		}

		if err := client.SaveVolumeFileContent(uuid, args[1], content); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]interface{}{"status": "success", "path": args[1], "bytes": len(content)})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Wrote %s (%s)", args[1], formatFileSize(int64(len(content)))))
		return nil
	},
}

var filesMkdirCmd = &cobra.Command{
	Use:   "mkdir [VOLUME] [PATH]",
	Short: "Create a directory on a volume",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		if err := client.CreateVolumeDirectory(uuid, args[1]); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "path": args[1]})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Created directory %s", args[1]))
		return nil
	},
}

var filesRmCmd = &cobra.Command{
	Use:     "rm [VOLUME] [PATH]",
	Aliases: []string{"delete", "remove"},
	Short:   "Delete a file or directory from a volume",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		if !filesForceDelete && !flagJSON && !flagQuiet {
			fmt.Printf("Delete %q from volume %s? [y/N] ", args[1], shortID(uuid))
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := client.DeleteVolumePath(uuid, args[1]); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "path": args[1]})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Deleted %s", args[1]))
		return nil
	},
}

var filesMvCmd = &cobra.Command{
	Use:     "mv [VOLUME] [OLD_PATH] [NEW_PATH]",
	Aliases: []string{"rename", "move"},
	Short:   "Rename or move a file/directory on a volume",
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		if err := client.RenameVolumePath(uuid, args[1], args[2]); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "old_path": args[1], "new_path": args[2]})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Moved %s -> %s", args[1], args[2]))
		return nil
	},
}

var filesCpCmd = &cobra.Command{
	Use:     "cp [VOLUME] [SOURCE_PATH] [DEST_PATH]",
	Aliases: []string{"copy"},
	Short:   "Copy a file/directory on a volume",
	Args:    cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		uuid, err := client.ResolveVolumeUUID(args[0])
		if err != nil {
			return err
		}
		if err := client.CopyVolumePath(uuid, args[1], args[2]); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "source_path": args[1], "dest_path": args[2]})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Copied %s -> %s", args[1], args[2]))
		return nil
	},
}

func resolveWriteContent(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("content") {
		return filesWriteValue, nil
	}
	if filesWriteFile != "" {
		if filesWriteFile == "-" {
			data, err := io.ReadAll(os.Stdin)
			return string(data), err
		}
		data, err := os.ReadFile(filesWriteFile)
		return string(data), err
	}
	// Fall back to stdin so `... write vol path < file` works.
	data, err := io.ReadAll(os.Stdin)
	return string(data), err
}

func printVolumeFiles(resp *api.VolumeFileListResponse) {
	switch getOutputFormat() {
	case output.FormatJSON:
		output.PrintJSON(resp)
	case output.FormatQuiet:
		paths := make([]string, len(resp.Items))
		for i, item := range resp.Items {
			paths[i] = item.Path
		}
		output.PrintQuiet(paths)
	default:
		if len(resp.Items) == 0 {
			fmt.Println("No files found.")
			return
		}
		headers := []string{"TYPE", "NAME", "SIZE", "MODIFIED"}
		rows := make([][]string, len(resp.Items))
		for i, item := range resp.Items {
			kind := "file"
			size := formatFileSize(item.Size)
			if item.IsDirectory {
				kind = "dir"
				size = "-"
			}
			rows[i] = []string{kind, item.Name, size, shortTime(item.Modified)}
		}
		output.PrintTable(headers, rows)
		if resp.Info != nil {
			fmt.Printf("%d folder(s), %d file(s)\n", resp.Info.Directories, resp.Info.Files)
		}
	}
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func baseName(p string) string {
	p = strings.TrimRight(p, "/")
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		return p[idx+1:]
	}
	return p
}
