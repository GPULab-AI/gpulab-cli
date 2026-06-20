package commands

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	serverlessName             string
	serverlessTemplate         string
	serverlessGPUType          string
	serverlessRegion           string
	serverlessVolume           string
	serverlessGPUCount         int
	serverlessMemory           int
	serverlessPort             int
	serverlessHealthPath       string
	serverlessMinReplicas      int
	serverlessMaxReplicas      int
	serverlessConcurrency      int
	serverlessOverflow         bool
	serverlessAutoscaling      bool
	serverlessAutoscalingTpl   string
	serverlessPolicy           string
	serverlessPolicyFile       string
	serverlessMetricsJSON      string
	serverlessMetricsFile      string
	serverlessIdleTimeout      int
	serverlessColdStartTimeout int
	serverlessRequestTimeout   int
	serverlessPullPolicy       string
	serverlessCUDA             string
	serverlessEnv              []string
	serverlessEnvFile          string
	serverlessCommand          string
	serverlessNotes            string
	serverlessEnable           bool
	serverlessDisable          bool
	serverlessForceDelete      bool
	serverlessPage             int
	serverlessPerPage          int
	serverlessAllPages         bool
	serverlessDetails          bool
	serverlessReplica          string
	serverlessDeployLogs       bool
	serverlessFollowLogs       bool
	serverlessLogTail          int
	serverlessLogTimestamps    bool
	serverlessInvokeMethod     string
	serverlessInvokeData       string
	serverlessInvokeDataFile   string
	serverlessInvokeHeader     []string
	serverlessInvokeWait       bool
	serverlessInvokeTimeout    int
	serverlessWatchStatus      bool
	serverlessReplicaForce     bool
)

func init() {
	rootCmd.AddCommand(serverlessCmd)

	serverlessCmd.AddCommand(serverlessListCmd)
	serverlessCmd.AddCommand(serverlessCreateCmd)
	serverlessCmd.AddCommand(serverlessInspectCmd)
	serverlessCmd.AddCommand(serverlessUpdateCmd)
	serverlessCmd.AddCommand(serverlessDeleteCmd)
	serverlessCmd.AddCommand(serverlessOptionsCmd)
	serverlessCmd.AddCommand(serverlessReplicasCmd)
	serverlessCmd.AddCommand(serverlessRestartReplicaCmd)
	serverlessCmd.AddCommand(serverlessDeleteReplicaCmd)
	serverlessCmd.AddCommand(serverlessRequestsCmd)
	serverlessCmd.AddCommand(serverlessAutoscalingLogsCmd)
	serverlessCmd.AddCommand(serverlessLogsCmd)
	serverlessCmd.AddCommand(serverlessCancelRequestCmd)
	serverlessCmd.AddCommand(serverlessValidatePolicyCmd)
	serverlessCmd.AddCommand(serverlessStatusCmd)
	serverlessCmd.AddCommand(serverlessInvokeCmd)

	addServerlessWriteFlags(serverlessCreateCmd, true)
	addServerlessWriteFlags(serverlessUpdateCmd, false)

	serverlessDeleteCmd.Flags().BoolVar(&serverlessForceDelete, "force", false, "Skip confirmation")

	serverlessRequestsCmd.Flags().IntVar(&serverlessPage, "page", 1, "Page number")
	serverlessRequestsCmd.Flags().IntVar(&serverlessPerPage, "per-page", 25, "Rows per page")
	serverlessRequestsCmd.Flags().BoolVar(&serverlessAllPages, "all", false, "Fetch every page instead of just one")
	serverlessRequestsCmd.Flags().BoolVar(&serverlessDetails, "details", false, "Print request/response bodies in human output")

	serverlessAutoscalingLogsCmd.Flags().IntVar(&serverlessPage, "page", 1, "Page number")
	serverlessAutoscalingLogsCmd.Flags().IntVar(&serverlessPerPage, "per-page", 25, "Rows per page")
	serverlessAutoscalingLogsCmd.Flags().BoolVar(&serverlessAllPages, "all", false, "Fetch every page instead of just one")
	serverlessAutoscalingLogsCmd.Flags().BoolVar(&serverlessDetails, "details", false, "Print metrics and context in human output")

	serverlessDeleteReplicaCmd.Flags().BoolVar(&serverlessReplicaForce, "force", false, "Skip confirmation")

	serverlessLogsCmd.Flags().StringVar(&serverlessReplica, "replica", "", "Replica UUID or prefix (default: first running replica, use 'all' for every replica)")
	serverlessLogsCmd.Flags().BoolVar(&serverlessDeployLogs, "deploy", false, "Show deployment logs instead of runtime logs")
	serverlessLogsCmd.Flags().BoolVarP(&serverlessFollowLogs, "follow", "f", false, "Follow logs")
	serverlessLogsCmd.Flags().IntVarP(&serverlessLogTail, "tail", "n", 100, "Number of runtime log lines from the end")
	serverlessLogsCmd.Flags().BoolVarP(&serverlessLogTimestamps, "timestamps", "t", false, "Show runtime log timestamps")

	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessAutoscalingTpl, "template", "", "Autoscaling template key")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessPolicy, "policy", "", "Autoscaling policy code")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessPolicyFile, "policy-file", "", "Read autoscaling policy code from file")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessMetricsJSON, "metrics-json", "", "Autoscaling metrics JSON")
	serverlessValidatePolicyCmd.Flags().StringVar(&serverlessMetricsFile, "metrics-file", "", "Read autoscaling metrics JSON from file")

	serverlessStatusCmd.Flags().BoolVarP(&serverlessWatchStatus, "watch", "w", false, "Poll until the request is complete")

	serverlessInvokeCmd.Flags().StringVarP(&serverlessInvokeMethod, "method", "X", "POST", "HTTP method")
	serverlessInvokeCmd.Flags().StringVarP(&serverlessInvokeData, "data", "d", "", "Request body")
	serverlessInvokeCmd.Flags().StringVar(&serverlessInvokeDataFile, "data-file", "", "Read request body from file")
	serverlessInvokeCmd.Flags().StringArrayVarP(&serverlessInvokeHeader, "header", "H", nil, "Request header (repeatable, e.g. -H 'Content-Type: application/json')")
	serverlessInvokeCmd.Flags().BoolVar(&serverlessInvokeWait, "wait", false, "Poll queued async requests until complete")
	serverlessInvokeCmd.Flags().IntVar(&serverlessInvokeTimeout, "timeout", 600, "Wait timeout in seconds")
}

