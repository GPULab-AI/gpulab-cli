package commands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/GPULab-AI/gpulab-cli/internal/updater"
	"github.com/spf13/cobra"
)

var (
	updateCheckOnly    bool
	updateForce        bool
	updateSkipChecksum bool
	updateRepo         string
	updateTimeout      time.Duration
)

func init() {
	updateCmd.Flags().BoolVar(&updateCheckOnly, "check", false, "Only check whether an update is available")
	updateCmd.Flags().BoolVar(&updateForce, "force", false, "Install the latest release even if the current version cannot be compared")
	updateCmd.Flags().BoolVar(&updateSkipChecksum, "skip-checksum", false, "Skip release checksum verification")
	updateCmd.Flags().StringVar(&updateRepo, "repo", "", "GitHub repo to check (default \"GPULab-AI/gpulab-cli\")")
	updateCmd.Flags().DurationVar(&updateTimeout, "timeout", 2*time.Minute, "Update check/download timeout")

	rootCmd.AddCommand(updateCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the GPULab CLI",
	Long: `Check GitHub Releases for a newer GPULab CLI version and install it.

The updater downloads the release archive for your platform, verifies it against
the release checksums.txt file, and replaces the current executable.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if updateTimeout <= 0 {
			updateTimeout = 2 * time.Minute
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), updateTimeout)
		defer cancel()

		checkOptions := updater.CheckOptions{
			CurrentVersion: versionStr,
			Repo:           firstNonEmpty(updateRepo, updater.DefaultRepo),
			HTTPClient: &http.Client{
				Timeout: updateTimeout,
			},
			UserAgent: "gpulab-cli/" + versionStr,
		}

		if updateCheckOnly {
			info, err := updater.Check(ctx, checkOptions)
			if err != nil {
				return err
			}
			if flagJSON {
				output.PrintJSON(info)
			} else {
				printUpdateCheck(info)
			}
			return nil
		}

		result, err := updater.Install(ctx, updater.InstallOptions{
			CheckOptions: checkOptions,
			Force:        updateForce,
			SkipChecksum: updateSkipChecksum,
		})
		if err != nil {
			return err
		}

		if flagJSON {
			output.PrintJSON(result)
		} else {
			printUpdateInstallResult(result)
		}

		return nil
	},
}

func printUpdateCheck(info *updater.Info) {
	if !info.CanCompare {
		fmt.Printf("Latest GPULab CLI release: %s\n", formatVersion(info.LatestVersion))
		fmt.Printf("Current build: %s\n", info.CurrentVersion)
		fmt.Println("This build version cannot be compared automatically.")
		fmt.Println("Run `gpulab update --force` to install the latest release over this binary.")
		return
	}

	if info.UpdateAvailable {
		fmt.Printf("Update available: %s -> %s\n", formatVersion(info.CurrentVersion), formatVersion(info.LatestVersion))
		fmt.Println("Run `gpulab update` to install it.")
		return
	}

	fmt.Printf("GPULab CLI is up to date (%s).\n", formatVersion(info.CurrentVersion))
}

func printUpdateInstallResult(result *updater.InstallResult) {
	if !result.UpdateAvailable && !updateForce {
		fmt.Printf("GPULab CLI is up to date (%s).\n", formatVersion(result.CurrentVersion))
		return
	}

	output.PrintSuccess(fmt.Sprintf("Updated GPULab CLI to %s.", formatVersion(result.LatestVersion)))
	if result.ChecksumVerified {
		fmt.Println("Checksum verified.")
	}
	if result.InstalledPath != "" {
		fmt.Println("Installed:", result.InstalledPath)
	}
}

func formatVersion(version string) string {
	version = updater.NormalizeVersion(version)
	if version == "" {
		return "unknown"
	}
	return "v" + version
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
