package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var traceEndpoint string

func init() {
	serverlessCmd.AddCommand(serverlessTraceCmd)
	serverlessTraceCmd.Flags().StringVar(&traceEndpoint, "endpoint", "", "Endpoint to search for the request log (default: inferred from the request)")
}

var serverlessTraceCmd = &cobra.Command{
	Use:     "trace [REQUEST_UUID]",
	Aliases: []string{"track"},
	Short:   "Trace a serverless request's lifecycle (queued → started → completed → result)",
	Long: `Follow a single serverless request end to end: when it was queued, when a
replica picked it up, when it completed, how long each stage took, and the final
HTTP result (200/4xx/5xx) or error. This reconstructs what you'd otherwise dig
out of request logs by hand.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := requireAuth()
		reqID := args[0]

		status, statusErr := client.GetServerlessRequestStatus(reqID)

		endpointName := ""
		endpointID := traceEndpoint
		if status != nil {
			endpointName = status.Service["name"]
			if endpointID == "" {
				endpointID = endpointKeyFromURL(status.Service["endpoint_url"])
			}
		}

		var logEntry *api.ServerlessRequestLog
		if endpointID != "" {
			if identifier, err := client.ResolveServerlessService(endpointID); err == nil {
				logEntry = findRequestLog(client, identifier, reqID)
			}
		}

		if status == nil && logEntry == nil {
			return fmt.Errorf("request %q not found (try --endpoint to scope the search): %v", reqID, statusErr)
		}

		if flagJSON {
			output.PrintJSON(map[string]interface{}{
				"request_id": reqID,
				"endpoint":   endpointName,
				"status":     status,
				"log":        logEntry,
			})
			return nil
		}
		printServerlessTrace(reqID, endpointName, status, logEntry)
		return nil
	},
}

// endpointKeyFromURL extracts the endpoint key (last path segment) from an
// endpoint URL like https://gpulab.ai/api/serverless/m19nv5t94z.
func endpointKeyFromURL(rawURL string) string {
	rawURL = strings.TrimRight(rawURL, "/")
	if idx := strings.LastIndex(rawURL, "/"); idx >= 0 {
		return rawURL[idx+1:]
	}
	return ""
}

// findRequestLog scans an endpoint's recent request logs (bounded) for a UUID.
func findRequestLog(client *api.Client, identifier, reqID string) *api.ServerlessRequestLog {
	for page := 1; page <= 5; page++ {
		resp, err := client.ListServerlessRequests(identifier, page, 100)
		if err != nil {
			return nil
		}
		for i := range resp.Data {
			if resp.Data[i].UUID == reqID {
				return &resp.Data[i]
			}
		}
		if page >= resp.LastPage {
			break
		}
	}
	return nil
}

func printServerlessTrace(reqID, endpointName string, status *api.ServerlessStatusResponse, log *api.ServerlessRequestLog) {
	fmt.Printf("Request:  %s\n", reqID)
	if endpointName != "" {
		fmt.Printf("Endpoint: %s\n", endpointName)
	}

	state, code, method, path, errMsg := "", "", "", "", ""
	var queuedAt, startedAt, completedAt string
	var durationMS *int64

	if log != nil {
		method = log.Method
		path = requestTarget(log.Path, log.QueryString)
		state = log.Status
		errMsg = log.ErrorMessage
		queuedAt, startedAt, completedAt = log.QueuedAt, log.StartedAt, log.CompletedAt
		durationMS = log.DurationMS
		if log.ResponseStatusCode != nil {
			code = strconv.Itoa(*log.ResponseStatusCode)
		}
	}
	if status != nil {
		if state == "" {
			state = status.Status
		}
		queuedAt = firstNonEmpty(queuedAt, status.QueuedAt)
		startedAt = firstNonEmpty(startedAt, status.StartedAt)
		completedAt = firstNonEmpty(completedAt, status.CompletedAt)
		if code == "" && status.Response != nil {
			if c, ok := status.Response["status_code"]; ok && c != nil {
				code = fmt.Sprintf("%v", c)
			}
		}
	}

	if method != "" || path != "" {
		fmt.Printf("Call:     %s %s\n", method, path)
	}
	if state != "" {
		fmt.Printf("State:    %s\n", output.StatusColor(state))
	}
	fmt.Printf("Result:   %s\n", resultLabel(code))

	fmt.Printf("\nTimeline:\n")
	qt := printTraceStage("queued", queuedAt, "")
	st := printTraceStage("started", startedAt, deltaLabel(qt, startedAt, "wait"))
	printTraceStage("completed", completedAt, deltaLabel(st, completedAt, "run"))

	if durationMS != nil {
		fmt.Printf("\nDuration: %dms\n", *durationMS)
	}
	if strings.TrimSpace(errMsg) != "" {
		fmt.Printf("Error:    %s\n", errMsg)
	}
}

// printTraceStage prints "  <label>  <time>  (<extra>)" and returns the parsed
// time string it used (for delta computation), or "" if absent.
func printTraceStage(label, ts, extra string) string {
	if strings.TrimSpace(ts) == "" {
		fmt.Printf("  %-10s —\n", label)
		return ""
	}
	if extra != "" {
		fmt.Printf("  %-10s %s   (%s)\n", label, output.FormatTimeFull(ts), extra)
	} else {
		fmt.Printf("  %-10s %s\n", label, output.FormatTimeFull(ts))
	}
	return ts
}

// deltaLabel returns "+<dur> <name>" between two timestamps, or "" if either is
// missing/unparseable.
func deltaLabel(fromTS, toTS, name string) string {
	from, ok1 := output.ParseTimestamp(fromTS)
	to, ok2 := output.ParseTimestamp(toTS)
	if !ok1 || !ok2 {
		return ""
	}
	d := to.Sub(from)
	if d < 0 {
		return ""
	}
	return "+" + output.HumanDuration(d) + " " + name
}

func resultLabel(code string) string {
	if code == "" {
		return "—"
	}
	if n, err := strconv.Atoi(code); err == nil {
		switch {
		case n >= 200 && n < 300:
			return output.Good("HTTP " + code)
		case n >= 400:
			return output.Warn("HTTP " + code)
		}
	}
	return "HTTP " + code
}