func addServerlessWriteFlags(cmd *cobra.Command, create bool) {
	cmd.Flags().StringVar(&serverlessName, "name", "", "Endpoint name")
	cmd.Flags().StringVar(&serverlessTemplate, "template", "", "Template ID, UUID, or name")
	cmd.Flags().StringVar(&serverlessGPUType, "gpu-type", "", "GPU type ID or name")
	cmd.Flags().StringVar(&serverlessRegion, "region", "", "Region ID, slug, or name")
	cmd.Flags().StringVar(&serverlessVolume, "volume", "", "Network volume ID, UUID, or name")
	cmd.Flags().IntVar(&serverlessGPUCount, "gpu-count", 1, "GPUs per replica")
	cmd.Flags().IntVar(&serverlessMemory, "memory", 32, "Memory per replica in GB")
	cmd.Flags().IntVar(&serverlessPort, "port", 8000, "Replica endpoint port")
	cmd.Flags().StringVar(&serverlessHealthPath, "health-path", "/health", "Health check path")
	cmd.Flags().IntVar(&serverlessMinReplicas, "min-replicas", 0, "Minimum warm replicas")
	cmd.Flags().IntVar(&serverlessMaxReplicas, "max-replicas", 1, "Maximum replicas")
	cmd.Flags().IntVar(&serverlessConcurrency, "concurrency", 1, "Max concurrent requests per replica")
	cmd.Flags().BoolVar(&serverlessOverflow, "overflow", false, "Allow overflow routing to another warm replica on 5xx")
	cmd.Flags().BoolVar(&serverlessAutoscaling, "autoscaling", false, "Enable custom autoscaling")
	cmd.Flags().StringVar(&serverlessAutoscalingTpl, "autoscaling-template", "", "Autoscaling template key")
	cmd.Flags().StringVar(&serverlessPolicy, "policy", "", "Autoscaling policy code")
	cmd.Flags().StringVar(&serverlessPolicyFile, "policy-file", "", "Read autoscaling policy code from file")
	cmd.Flags().StringVar(&serverlessMetricsJSON, "metrics-json", "", "Autoscaling metrics JSON")
	cmd.Flags().StringVar(&serverlessMetricsFile, "metrics-file", "", "Read autoscaling metrics JSON from file")
	cmd.Flags().IntVar(&serverlessIdleTimeout, "idle-timeout", 60, "Idle timeout in seconds")
	cmd.Flags().IntVar(&serverlessColdStartTimeout, "cold-start-timeout", 300, "Cold start timeout in seconds")
	cmd.Flags().IntVar(&serverlessRequestTimeout, "request-timeout", 600, "Request timeout in seconds")
	cmd.Flags().StringVar(&serverlessPullPolicy, "pull-policy", "cached", "Image pull policy (cached|latest)")
	cmd.Flags().StringVar(&serverlessCUDA, "cuda", "", "CUDA version label")
	cmd.Flags().StringArrayVarP(&serverlessEnv, "env", "e", nil, "Environment variable (KEY=VALUE or KEY to inherit from host)")
	cmd.Flags().StringVar(&serverlessEnvFile, "env-file", "", "Read environment variables from file")
	cmd.Flags().StringVar(&serverlessCommand, "command", "", "Replica startup command")
	cmd.Flags().StringVar(&serverlessNotes, "notes", "", "Endpoint notes")
	cmd.Flags().BoolVar(&serverlessEnable, "enable", false, "Enable endpoint")
	cmd.Flags().BoolVar(&serverlessDisable, "disable", false, "Disable endpoint")

	if create {
		cmd.MarkFlagRequired("name")
		cmd.MarkFlagRequired("template")
		cmd.MarkFlagRequired("gpu-type")
	}
}

var serverlessCmd = &cobra.Command{
	Use:     "serverless",
	Aliases: []string{"sl"},
	Short:   "Manage serverless GPU endpoints",
	RunE:    serverlessListCmd.RunE,
}

var serverlessListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List serverless GPU endpoints",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		services, err := client.ListServerlessServices()
		if err != nil {
			return err
		}
		printServerlessServices(services)
		return nil
	},
}

var serverlessCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a serverless GPU endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		req, err := buildServerlessRequest(cmd, client, nil)
		if err != nil {
			return err
		}
		resp, err := client.CreateServerlessService(req)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless endpoint created: %s", resp.Data.UUID))
		fmt.Printf("Name:         %s\n", resp.Data.Name)
		fmt.Printf("Endpoint key: %s\n", resp.Data.EndpointKey)
		fmt.Printf("URL:          %s\n", resp.Data.EndpointURL)
		return nil
	},
}

var serverlessInspectCmd = &cobra.Command{
	Use:   "inspect [ENDPOINT]",
	Short: "Show serverless endpoint details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(detail)
			return nil
		}
		printServerlessDetail(detail)
		return nil
	},
}

var serverlessUpdateCmd = &cobra.Command{
	Use:   "update [ENDPOINT]",
	Short: "Update a serverless GPU endpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		req, err := buildServerlessRequest(cmd, client, &detail.Data)
		if err != nil {
			return err
		}
		resp, err := client.UpdateServerlessService(identifier, req)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless endpoint updated: %s", resp.Data.UUID))
		fmt.Printf("Name: %s\n", resp.Data.Name)
		fmt.Printf("URL:  %s\n", resp.Data.EndpointURL)
		return nil
	},
}

var serverlessDeleteCmd = &cobra.Command{
	Use:     "delete [ENDPOINT]",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a serverless GPU endpoint and its replicas",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		if !serverlessForceDelete {
			fmt.Printf("Delete serverless endpoint %s and all warm replicas? [y/N] ", shortID(identifier))
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := client.DeleteServerlessService(identifier); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "uuid": identifier})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless endpoint deleted: %s", shortID(identifier)))
		return nil
	},
}

var serverlessOptionsCmd = &cobra.Command{
	Use:   "options",
	Short: "List serverless templates, GPU types, regions, volumes, and autoscaling templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		options, err := client.GetServerlessOptions()
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(options)
			return nil
		}
		printServerlessOptions(options)
		return nil
	},
}

var serverlessReplicasCmd = &cobra.Command{
	Use:   "replicas [ENDPOINT]",
	Short: "List replicas for a serverless endpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(detail.Replicas)
			return nil
		}
		printServerlessReplicas(detail.Replicas)
		return nil
	},
}

