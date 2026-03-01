package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/gpulab/gpulab-cli/internal/api"
	"github.com/gpulab/gpulab-cli/internal/config"
	"github.com/gpulab/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(whoamiCmd)
	authCmd.AddCommand(statusCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login with an API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		apiKey := flagAPIKey

		if apiKey == "" {
			// Try env var
			apiKey = os.Getenv("GPULAB_API_KEY")
		}

		if apiKey == "" {
			fmt.Print("Enter your API key: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			apiKey = strings.TrimSpace(input)
		}

		if apiKey == "" {
			return fmt.Errorf("API key is required")
		}

		// Validate the key by calling /account
		apiURL := config.GetAPIURL(flagAPIURL)
		client := api.NewClient(apiURL, apiKey, flagDebug)
		account, err := client.GetAccount()
		if err != nil {
			return fmt.Errorf("invalid API key: %w", err)
		}

		// Save to config
		cfg, _ := config.Load()
		cfg.APIKey = apiKey
		if flagAPIURL != "" {
			cfg.APIURL = flagAPIURL
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		output.PrintSuccess(fmt.Sprintf("Logged in as %s (%s)", account.Name, account.Email))
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		cfg.APIKey = ""
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		output.PrintSuccess("Logged out successfully")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user info",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		account, err := client.GetAccount()
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(account)
			return nil
		}

		fmt.Printf("Name:       %s\n", account.Name)
		fmt.Printf("Email:      %s\n", account.Email)
		if account.Team != nil {
			fmt.Printf("Team:       %s\n", account.Team.Name)
		}
		fmt.Printf("Containers: %d\n", account.ContainerCount)
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()

		if flagJSON {
			output.PrintJSON(map[string]interface{}{
				"api_url":       cfg.APIURL,
				"authenticated": cfg.APIKey != "",
				"config_path":   config.ConfigPath(),
			})
			return nil
		}

		fmt.Printf("Config:  %s\n", config.ConfigPath())
		fmt.Printf("API URL: %s\n", cfg.APIURL)
		if cfg.APIKey != "" {
			// Show masked key
			masked := cfg.APIKey[:8] + "..." + cfg.APIKey[len(cfg.APIKey)-4:]
			fmt.Printf("API Key: %s\n", masked)
		} else {
			fmt.Printf("API Key: (not set)\n")
		}
		return nil
	},
}
