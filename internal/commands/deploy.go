package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/gpulab/gpulab-cli/internal/api"
	"github.com/gpulab/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	deployName       string
	deployTemplate   string
	deployGPUType    string
	deployVolume     string
	deployPorts      string
	deployPort       []string
	deployEnv        []string
	deployEnvFile    string
	deployCommand    string
	deployMemory     int
	deployWait       bool
	deployPullPolicy string
)

func init() {
	deployCmd.Flags().StringVar(&deployName, "name", "", "Container name (required)")
	deployCmd.Flags().StringVar(&deployTemplate, "template", "", "Template UUID or name (required)")
	deployCmd.Flags().StringVar(&deployGPUType, "gpu-type", "", "GPU type (required)")
	deployCmd.Flags().StringVar(&deployVolume, "volume", "", "Network volume UUID")
	deployCmd.Flags().StringVar(&deployPorts, "ports", "", "Exposed ports (comma-separated, e.g. \"8080,3000\")")
	deployCmd.Flags().StringArrayVarP(&deployPort, "publish", "p", nil, "Expose a port (repeatable, e.g. -p 8080 -p 3000)")
	deployCmd.Flags().StringArrayVarP(&deployEnv, "env", "e", nil, "Environment variables (KEY=VALUE or KEY to inherit from host)")
	deployCmd.Flags().StringVar(&deployEnvFile, "env-file", "", "Read environment variables from a file")
	deployCmd.Flags().StringVar(&deployCommand, "command", "", "Startup command (alternative to trailing args)")
	deployCmd.Flags().IntVar(&deployMemory, "memory", 0, "Memory limit in GB (1-40)")
	deployCmd.Flags().BoolVar(&deployWait, "wait", false, "Wait for container to be running")
	deployCmd.Flags().StringVar(&deployPullPolicy, "pull-policy", "cached", "Image pull policy (cached|latest)")

	deployCmd.MarkFlagRequired("name")
	deployCmd.MarkFlagRequired("template")
	deployCmd.MarkFlagRequired("gpu-type")

	rootCmd.AddCommand(deployCmd)
}

var deployCmd = &cobra.Command{
	Use:   "deploy [flags] [-- COMMAND [ARGS...]]",
	Short: "Deploy a new GPU container",
	Long: `Deploy a new GPU container on GPULab.

Ports can be exposed in two ways:

  --ports "8080,3000"   Comma-separated list
  -p 8080 -p 3000      Docker-style, repeatable
  -p 8080:80            host:container syntax (host port is auto-assigned)

Environment variables can be passed in multiple ways:

  -e KEY=VALUE          Set a variable explicitly
  -e KEY                Inherit value from host environment
  --env-file .env       Load variables from a file (KEY=VALUE per line)

All three can be combined. Explicit -e values override --env-file values.

The startup command can be specified in two ways:

  --command "CMD"       As a flag value
  -- CMD [ARGS...]      Docker-style, everything after -- becomes the command

Examples:
  gpulab deploy --name train --template pytorch --gpu-type "RTX 4090" \
    -p 8080 -p 6006 \
    -e HF_TOKEN=hf_xxx -e WANDB_API_KEY --env-file .env \
    --wait -- python train.py --epochs 100 --lr 0.001

  gpulab deploy --name sglang --template pytorch --gpu-type "RTX 4090" \
    -e HF_TOKEN -e MODEL_NAME=meta-llama/Llama-3-8B \
    -- sglang.deploy --model $MODEL_NAME --tp 4`,
	Args:               cobra.ArbitraryArgs,
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()

		// Determine the container command.
		// Priority: trailing args after "--" > --command flag
		containerCommand := deployCommand

		if len(args) > 0 {
			containerCommand = strings.Join(args, " ")
		} else {
			trailingArgs := getTrailingArgs()
			if len(trailingArgs) > 0 {
				containerCommand = strings.Join(trailingArgs, " ")
			}
		}

		// Resolve template name to UUID if needed
		templateUUID := deployTemplate
		if !strings.Contains(templateUUID, "-") || len(templateUUID) < 20 {
			templates, err := client.ListTemplates()
			if err == nil {
				// Exact match first
				for _, t := range templates {
					if strings.EqualFold(t.Name, deployTemplate) {
						templateUUID = t.TemplateUUID
						break
					}
				}
				// Prefix/contains match if no exact match found
				if templateUUID == deployTemplate {
					for _, t := range templates {
						if strings.HasPrefix(strings.ToLower(t.Name), strings.ToLower(deployTemplate)) {
							templateUUID = t.TemplateUUID
							break
						}
					}
				}
			}
		}

		// Merge -p flags into --ports
		ports := mergePorts(deployPorts, deployPort)

		// Build env vars: env-file first, then -e flags override
		envVars, err := buildEnvVars(deployEnvFile, deployEnv)
		if err != nil {
			return err
		}

		req := &api.CreateContainerRequest{
			ServerName:      deployName,
			Type:            "GPU",
			GPUType:         deployGPUType,
			TemplateUUID:    templateUUID,
			OpenedPorts:     ports,
			VolumeMountPath: "",
			Command:         containerCommand,
			ImagePullPolicy: deployPullPolicy,
		}

		if deployVolume != "" {
			req.NetworkVolumeUUID = deployVolume
		}
		if deployMemory > 0 {
			req.Memory = &deployMemory
		}
		if envVars != nil {
			req.EnvironmentVariables = envVars
		}

		resp, err := client.CreateContainer(req)
		if err != nil {
			return err
		}

		if flagJSON && !deployWait {
			output.PrintJSON(resp)
			return nil
		}

		if !flagJSON {
			output.PrintSuccess(fmt.Sprintf("Container deploying: %s", resp.ContainerID))
			if containerCommand != "" {
				fmt.Fprintf(os.Stderr, "  Command: %s\n", containerCommand)
			}
			if envVars != nil {
				fmt.Fprintf(os.Stderr, "  Env vars: %d\n", len(envVars))
			}
		}

		if deployWait {
			s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
			s.Suffix = " Waiting for container to be ready..."
			if !flagJSON {
				s.Start()
			}

			for {
				time.Sleep(3 * time.Second)
				container, err := client.GetContainer(resp.ContainerID)
				if err != nil {
					continue
				}

				status := strings.ToLower(container.Status)
				if status == "running" || status == "healthy" {
					if !flagJSON {
						s.Stop()
						output.PrintSuccess(fmt.Sprintf("Container is running: %s", resp.ContainerID))
					}
					if flagJSON {
						output.PrintJSON(map[string]string{
							"status":       "running",
							"container_id": resp.ContainerID,
						})
					}
					return nil
				}

				if status == "failed" || status == "timeout" {
					if !flagJSON {
						s.Stop()
					}
					return fmt.Errorf("container deployment failed (status: %s)", container.Status)
				}
			}
		}

		return nil
	},
}