var serverlessRestartReplicaCmd = &cobra.Command{
	Use:   "restart-replica [ENDPOINT] [REPLICA]",
	Short: "Restart a single replica (tears it down and re-provisions)",
	Long: `Restart a single replica of a serverless endpoint.

The replica is torn down and a fresh one is brought back up to the warm-pool
minimum. For autoscaling endpoints the autoscaler reconciles on its next cycle.
REPLICA may be a full UUID, a UUID prefix, or a server name (see 'serverless
replicas <endpoint>').`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		replicaUUID, err := resolveServerlessReplicaUUID(client, identifier, args[1])
		if err != nil {
			return err
		}
		resp, err := client.RestartServerlessReplica(identifier, replicaUUID)
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Replica restarted: %s", shortID(replicaUUID)))
		if resp.ReplacementProvisioned {
			fmt.Println("A replacement replica is being provisioned.")
		}
		return nil
	},
}

var serverlessDeleteReplicaCmd = &cobra.Command{
	Use:     "delete-replica [ENDPOINT] [REPLICA]",
	Aliases: []string{"rm-replica"},
	Short:   "Delete a single replica of a serverless endpoint",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		replicaUUID, err := resolveServerlessReplicaUUID(client, identifier, args[1])
		if err != nil {
			return err
		}
		if !serverlessReplicaForce && !flagJSON && !flagQuiet {
			fmt.Printf("Delete replica %s of %s? [y/N] ", shortID(replicaUUID), shortID(identifier))
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := client.DeleteServerlessReplica(identifier, replicaUUID); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "action": "deleted", "uuid": replicaUUID})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Replica deleted: %s", shortID(replicaUUID)))
		return nil
	},
}

var serverlessRequestsCmd = &cobra.Command{
	Use:     "requests [ENDPOINT]",
	Aliases: []string{"request-logs"},
	Short:   "Show serverless request logs",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		var page *api.ServerlessPage[api.ServerlessRequestLog]
		if serverlessAllPages {
			page, err = fetchAllServerlessPages(func(p int) (*api.ServerlessPage[api.ServerlessRequestLog], error) {
				return client.ListServerlessRequests(identifier, p, serverlessPerPage)
			})
		} else {
			page, err = client.ListServerlessRequests(identifier, serverlessPage, serverlessPerPage)
		}
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(page)
			return nil
		}
		printServerlessRequests(page, serverlessDetails)
		return nil
	},
}

var serverlessAutoscalingLogsCmd = &cobra.Command{
	Use:     "autoscaling-logs [ENDPOINT]",
	Aliases: []string{"autoscaling-history", "history"},
	Short:   "Show serverless autoscaling logs and history",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		var page *api.ServerlessPage[api.ServerlessAutoscalingLog]
		if serverlessAllPages {
			page, err = fetchAllServerlessPages(func(p int) (*api.ServerlessPage[api.ServerlessAutoscalingLog], error) {
				return client.ListServerlessAutoscalingLogs(identifier, p, serverlessPerPage)
			})
		} else {
			page, err = client.ListServerlessAutoscalingLogs(identifier, serverlessPage, serverlessPerPage)
		}
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(page)
			return nil
		}
		printServerlessAutoscalingLogs(page, serverlessDetails)
		return nil
	},
}

var serverlessLogsCmd = &cobra.Command{
	Use:   "logs [ENDPOINT]",
	Short: "Show serverless replica container logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		identifier, err := client.ResolveServerlessService(args[0])
		if err != nil {
			return err
		}
		detail, err := client.GetServerlessService(identifier)
		if err != nil {
			return err
		}
		replicas, err := selectServerlessLogReplicas(detail.Replicas, serverlessReplica, serverlessDeployLogs)
		if err != nil {
			return err
		}
		if !serverlessFollowLogs {
			return printServerlessReplicaLogs(client, replicas, serverlessDeployLogs, serverlessLogTail, "", serverlessLogTimestamps)
		}
		since := fmt.Sprintf("%d", time.Now().Unix())
		for {
			if err := printServerlessReplicaLogs(client, replicas, serverlessDeployLogs, 0, since, serverlessLogTimestamps); err != nil && flagDebug {
				fmt.Fprintf(os.Stderr, "[DEBUG] log poll failed: %v\n", err)
			}
			since = fmt.Sprintf("%d", time.Now().Unix())
			time.Sleep(2 * time.Second)
		}
	},
}

var serverlessCancelRequestCmd = &cobra.Command{
	Use:   "cancel-request [REQUEST_UUID]",
	Short: "Cancel a queued or provisioning serverless request",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		if err := client.CancelServerlessRequest(args[0]); err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(map[string]string{"status": "success", "request_uuid": args[0]})
			return nil
		}
		output.PrintSuccess(fmt.Sprintf("Serverless request cancelled: %s", shortID(args[0])))
		return nil
	},
}

var serverlessValidatePolicyCmd = &cobra.Command{
	Use:   "validate-policy",
	Short: "Validate serverless autoscaling policy code",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		policy, metrics, err := resolveServerlessPolicyInputs(client, serverlessAutoscalingTpl)
		if err != nil {
			return err
		}
		resp, err := client.ValidateServerlessPolicy(&api.ServerlessPolicyValidationRequest{
			AutoscalingTemplateKey:   serverlessAutoscalingTpl,
			AutoscalingPolicyCode:    policy,
			AutoscalingMetricsConfig: metrics,
		})
		if err != nil {
			return err
		}
		if flagJSON {
			output.PrintJSON(resp)
			return nil
		}
		output.PrintSuccess(resp.Message)
		fmt.Printf("Policy hash:   %s\n", resp.PolicyHash)
		fmt.Printf("Metric count:  %d\n", resp.MetricsCount)
		return nil
	},
}

var serverlessStatusCmd = &cobra.Command{
	Use:   "status [REQUEST_UUID]",
	Short: "Show public serverless request status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		for {
			resp, err := client.GetServerlessRequestStatus(args[0])
			if err != nil {
				return err
			}
			if flagJSON {
				output.PrintJSON(resp)
			} else {
				printServerlessStatus(resp)
			}
			if !serverlessWatchStatus || !resp.Processing {
				return nil
			}
			time.Sleep(3 * time.Second)
		}
	},
}

