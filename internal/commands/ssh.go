package commands

import (
	"fmt"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/GPULab-AI/gpulab-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(sshCmd)
}

var sshCmd = &cobra.Command{
	Use:   "ssh [UUID]",
	Short: "Open an interactive terminal in a container",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		uuid, err := client.ResolveContainerUUID(args[0])
		if err != nil {
			return err
		}

		// Start terminal on server
		info, err := client.StartTerminal(uuid)
		if err != nil {
			return fmt.Errorf("failed to start terminal: %w", err)
		}

		terminalURL := ""
		if info.Terminal.TerminalURL != "" {
			terminalURL = info.Terminal.TerminalURL
		}

		if terminalURL == "" {
			return fmt.Errorf("no terminal URL returned")
		}

		// Convert HTTP URL to WebSocket URL
		wsURL := terminal.HTTPToWS(terminalURL) + "/ws"

		if flagJSON {
			output.PrintJSON(map[string]string{
				"terminal_url": terminalURL,
				"ws_url":       wsURL,
			})
			return nil
		}

		fmt.Printf("Connecting to %s...\n", uuid[:12])

		return terminal.Connect(wsURL)
	},
}
