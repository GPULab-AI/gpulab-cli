package commands

import (
	"fmt"
	"os"

	"github.com/gpulab/gpulab-cli/internal/api"
	"github.com/gpulab/gpulab-cli/internal/config"
	"github.com/gpulab/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	versionStr string
	commitStr  string
	dateStr    string

	flagJSON   bool
	flagQuiet  bool
	flagAPIKey string
	flagAPIURL string
	flagDebug  bool
)

func SetVersionInfo(version, commit, date string) {
	versionStr = version
	commitStr = commit
	dateStr = date
}

var rootCmd = &cobra.Command{
	Use:   "gpulab",
	Short: "GPULab CLI - Manage GPU containers from your terminal",
	Long:  "GPULab CLI provides command-line access to GPULab GPU containers.\nDeploy, manage, and interact with containers running on GPU servers.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.PrintError(err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "Quiet output (IDs only)")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key (overrides config/env)")
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "API base URL (overrides config/env)")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "Enable debug output")
}

func getOutputFormat() output.Format {
	if flagJSON {
		return output.FormatJSON
	}
	if flagQuiet {
		return output.FormatQuiet
	}
	return output.FormatTable
}

func newAPIClient() *api.Client {
	apiKey := config.GetAPIKey(flagAPIKey)
	apiURL := config.GetAPIURL(flagAPIURL)

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: No API key configured. Run 'gpulab auth login' or set GPULAB_API_KEY.")
		os.Exit(1)
	}

	return api.NewClient(apiURL, apiKey, flagDebug)
}

// requireAuth is like newAPIClient but used for commands that need auth
func requireAuth() *api.Client {
	return newAPIClient()
}