var serverlessInvokeCmd = &cobra.Command{
	Use:   "invoke [ENDPOINT] [PATH]",
	Short: "Invoke a serverless endpoint",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		target, err := resolveInvokeURL(client, args)
		if err != nil {
			return err
		}
		resp, body, err := invokeServerlessURL(cmd.Context(), target)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if flagJSON {
			output.PrintJSON(map[string]interface{}{
				"status_code": resp.StatusCode,
				"headers":     resp.Header,
				"body":        string(body),
			})
		} else {
			fmt.Print(string(body))
			if len(body) > 0 && body[len(body)-1] != '\n' {
				fmt.Println()
			}
		}

		if serverlessInvokeWait && resp.StatusCode == http.StatusAccepted {
			requestID := parseRequestID(body)
			if requestID == "" {
				return nil
			}
			return waitServerlessRequest(client, requestID, time.Duration(serverlessInvokeTimeout)*time.Second)
		}
		return nil
	},
}

func buildServerlessRequest(cmd *cobra.Command, client *api.Client, existing *api.ServerlessService) (*api.ServerlessServiceRequest, error) {
	req := &api.ServerlessServiceRequest{
		IsEnabled:               true,
		GPUCount:                1,
		Memory:                  32,
		EndpointPort:            8000,
		HealthCheckPath:         "/health",
		MinReplicas:             0,
		MaxReplicas:             1,
		MaxConcurrentRequests:   1,
		IdleTimeoutSeconds:      60,
		ColdStartTimeoutSeconds: 300,
		RequestTimeoutSeconds:   600,
		ImagePullPolicy:         "cached",
	}

	if existing != nil {
		req = serverlessRequestFromService(*existing)
	}

	setStringFlag(cmd, "name", &req.Name, serverlessName)
	setStringFlag(cmd, "template", &req.Template, serverlessTemplate)
	setStringFlag(cmd, "gpu-type", &req.GPUType, serverlessGPUType)
	setStringFlag(cmd, "region", &req.Region, serverlessRegion)
	setStringFlag(cmd, "volume", &req.NetworkVolume, serverlessVolume)
	setIntFlag(cmd, "gpu-count", &req.GPUCount, serverlessGPUCount)
	setIntFlag(cmd, "memory", &req.Memory, serverlessMemory)
	setIntFlag(cmd, "port", &req.EndpointPort, serverlessPort)
	setStringFlag(cmd, "health-path", &req.HealthCheckPath, serverlessHealthPath)
	setIntFlag(cmd, "min-replicas", &req.MinReplicas, serverlessMinReplicas)
	setIntFlag(cmd, "max-replicas", &req.MaxReplicas, serverlessMaxReplicas)
	setIntFlag(cmd, "concurrency", &req.MaxConcurrentRequests, serverlessConcurrency)
	setBoolFlag(cmd, "overflow", &req.AllowOverflowRequests, serverlessOverflow)
	setBoolFlag(cmd, "autoscaling", &req.AutoscalingEnabled, serverlessAutoscaling)
	setStringFlag(cmd, "autoscaling-template", &req.AutoscalingTemplateKey, serverlessAutoscalingTpl)
	setIntFlag(cmd, "idle-timeout", &req.IdleTimeoutSeconds, serverlessIdleTimeout)
	setIntFlag(cmd, "cold-start-timeout", &req.ColdStartTimeoutSeconds, serverlessColdStartTimeout)
	setIntFlag(cmd, "request-timeout", &req.RequestTimeoutSeconds, serverlessRequestTimeout)
	setStringFlag(cmd, "pull-policy", &req.ImagePullPolicy, serverlessPullPolicy)
	setStringFlag(cmd, "cuda", &req.CUDAVersion, serverlessCUDA)
	setStringFlag(cmd, "command", &req.Command, serverlessCommand)
	setStringFlag(cmd, "notes", &req.Notes, serverlessNotes)

	if cmd.Flags().Changed("enable") {
		req.IsEnabled = true
	}
	if cmd.Flags().Changed("disable") {
		req.IsEnabled = false
	}
	if serverlessEnable && serverlessDisable {
		return nil, fmt.Errorf("--enable and --disable cannot be used together")
	}

	if req.Name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if req.Template == "" && req.TemplateID == 0 {
		return nil, fmt.Errorf("--template is required")
	}
	if req.GPUType == "" && req.GPUTypeID == 0 {
		return nil, fmt.Errorf("--gpu-type is required")
	}

	if req.Template != "" || req.GPUType != "" || req.Region != "" || req.NetworkVolume != "" {
		options, err := client.GetServerlessOptions()
		if err != nil {
			return nil, err
		}
		if req.Template != "" {
			template, err := resolveServerlessTemplateOption(options.Templates, req.Template)
			if err != nil {
				return nil, err
			}
			req.TemplateID = template.ID
			req.Template = ""
		}
		if req.GPUType != "" {
			gpuType, err := resolveServerlessGPUTypeOption(options.GPUTypes, req.GPUType)
			if err != nil {
				return nil, err
			}
			req.GPUTypeID = gpuType.ID
			req.GPUType = ""
		}
		if req.Region != "" {
			region, err := resolveServerlessRegionOption(options.Regions, req.Region)
			if err != nil {
				return nil, err
			}
			req.RegionID = region.ID
			req.Region = ""
		}
		if req.NetworkVolume != "" {
			volume, err := resolveServerlessVolumeOption(options.NetworkVolumes, req.NetworkVolume)
			if err != nil {
				return nil, err
			}
			id := volume.ID
			req.NetworkVolumeID = &id
			req.NetworkVolume = ""
		}
	}

	if cmd.Flags().Changed("env") || cmd.Flags().Changed("env-file") {
		envVars, err := buildEnvVars(serverlessEnvFile, serverlessEnv)
		if err != nil {
			return nil, err
		}
		req.EnvironmentVariables = envVars
	}

	if cmd.Flags().Changed("policy") || cmd.Flags().Changed("policy-file") || cmd.Flags().Changed("metrics-json") ||
		cmd.Flags().Changed("metrics-file") || cmd.Flags().Changed("autoscaling-template") {
		policy, metrics, err := resolveServerlessPolicyInputs(client, req.AutoscalingTemplateKey)
		if err != nil {
			return nil, err
		}
		req.AutoscalingPolicyCode = policy
		req.AutoscalingMetricsConfig = metrics
		if policy != "" {
			req.AutoscalingEnabled = true
		}
	}

	return req, nil
}

