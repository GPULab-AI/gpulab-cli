package commands

import (
	"fmt"
	"os"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/config"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
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
	Short: "GPULab CLI - Manage GPU containers, serverless endpoints, and volumes",
	Long: `GPULab CLI provides command-line access to the GPULab platform: deploy and
manage GPU containers, run serverless GPU endpoints, and work with network
volumes (including their files).

Authentication (in priority order):
  1. --api-key flag
  2. GPULAB_API_KEY environment variable
  3. ~/.gpulab/config.json (run 'gpulab auth login')

The API base URL defaults to https://gpulab.ai/api and can be overridden with
--api-url or GPULAB_API_URL.

Output modes (use these for scripting and AI agents):
  --json     emit structured JSON instead of tables
  --quiet/-q print only IDs, one per line
  --debug    log every HTTP request/response to stderr

Common commands:
  gpulab ps                          list containers
  gpulab logs <container> -f         stream container logs
  gpulab volumes                     list network volumes
  gpulab volumes files ls <volume>   browse files on a volume
  gpulab serverless list             list serverless GPU endpoints`,
	Example: `  gpulab ps --json
  gpulab volumes files upload my-vol ./model.bin --dest models
  gpulab serverless logs my-endpoint --follow`,
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