// mergePorts combines --ports "8080,3000" and -p 8080 -p 3000 into a single
// comma-separated string. Also handles Docker-style host:container syntax
// (e.g. -p 8080:80) by extracting the container port, since GPULab
// auto-assigns host ports.
func mergePorts(portsFlag string, portFlags []string) string {
	seen := make(map[string]bool)
	var ports []string

	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		// Handle host:container syntax — take the container port (right side)
		if idx := strings.LastIndex(raw, ":"); idx >= 0 {
			raw = raw[idx+1:]
		}
		// Strip /tcp, /udp suffixes
		if idx := strings.Index(raw, "/"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
		if raw != "" && !seen[raw] {
			seen[raw] = true
			ports = append(ports, raw)
		}
	}

	// From --ports flag
	for _, p := range strings.Split(portsFlag, ",") {
		add(p)
	}
	// From -p flags
	for _, p := range portFlags {
		add(p)
	}

	return strings.Join(ports, ",")
}

// buildEnvVars merges env-file and -e flag values into a single map.
// -e flags take precedence over env-file values.
// If a -e value is just "KEY" (no =), it inherits the value from the host environment.
func buildEnvVars(envFile string, envFlags []string) (map[string]string, error) {
	vars := make(map[string]string)
	hasAny := false

	// 1. Load from env file first (lower priority)
	if envFile != "" {
		fileVars, err := parseEnvFile(envFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read env file %s: %w", envFile, err)
		}
		for k, v := range fileVars {
			vars[k] = v
			hasAny = true
		}
	}

	// 2. Apply -e flags (higher priority, overrides file)
	for _, e := range envFlags {
		if idx := strings.IndexByte(e, '='); idx >= 0 {
			// KEY=VALUE
			key := e[:idx]
			value := e[idx+1:]
			if key != "" {
				vars[key] = value
				hasAny = true
			}
		} else {
			// Bare KEY — inherit from host environment
			if val, ok := os.LookupEnv(e); ok {
				vars[e] = val
				hasAny = true
			} else {
				return nil, fmt.Errorf("environment variable %q not set on host (use -e %s=VALUE to set explicitly)", e, e)
			}
		}
	}

	if !hasAny {
		return nil, nil
	}
	return vars, nil
}

// parseEnvFile reads a .env-style file. Supports:
//   - KEY=VALUE
//   - KEY="VALUE" or KEY='VALUE' (quotes stripped)
//   - # comments and blank lines (skipped)
//   - export KEY=VALUE (export prefix stripped)
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Strip optional "export " prefix
		line = strings.TrimPrefix(line, "export ")

		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue // skip lines without =
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		if key == "" {
			continue
		}

		// Strip surrounding quotes
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		vars[key] = value
	}

	return vars, scanner.Err()
}

// getTrailingArgs extracts arguments after "--" from os.Args
func getTrailingArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1:]
		}
	}
	return nil
}
