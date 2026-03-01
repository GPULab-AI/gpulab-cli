package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gpulab/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var execTimeout int

// shellJoin quotes arguments as needed to preserve them through sh -c.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n\"'\\$`!#&|;(){}[]<>?*~") {
			// Single-quote the arg, escaping any embedded single quotes
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		} else {
			quoted[i] = arg
		}
	}
	return strings.Join(quoted, " ")
}

func init() {
	execCmd.Flags().IntVar(&execTimeout, "timeout", 30, "Command timeout in seconds (1-300)")
	rootCmd.AddCommand(execCmd)
}

var execCmd = &cobra.Command{
	Use:   "exec [UUID] -- [COMMAND...]",
	Short: "Execute a command in a container",
	Long:  "Execute a command synchronously in a running container.\nEverything after -- is treated as the command.\n\nExample: gpulab exec abc123 -- nvidia-smi",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		// Find the "--" separator
		uuidArg := args[0]
		var commandParts []string

		// Check if args has command parts after UUID
		dashIdx := -1
		for i, a := range os.Args {
			if a == "--" {
				dashIdx = i
				break
			}
		}

		if dashIdx >= 0 && dashIdx+1 < len(os.Args) {
			commandParts = os.Args[dashIdx+1:]
		} else if len(args) > 1 {
			commandParts = args[1:]
		}

		if len(commandParts) == 0 {
			return fmt.Errorf("no command specified. Usage: gpulab exec UUID -- COMMAND")
		}

		// Shell-quote each argument to preserve spaces and special characters
		command := shellJoin(commandParts)

		uuid, err := client.ResolveContainerUUID(uuidArg)
		if err != nil {
			return err
		}

		// Use a client with extended timeout
		execClient := client.WithTimeout(time.Duration(execTimeout+10) * time.Second)

		result, err := execClient.ExecCommand(uuid, command, execTimeout)
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(result)
			return nil
		}

		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}

		if result.ExitCode != 0 {
			os.Exit(result.ExitCode)
		}

		return nil
	},
}
