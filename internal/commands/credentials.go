package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	credRegistry      string
	credUsername      string
	credPassword      string
	credPasswordStdin bool
	credDeleteForce   bool
)

func init() {
	rootCmd.AddCommand(credentialsCmd)
	credentialsCmd.AddCommand(credentialsListCmd)
	credentialsCmd.AddCommand(credentialsAddCmd)
	credentialsCmd.AddCommand(credentialsDeleteCmd)

	credentialsAddCmd.Flags().StringVar(&credRegistry, "registry", "docker.io", "Registry host (e.g. docker.io, quay.io, ghcr.io)")
	credentialsAddCmd.Flags().StringVar(&credUsername, "username", "", "Registry username")
	credentialsAddCmd.Flags().StringVar(&credPassword, "password", "", "Registry password or access token")
	credentialsAddCmd.Flags().BoolVar(&credPasswordStdin, "password-stdin", false, "Read the password from stdin (keeps it out of shell history)")
	credentialsAddCmd.MarkFlagRequired("username")

	credentialsDeleteCmd.Flags().BoolVar(&credDeleteForce, "force", false, "Skip confirmation")
}

var credentialsCmd = &cobra.Command{
	Use:     "credentials",
	Aliases: []string{"creds"},
	Short:   "Manage Docker registry credentials (for private images)",
	Long: `Store, list, and delete Docker registry credentials used to pull private images.

Reference a credential from a template with 'gpulab templates create --credentials <id>',
or set one inline with --registry-username / --registry-password on create/edit.

Running 'gpulab credentials' with no subcommand lists every credential.`,
	Example: `  gpulab credentials add --registry docker.io --username adhik --password ***
  gpulab credentials add --username adhik --password-stdin < token.txt
  gpulab credentials                                 # list credentials
  gpulab credentials rm 42`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return credentialsListCmd.RunE(cmd, args)
	},
}

var credentialsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List stored registry credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		creds, err := client.ListCredentials()
		if err != nil {
			return err
		}

		switch getOutputFormat() {
		case output.FormatJSON:
			output.PrintJSON(creds)
		case output.FormatQuiet:
			ids := make([]string, len(creds))
			for i, c := range creds {
				ids[i] = strconv.Itoa(c.ID)
			}
			output.PrintQuiet(ids)
		default:
			if len(creds) == 0 {
				fmt.Println("No credentials found. Add one with 'gpulab credentials add'.")
				return nil
			}
			headers := []string{"ID", "REGISTRY", "USERNAME", "CREATED"}
			rows := make([][]string, len(creds))
			for i, c := range creds {
				rows[i] = []string{strconv.Itoa(c.ID), c.Registry, c.Username, c.CreatedAt}
			}
			output.PrintTable(headers, rows)
		}
		return nil
	},
}

var credentialsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a Docker registry credential",
	Long: `Store a Docker registry credential for pulling private images.

The password may be passed with --password or piped in via --password-stdin
(recommended, so it stays out of your shell history).`,
	Example: `  gpulab credentials add --username adhik --password dckr_pat_xxx
  echo "$DOCKER_TOKEN" | gpulab credentials add --username adhik --password-stdin
  gpulab credentials add --registry ghcr.io --username adhik --password ghp_xxx`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		password := credPassword
		if credPasswordStdin {
			data, err := io.ReadAll(bufio.NewReader(os.Stdin))
			if err != nil {
				return fmt.Errorf("failed to read password from stdin: %w", err)
			}
			password = strings.TrimRight(string(data), "\r\n")
		}
		if password == "" {
			return fmt.Errorf("a password is required; pass --password or --password-stdin")
		}

		cred, err := client.CreateCredential(&api.CreateCredentialRequest{
			Registry: credRegistry,
			Username: credUsername,
			Password: password,
		})
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(cred)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Credential created: %d", cred.ID))
		fmt.Printf("Registry: %s\n", cred.Registry)
		fmt.Printf("Username: %s\n", cred.Username)
		fmt.Printf("\nReference it on a template with --credentials %d\n", cred.ID)
		return nil
	},
}

var credentialsDeleteCmd = &cobra.Command{
	Use:     "delete [ID]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a registry credential",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("credential id must be a number, got %q", args[0])
		}

		if !credDeleteForce && !flagJSON && !flagQuiet {
			fmt.Printf("Delete credential %d? [y/N] ", id)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		if err := client.DeleteCredential(id); err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(map[string]interface{}{"status": "success", "action": "deleted", "id": id})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Credential deleted: %d", id))
		return nil
	},
}