func resolveServerlessTemplateOption(options []api.ServerlessTemplateOption, value string) (*api.ServerlessTemplateOption, error) {
	var matches []api.ServerlessTemplateOption
	for _, option := range options {
		if strconv.Itoa(option.ID) == value || option.TemplateUUID == value || strings.EqualFold(option.Name, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.Name), strings.ToLower(value)) || strings.HasPrefix(option.TemplateUUID, value) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless template %q not found", value)
	}
	return nil, fmt.Errorf("serverless template %q is ambiguous", value)
}

func resolveServerlessGPUTypeOption(options []api.ServerlessGPUTypeOption, value string) (*api.ServerlessGPUTypeOption, error) {
	var matches []api.ServerlessGPUTypeOption
	for _, option := range options {
		if strconv.Itoa(option.ID) == value || strings.EqualFold(option.GPUName, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.GPUName), strings.ToLower(value)) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless GPU type %q not found", value)
	}
	return nil, fmt.Errorf("serverless GPU type %q is ambiguous", value)
}

func resolveServerlessRegionOption(options []api.ServerlessRegionOption, value string) (*api.ServerlessRegionOption, error) {
	var matches []api.ServerlessRegionOption
	for _, option := range options {
		if option.ID == value || option.Slug == value || strings.EqualFold(option.Name, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.Name), strings.ToLower(value)) || strings.HasPrefix(option.ID, value) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless region %q not found", value)
	}
	return nil, fmt.Errorf("serverless region %q is ambiguous", value)
}

func resolveServerlessVolumeOption(options []api.ServerlessNetworkVolumeOption, value string) (*api.ServerlessNetworkVolumeOption, error) {
	var matches []api.ServerlessNetworkVolumeOption
	for _, option := range options {
		if strconv.Itoa(option.ID) == value || option.VolumeUUID == value || strings.EqualFold(option.VolumeName, value) {
			return &option, nil
		}
		if strings.Contains(strings.ToLower(option.VolumeName), strings.ToLower(value)) || strings.HasPrefix(option.VolumeUUID, value) {
			matches = append(matches, option)
		}
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("serverless network volume %q not found", value)
	}
	return nil, fmt.Errorf("serverless network volume %q is ambiguous", value)
}

func serverlessRequestFromService(service api.ServerlessService) *api.ServerlessServiceRequest {
	req := &api.ServerlessServiceRequest{
		Name:                     service.Name,
		IsEnabled:                service.IsEnabled,
		TemplateID:               service.TemplateID,
		GPUTypeID:                service.GPUTypeID,
		RegionID:                 service.RegionID,
		GPUCount:                 service.GPUCount,
		Memory:                   service.Memory,
		EndpointPort:             service.EndpointPort,
		HealthCheckPath:          service.HealthCheckPath,
		MinReplicas:              service.MinReplicas,
		MaxReplicas:              service.MaxReplicas,
		MaxConcurrentRequests:    service.MaxConcurrentRequests,
		AllowOverflowRequests:    service.AllowOverflowRequests,
		AutoscalingEnabled:       service.AutoscalingEnabled,
		AutoscalingTemplateKey:   service.AutoscalingTemplateKey,
		AutoscalingPolicyCode:    service.AutoscalingPolicyCode,
		AutoscalingMetricsConfig: service.AutoscalingMetricsConfig,
		IdleTimeoutSeconds:       service.IdleTimeoutSeconds,
		ColdStartTimeoutSeconds:  service.ColdStartTimeoutSeconds,
		RequestTimeoutSeconds:    service.RequestTimeoutSeconds,
		ImagePullPolicy:          service.ImagePullPolicy,
		CUDAVersion:              service.CUDAVersion,
		EnvironmentVariables:     service.EnvironmentVariables,
		Command:                  service.Command,
		Notes:                    service.Notes,
	}
	if service.NetworkVolumeID != nil {
		v := *service.NetworkVolumeID
		req.NetworkVolumeID = &v
	}
	return req
}

func resolveServerlessPolicyInputs(client *api.Client, templateKey string) (string, interface{}, error) {
	policy := serverlessPolicy
	if serverlessPolicyFile != "" {
		data, err := os.ReadFile(serverlessPolicyFile)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read policy file: %w", err)
		}
		policy = string(data)
	}

	var metrics interface{} = serverlessMetricsJSON
	if serverlessMetricsFile != "" {
		data, err := os.ReadFile(serverlessMetricsFile)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read metrics file: %w", err)
		}
		metrics = string(data)
	}

	if templateKey != "" && (policy == "" || metrics == "") {
		options, err := client.GetServerlessOptions()
		if err != nil {
			return "", nil, err
		}
		for _, template := range options.AutoscalingTemplates {
			if template.Key == templateKey {
				if policy == "" {
					policy = template.PolicyCode
				}
				if metrics == "" {
					metrics = template.MetricsConfig
				}
				break
			}
		}
	}

	if policy == "" {
		return "", nil, fmt.Errorf("autoscaling policy is required (use --policy, --policy-file, or --autoscaling-template)")
	}
	return policy, metrics, nil
}

// fetchAllServerlessPages walks every page of a paginated serverless endpoint
// and returns a single page containing all rows, so `--all` shows everything.
func fetchAllServerlessPages[T any](fetch func(page int) (*api.ServerlessPage[T], error)) (*api.ServerlessPage[T], error) {
	first, err := fetch(1)
	if err != nil {
		return nil, err
	}
	for p := 2; p <= first.LastPage; p++ {
		next, err := fetch(p)
		if err != nil {
			return nil, err
		}
		first.Data = append(first.Data, next.Data...)
	}
	first.CurrentPage = 1
	first.LastPage = 1
	first.HasNextPage = false
	first.HasPrevPage = false
	return first, nil
}

func printServerlessServices(services []api.ServerlessService) {
	switch getOutputFormat() {
	case output.FormatJSON:
		output.PrintJSON(services)
	case output.FormatQuiet:
		values := make([]string, len(services))
		for i, service := range services {
			values[i] = service.UUID
		}
		output.PrintQuiet(values)
	default:
		if len(services) == 0 {
			fmt.Println("No serverless endpoints found.")
			return
		}
		headers := []string{"UUID", "NAME", "STATUS", "GPU", "REPLICAS", "REQUESTS", "URL"}
		rows := make([][]string, len(services))
		for i, service := range services {
			status := "paused"
			if service.IsEnabled {
				status = "active"
			}
			rows[i] = []string{
				shortID(service.UUID),
				service.Name,
				output.StatusColor(status),
				service.GPUTypeName,
				fmt.Sprintf("%d/%d", service.ActiveReplicasCount, service.MaxReplicas),
				fmt.Sprintf("%d", service.TotalRequestsCount),
				service.EndpointURL,
			}
		}
		output.PrintTable(headers, rows)
	}
}

func printServerlessDetail(detail *api.ServerlessDetail) {
	service := detail.Data
	fmt.Printf("UUID:          %s\n", service.UUID)
	fmt.Printf("Name:          %s\n", service.Name)
	fmt.Printf("Status:        %s\n", boolStatus(service.IsEnabled, "active", "paused"))
	fmt.Printf("Endpoint key:  %s\n", service.EndpointKey)
	fmt.Printf("URL:           %s\n", service.EndpointURL)
	fmt.Printf("Template:      %s\n", service.TemplateName)
	if service.RegionName != "" {
		fmt.Printf("Region:        %s\n", service.RegionName)
	}

	fmt.Printf("\nResources:\n")
	fmt.Printf("  GPU:         %s x%d\n", service.GPUTypeName, service.GPUCount)
	fmt.Printf("  Memory:      %d GB\n", service.Memory)
	fmt.Printf("  Port:        %d\n", service.EndpointPort)
	fmt.Printf("  Health:      %s\n", service.HealthCheckPath)
	fmt.Printf("  Pull policy: %s\n", service.ImagePullPolicy)
	if service.CUDAVersion != "" {
		fmt.Printf("  CUDA:        %s\n", service.CUDAVersion)
	}

	fmt.Printf("\nNetwork Volume:\n")
	if service.NetworkVolumeName != "" || service.NetworkVolumeUUID != "" {
		fmt.Printf("  Name:        %s\n", service.NetworkVolumeName)
		if service.NetworkVolumeUUID != "" {
			fmt.Printf("  UUID:        %s\n", service.NetworkVolumeUUID)
		}
		if service.NetworkVolumeMaxSize != nil {
			fmt.Printf("  Size:        %d GB\n", *service.NetworkVolumeMaxSize)
		}
		if service.TemplateVolumeMountPath != "" {
			fmt.Printf("  Mount path:  %s\n", service.TemplateVolumeMountPath)
		}
	} else {
		fmt.Printf("  (none)\n")
	}

	fmt.Printf("\nScaling:\n")
	fmt.Printf("  Replicas:    %d active, %d provisioning (min %d, max %d)\n", service.ActiveReplicasCount, service.ProvisioningReplicasCount, service.MinReplicas, service.MaxReplicas)
	fmt.Printf("  Concurrency: %d per replica\n", service.MaxConcurrentRequests)
	fmt.Printf("  Overflow:    %s\n", boolStatus(service.AllowOverflowRequests, "enabled", "disabled"))
	fmt.Printf("  Autoscaling: %s\n", boolStatus(service.AutoscalingEnabled, "enabled", "disabled"))
	if service.AutoscalingTemplateKey != "" {
		fmt.Printf("  Template:    %s\n", service.AutoscalingTemplateKey)
	}
	if service.AutoscalingPolicyCode != "" {
		fmt.Printf("  Policy:      %s\n", summarizeText(service.AutoscalingPolicyCode))
	}
	if service.AutoscalingMetricsConfig != "" {
		fmt.Printf("  Metrics:     %s\n", summarizeText(service.AutoscalingMetricsConfig))
	}
	if service.AutoscalingLastEvaluatedAt != "" {
		fmt.Printf("  Last eval:   %s\n", shortTime(service.AutoscalingLastEvaluatedAt))
	}
	if service.AutoscalingLastErrorAt != "" {
		fmt.Printf("  Last error:  %s\n", shortTime(service.AutoscalingLastErrorAt))
	}

	fmt.Printf("\nTimeouts:\n")
	fmt.Printf("  Idle:        %ds\n", service.IdleTimeoutSeconds)
	fmt.Printf("  Cold start:  %ds\n", service.ColdStartTimeoutSeconds)
	fmt.Printf("  Request:     %ds\n", service.RequestTimeoutSeconds)

	fmt.Printf("\nTraffic:\n")
	fmt.Printf("  Requests:    %d total, %d queued\n", service.TotalRequestsCount, service.QueuedRequestsCount)
	if service.LastInvokedAt != "" {
		fmt.Printf("  Last invoke: %s\n", shortTime(service.LastInvokedAt))
	}
	if service.LastScaledAt != "" {
		fmt.Printf("  Last scaled: %s\n", shortTime(service.LastScaledAt))
	}

	if len(service.EnvironmentVariablesMap) > 0 {
		fmt.Printf("\nEnvironment:   %d variable(s)\n", len(service.EnvironmentVariablesMap))
	}
	if service.Command != "" {
		fmt.Printf("Command:       %s\n", service.Command)
	}
	if service.Notes != "" {
		fmt.Printf("Notes:         %s\n", service.Notes)
	}

	fmt.Printf("\nReplicas:\n")
	printServerlessReplicas(detail.Replicas)

	if len(detail.AutoscalingLogs.Data) > 0 {
		fmt.Printf("\nRecent autoscaling logs:\n")
		printServerlessAutoscalingLogs(&detail.AutoscalingLogs, false)
	}
}

// summarizeText collapses a possibly-multiline config value to a single line for
// summary display; the full value is available via --json.
func summarizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(none)"
	}
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return fmt.Sprintf("%s … (%d lines)", strings.TrimSpace(s[:idx]), strings.Count(s, "\n")+1)
	}
	if len(s) > 70 {
		return s[:67] + "..."
	}
	return s
}

// resolveServerlessReplicaUUID maps a full UUID, UUID prefix, or server name to
// a replica UUID within the given endpoint.
func resolveServerlessReplicaUUID(client *api.Client, identifier, partial string) (string, error) {
	detail, err := client.GetServerlessService(identifier)
	if err != nil {
		return "", err
	}
	var matches []api.ServerlessReplica
	for _, replica := range detail.Replicas {
		if replica.UUID == partial {
			return replica.UUID, nil
		}
		if strings.HasPrefix(replica.UUID, partial) || strings.HasPrefix(replica.ServerName, partial) {
			matches = append(matches, replica)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no replica found matching %q (run 'gpulab serverless replicas %s' to list)", partial, partial)
	case 1:
		return matches[0].UUID, nil
	default:
		msg := fmt.Sprintf("ambiguous replica %q matches %d replicas:\n", partial, len(matches))
		for _, m := range matches {
			msg += fmt.Sprintf("  %s  %s  %s\n", m.UUID, m.ServerName, m.Status)
		}
		return "", fmt.Errorf("%s", msg)
	}
}

func printServerlessReplicas(replicas []api.ServerlessReplica) {
	if len(replicas) == 0 {
		fmt.Println("No replicas found.")
		return
	}
	headers := []string{"UUID", "NAME", "STATUS", "SERVER", "UPTIME", "CREATED"}
	rows := make([][]string, len(replicas))
	for i, replica := range replicas {
		server := ""
		if replica.Server != nil {
			server = replica.Server["name"]
		}
		rows[i] = []string{
			shortID(replica.UUID),
			replica.ServerName,
			output.StatusColor(replica.Status),
			server,
			replica.Uptime,
			shortTime(replica.CreatedAt),
		}
	}
	output.PrintTable(headers, rows)
}

func printServerlessRequests(page *api.ServerlessPage[api.ServerlessRequestLog], details bool) {
	if len(page.Data) == 0 {
		fmt.Println("No serverless request logs found.")
		return
	}
	headers := []string{"UUID", "STATUS", "METHOD", "PATH", "CODE", "DURATION", "QUEUED"}
	rows := make([][]string, len(page.Data))
	for i, request := range page.Data {
		code := "-"
		if request.ResponseStatusCode != nil {
			code = strconv.Itoa(*request.ResponseStatusCode)
		}
		duration := "-"
		if request.DurationMS != nil {
			duration = fmt.Sprintf("%dms", *request.DurationMS)
		}
		rows[i] = []string{
			shortID(request.UUID),
			output.StatusColor(request.Status),
			request.Method,
			requestTarget(request.Path, request.QueryString),
			code,
			duration,
			shortTime(request.QueuedAt),
		}
	}
	output.PrintTable(headers, rows)
	fmt.Printf("Page %d of %d, %d total\n", page.CurrentPage, page.LastPage, page.Total)

	if details {
		for _, request := range page.Data {
			fmt.Printf("\n%s %s %s\n", request.UUID, request.Method, requestTarget(request.Path, request.QueryString))
			printBodyPayload("Request", request.RequestBody)
			printBodyPayload("Response", request.ResponseBody)
			if request.ErrorMessage != "" {
				fmt.Printf("Error: %s\n", request.ErrorMessage)
			}
		}
	}
}

func printServerlessAutoscalingLogs(page *api.ServerlessPage[api.ServerlessAutoscalingLog], details bool) {
	if len(page.Data) == 0 {
		fmt.Println("No autoscaling logs found.")
		return
	}
	headers := []string{"TIME", "LEVEL", "EVENT", "CURRENT", "DESIRED", "MESSAGE"}
	rows := make([][]string, len(page.Data))
	for i, log := range page.Data {
		current := "-"
		desired := "-"
		if log.CurrentReplicas != nil {
			current = strconv.Itoa(*log.CurrentReplicas)
		}
		if log.DesiredReplicas != nil {
			desired = strconv.Itoa(*log.DesiredReplicas)
		}
		rows[i] = []string{shortTime(log.CreatedAt), log.Level, log.Event, current, desired, log.Message}
	}
	output.PrintTable(headers, rows)
	fmt.Printf("Page %d of %d, %d total\n", page.CurrentPage, page.LastPage, page.Total)

	if details {
		for _, log := range page.Data {
			fmt.Printf("\n[%s] %s %s\n", shortTime(log.CreatedAt), log.Level, log.Event)
			printIndentedJSON("Metrics", log.Metrics)
			printIndentedJSON("Context", log.Context)
		}
	}
}

func printServerlessOptions(options *api.ServerlessOptions) {
	fmt.Println("Templates")
	templateRows := make([][]string, len(options.Templates))
	for i, template := range options.Templates {
		memory := "-"
		if template.MemoryLimit != nil {
			memory = fmt.Sprintf("%d GB", *template.MemoryLimit)
		}
		templateRows[i] = []string{strconv.Itoa(template.ID), template.Name, template.DockerImage, memory}
	}
	output.PrintTable([]string{"ID", "NAME", "IMAGE", "MEMORY"}, templateRows)

	fmt.Println("\nGPU Types")
	gpuRows := make([][]string, len(options.GPUTypes))
	for i, gpu := range options.GPUTypes {
		price := "-"
		if value, err := gpu.GPUPrice.Float64(); err == nil && value > 0 {
			price = fmt.Sprintf("$%.2f/hr", value)
		}
		gpuRows[i] = []string{strconv.Itoa(gpu.ID), gpu.GPUName, fmt.Sprintf("%d MB", gpu.GPUTotalMemory), fmt.Sprintf("%d", gpu.FreeGPUsCount), price}
	}
	output.PrintTable([]string{"ID", "GPU", "MEMORY", "FREE", "PRICE"}, gpuRows)

	fmt.Println("\nRegions")
	regionRows := make([][]string, len(options.Regions))
	for i, region := range options.Regions {
		regionRows[i] = []string{region.ID, region.Name, region.Slug}
	}
	output.PrintTable([]string{"ID", "NAME", "SLUG"}, regionRows)

	fmt.Println("\nNetwork Volumes")
	volumeRows := make([][]string, len(options.NetworkVolumes))
	for i, volume := range options.NetworkVolumes {
		size := "-"
		if volume.MaxSize != nil {
			size = fmt.Sprintf("%d GB", *volume.MaxSize)
		}
		volumeRows[i] = []string{strconv.Itoa(volume.ID), volume.VolumeName, volume.RegionName, size}
	}
	output.PrintTable([]string{"ID", "NAME", "REGION", "SIZE"}, volumeRows)

	fmt.Println("\nAutoscaling Templates")
	autoRows := make([][]string, len(options.AutoscalingTemplates))
	for i, template := range options.AutoscalingTemplates {
		autoRows[i] = []string{template.Key, template.Name, template.Summary}
	}
	output.PrintTable([]string{"KEY", "NAME", "SUMMARY"}, autoRows)
}

func selectServerlessLogReplicas(replicas []api.ServerlessReplica, selector string, deployLogs bool) ([]api.ServerlessReplica, error) {
	if len(replicas) == 0 {
		return nil, fmt.Errorf("no replicas found for this serverless endpoint")
	}
	if selector == "all" {
		return replicas, nil
	}
	if selector != "" {
		var matches []api.ServerlessReplica
		for _, replica := range replicas {
			if strings.HasPrefix(replica.UUID, selector) || strings.HasPrefix(replica.ServerName, selector) {
				matches = append(matches, replica)
			}
		}
		if len(matches) == 1 {
			return matches, nil
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no replica found with prefix %q", selector)
		}
		return nil, fmt.Errorf("ambiguous replica prefix %q", selector)
	}

	if deployLogs {
		for _, replica := range replicas {
			if replica.HasLogs {
				return []api.ServerlessReplica{replica}, nil
			}
		}
	}
	for _, replica := range replicas {
		status := strings.ToLower(replica.Status)
		if status == "running" || status == "healthy" || status == "unhealthy" {
			return []api.ServerlessReplica{replica}, nil
		}
	}
	return []api.ServerlessReplica{replicas[0]}, nil
}

func printServerlessReplicaLogs(client *api.Client, replicas []api.ServerlessReplica, deploy bool, tail int, since string, timestamps bool) error {
	results := make(map[string]string)
	for _, replica := range replicas {
		var logs string
		var err error
		if deploy {
			logs, err = client.GetDeploymentLogs(replica.UUID)
		} else {
			logs, err = client.GetContainerLogs(replica.UUID, tail, since, timestamps)
		}
		if err != nil {
			return err
		}
		results[replica.UUID] = logs
	}
	if flagJSON && since == "" {
		output.PrintJSON(results)
		return nil
	}
	for _, replica := range replicas {
		logs := results[replica.UUID]
		if len(replicas) > 1 && logs != "" {
			fmt.Printf("==> %s (%s) <==\n", replica.ServerName, shortID(replica.UUID))
		}
		fmt.Print(logs)
		if logs != "" && logs[len(logs)-1] != '\n' {
			fmt.Println()
		}
	}
	return nil
}

func resolveInvokeURL(client *api.Client, args []string) (string, error) {
	target := args[0]
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		if len(args) == 2 {
			return strings.TrimRight(target, "/") + "/" + strings.TrimLeft(args[1], "/"), nil
		}
		return target, nil
	}

	resolved, err := client.ResolveServerlessService(target)
	if err == nil {
		detail, err := client.GetServerlessService(resolved)
		if err != nil {
			return "", err
		}
		target = detail.Data.EndpointURL
	} else {
		target = strings.TrimRight(client.BaseURL, "/") + "/serverless/" + strings.TrimLeft(target, "/")
	}
	if len(args) == 2 {
		target = strings.TrimRight(target, "/") + "/" + strings.TrimLeft(args[1], "/")
	}
	return target, nil
}

func invokeServerlessURL(ctx context.Context, target string) (*http.Response, []byte, error) {
	var body io.Reader
	if serverlessInvokeDataFile != "" {
		data, err := os.ReadFile(serverlessInvokeDataFile)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(data)
	} else if serverlessInvokeData != "" {
		body = strings.NewReader(serverlessInvokeData)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(serverlessInvokeMethod), target, body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	for _, header := range serverlessInvokeHeader {
		name, value, ok := strings.Cut(header, ":")
		if !ok {
			return nil, nil, fmt.Errorf("invalid header %q, expected Name: Value", header)
		}
		req.Header.Set(strings.TrimSpace(name), strings.TrimSpace(value))
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: time.Duration(serverlessInvokeTimeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	return resp, data, nil
}

func waitServerlessRequest(client *api.Client, requestID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		status, err := client.GetServerlessRequestStatus(requestID)
		if err != nil {
			return err
		}
		if !status.Processing {
			if flagJSON {
				output.PrintJSON(status)
			} else {
				printServerlessStatus(status)
			}
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for serverless request %s", requestID)
		}
		time.Sleep(3 * time.Second)
	}
}

func printServerlessStatus(status *api.ServerlessStatusResponse) {
	fmt.Printf("Request:    %s\n", status.RequestID)
	fmt.Printf("Status:     %s\n", output.StatusColor(status.Status))
	fmt.Printf("Processing: %t\n", status.Processing)
	fmt.Printf("Message:    %s\n", status.Message)
	if status.Response != nil {
		if code, ok := status.Response["status_code"]; ok {
			fmt.Printf("HTTP Code:   %v\n", code)
		}
		if body, ok := status.Response["body"].(string); ok && body != "" {
			fmt.Printf("\n%s\n", body)
		}
	}
}

func parseRequestID(body []byte) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if value, ok := payload["request_id"].(string); ok {
		return value
	}
	return ""
}

func printBodyPayload(label string, body *api.ServerlessBodyPayload) {
	if body == nil {
		fmt.Printf("%s: <empty>\n", label)
		return
	}
	switch body.Encoding {
	case "utf8":
		fmt.Printf("%s:\n%s\n", label, body.Data)
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(body.Data)
		if err == nil {
			fmt.Printf("%s: <binary %d bytes>\n", label, len(decoded))
		} else {
			fmt.Printf("%s: <base64 %d bytes>\n", label, body.SizeBytes)
		}
	default:
		fmt.Printf("%s: <%s %d bytes>\n", label, body.Encoding, body.SizeBytes)
	}
}

func printIndentedJSON(label string, value interface{}) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil || string(data) == "{}" || string(data) == "null" {
		return
	}
	fmt.Printf("%s:\n%s\n", label, string(data))
}

func requestTarget(path, query string) string {
	if query == "" {
		return path
	}
	return path + "?" + query
}

func setStringFlag(cmd *cobra.Command, name string, target *string, value string) {
	if cmd.Flags().Changed(name) {
		*target = value
	}
}

func setIntFlag(cmd *cobra.Command, name string, target *int, value int) {
	if cmd.Flags().Changed(name) {
		*target = value
	}
}

func setBoolFlag(cmd *cobra.Command, name string, target *bool, value bool) {
	if cmd.Flags().Changed(name) {
		*target = value
	}
}

func boolStatus(enabled bool, trueValue, falseValue string) string {
	if enabled {
		return trueValue
	}
	return falseValue
}

func shortID(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func shortTime(value string) string {
	if len(value) > 16 {
		return value[:16]
	}
	return value
}

func addQueryParam(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
