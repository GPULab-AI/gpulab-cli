package commands

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
	"github.com/GPULab-AI/gpulab-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	healthLimit int
	healthLogs  bool
	healthStale bool
)

func init() {
	serverlessCmd.AddCommand(serverlessHealthCmd)
	serverlessHealthCmd.Flags().IntVar(&healthLimit, "limit", 200, "Number of recent requests to analyze")
	serverlessHealthCmd.Flags().BoolVar(&healthLogs, "logs", false, "Scan running replica logs for tracebacks/OOM")
	serverlessHealthCmd.Flags().BoolVar(&healthStale, "stale", false, "Also flag replicas running code older than the volume")
}

var serverlessHealthCmd = &cobra.Command{
	Use:   "health [ENDPOINT]",
	Short: "Summarize an endpoint's health: error rate, latency, queue, crashes, cold starts",
	Long: `Aggregate an endpoint's recent request logs, replicas, and autoscaling activity
into a single health surface: success/error rates, 4xx/5xx breakdown, latency,
queue depth, scale events, and (with --logs) detected worker tracebacks or CUDA
OOMs.`,
	Args: cobra.ExactArgs(1),
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
		svc := detail.Data

		// Recent requests, up to the limit.
		var reqs []api.ServerlessRequestLog
		var reqFetchErr error
		for page := 1; len(reqs) < healthLimit; page++ {
			p, perr := client.ListServerlessRequests(identifier, page, 100)
			if perr != nil {
				if page == 1 {
					reqFetchErr = perr
				}
				break
			}
			reqs = append(reqs, p.Data...)
			if page >= p.LastPage {
				break
			}
		}
		if len(reqs) > healthLimit {
			reqs = reqs[:healthLimit]
		}
		stats := computeRequestStats(reqs)

		asPage, _ := client.ListServerlessAutoscalingLogs(identifier, 1, 100)
		asStats := computeAutoscalingStats(asPage)

		var issues []logIssue
		if healthLogs {
			issues = scanReplicaIssues(client, detail.Replicas)
		}

		var stale map[string]*staleResult
		if healthStale {
			stale, _ = computeReplicaStaleness(client, svc.NetworkVolumeUUID, "", detail.Replicas, 2)
		}

		verdict := healthVerdict(stats, svc, issues, reqFetchErr == nil)

		if flagJSON {
			output.PrintJSON(map[string]interface{}{
				"endpoint":    identifier,
				"name":        svc.Name,
				"verdict":     verdict,
				"replicas":    map[string]int{"active": svc.ActiveReplicasCount, "provisioning": svc.ProvisioningReplicasCount, "min": svc.MinReplicas, "max": svc.MaxReplicas},
				"requests":    stats,
				"autoscaling": asStats,
				"queued_now":  svc.QueuedRequestsCount,
				"issues":      issues,
			})
			return nil
		}

		printHealth(svc, identifier, verdict, stats, asStats, detail.Replicas, stale, issues, len(reqs))
		return nil
	},
}

type requestStats struct {
	Total     int            `json:"total"`
	Success   int            `json:"success"`
	ClientErr int            `json:"client_errors"`
	NotFound  int            `json:"not_found"`
	ServerErr int            `json:"server_errors"`
	Errored   int            `json:"errored"`
	Errors    int            `json:"errors"` // deduplicated: 5xx OR errored (each request once)
	Statuses  map[string]int `json:"statuses"`
	AvgMS     int64          `json:"avg_ms"`
	P95MS     int64          `json:"p95_ms"`
	ErrorRate float64        `json:"error_rate"`
	durations []int64        // internal
}

func computeRequestStats(reqs []api.ServerlessRequestLog) requestStats {
	s := requestStats{Statuses: map[string]int{}}
	s.Total = len(reqs)
	for _, r := range reqs {
		if r.Status != "" {
			s.Statuses[r.Status]++
		}
		isErr := false
		if r.ResponseStatusCode != nil {
			c := *r.ResponseStatusCode
			switch {
			case c >= 200 && c < 300:
				s.Success++
			case c == 404:
				s.NotFound++
				s.ClientErr++
			case c >= 400 && c < 500:
				s.ClientErr++
			case c >= 500:
				s.ServerErr++
				isErr = true
			}
		}
		if strings.EqualFold(r.Status, "error") || strings.EqualFold(r.Status, "failed") || r.ErrorMessage != "" {
			s.Errored++
			isErr = true
		}
		// Count each failed request once so the rate can't exceed 100%.
		if isErr {
			s.Errors++
		}
		if r.DurationMS != nil {
			s.durations = append(s.durations, *r.DurationMS)
		}
	}
	if len(s.durations) > 0 {
		sorted := append([]int64(nil), s.durations...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		var sum int64
		for _, d := range sorted {
			sum += d
		}
		s.AvgMS = sum / int64(len(sorted))
		s.P95MS = sorted[int(float64(len(sorted)-1)*0.95)]
	}
	if s.Total > 0 {
		s.ErrorRate = float64(s.Errors) / float64(s.Total) * 100
	}
	return s
}

type autoscalingStats struct {
	ScaleUps     int    `json:"scale_ups"`
	ScaleDowns   int    `json:"scale_downs"`
	Errors       int    `json:"errors"`
	IdleTeardown int    `json:"idle_teardowns"`
	ForceKills   int    `json:"force_kills"`
	ColdStarts   int    `json:"cold_starts"`
	LastEvent    string `json:"last_event"`
	LastError    string `json:"last_error"`
	PingPong     bool   `json:"ping_pong"`
	WindowCount  int    `json:"window_count"`
}

func computeAutoscalingStats(page *api.ServerlessPage[api.ServerlessAutoscalingLog]) autoscalingStats {
	var s autoscalingStats
	if page == nil {
		return s
	}
	s.WindowCount = len(page.Data)
	var lastEventTime, lastErrorTime time.Time
	var dirs []int // scale directions in chronological order: +1 up, -1 down
	// The API returns newest-first; walk in reverse to get chronological order.
	for i := len(page.Data) - 1; i >= 0; i-- {
		log := page.Data[i]
		t, _ := output.ParseTimestamp(log.CreatedAt)
		if !t.IsZero() && (lastEventTime.IsZero() || t.After(lastEventTime)) {
			lastEventTime = t
			s.LastEvent = log.Event
		}
		if strings.EqualFold(log.Level, "error") {
			s.Errors++
			if !t.IsZero() && (lastErrorTime.IsZero() || t.After(lastErrorTime)) {
				lastErrorTime = t
				s.LastError = log.CreatedAt
			}
		}
		blob := strings.ToLower(log.Event + " " + log.Message)
		switch {
		case log.CurrentReplicas != nil && log.DesiredReplicas != nil && *log.DesiredReplicas > *log.CurrentReplicas:
			s.ScaleUps++
			dirs = append(dirs, 1)
		case log.CurrentReplicas != nil && log.DesiredReplicas != nil && *log.DesiredReplicas < *log.CurrentReplicas:
			s.ScaleDowns++
			dirs = append(dirs, -1)
		case strings.Contains(blob, "scale up") || strings.Contains(blob, "scale_up") || strings.Contains(blob, "scaled up"):
			s.ScaleUps++
			dirs = append(dirs, 1)
		case strings.Contains(blob, "scale down") || strings.Contains(blob, "scale_down") || strings.Contains(blob, "scaled down"):
			s.ScaleDowns++
			dirs = append(dirs, -1)
		}
		if strings.Contains(blob, "idle") {
			s.IdleTeardown++
		}
		if strings.Contains(blob, "force") || strings.Contains(blob, "kill") {
			s.ForceKills++
		}
		if strings.Contains(blob, "cold start") || strings.Contains(blob, "cold_start") || strings.Contains(blob, "provision") {
			s.ColdStarts++
		}
	}
	// Ping-pong = actual back-and-forth, not just high counts (which slow cold
	// starts can inflate). Require several direction reversals.
	transitions := 0
	for i := 1; i < len(dirs); i++ {
		if dirs[i] != dirs[i-1] {
			transitions++
		}
	}
	s.PingPong = transitions >= 4
	return s
}

// printAutoscalingSummary prints a one-glance summary of scaling activity above
// the raw autoscaling-log table.
func printAutoscalingSummary(s autoscalingStats) {
	fmt.Printf("Summary (last %d events): %d up, %d down, %d cold-start, %d idle-teardown, %d force-kill, %d error\n",
		s.WindowCount, s.ScaleUps, s.ScaleDowns, s.ColdStarts, s.IdleTeardown, s.ForceKills, s.Errors)
	if s.PingPong {
		fmt.Println(output.Warn("⚠ scaling ping-pong detected (frequent up+down — check idle-timeout vs traffic)"))
	}
	fmt.Println()
}

func scanReplicaIssues(client *api.Client, replicas []api.ServerlessReplica) []logIssue {
	merged := map[string]*logIssue{}
	for _, r := range replicas {
		switch strings.ToLower(r.Status) {
		case "running", "healthy", "unhealthy":
		default:
			continue
		}
		logs, err := client.GetContainerLogs(r.UUID, 300, "", false)
		if err != nil {
			continue
		}
		for _, iss := range detectLogIssues(logs) {
			if existing, ok := merged[iss.Category]; ok {
				existing.Count += iss.Count
			} else {
				copy := iss
				merged[iss.Category] = &copy
			}
		}
	}
	out := make([]logIssue, 0, len(merged))
	for _, v := range merged {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out
}

func healthVerdict(stats requestStats, svc api.ServerlessService, issues []logIssue, fetchOK bool) string {
	switch {
	case !fetchOK && stats.Total == 0:
		// Couldn't read request logs — don't claim healthy when we're blind.
		return "unknown"
	case stats.Total > 0 && stats.ErrorRate >= 20:
		return "unhealthy"
	case len(issues) > 0 || (stats.Total > 0 && stats.ErrorRate >= 5) || (svc.ActiveReplicasCount == 0 && svc.MinReplicas > 0):
		return "degraded"
	default:
		return "healthy"
	}
}

func printHealth(svc api.ServerlessService, identifier, verdict string, stats requestStats, as autoscalingStats, replicas []api.ServerlessReplica, stale map[string]*staleResult, issues []logIssue, window int) {
	verdictColored := verdict
	switch verdict {
	case "healthy":
		verdictColored = output.Good(verdict)
	case "degraded", "unhealthy", "unknown":
		verdictColored = output.Warn(verdict)
	}
	fmt.Printf("Health:    %s  —  %s (%s)\n", verdictColored, svc.Name, shortID(identifier))
	fmt.Printf("Endpoint:  %s, autoscaling %s\n", boolStatus(svc.IsEnabled, "active", "paused"), boolStatus(svc.AutoscalingEnabled, "on", "off"))
	fmt.Printf("Replicas:  %d active, %d provisioning (min %d, max %d)\n", svc.ActiveReplicasCount, svc.ProvisioningReplicasCount, svc.MinReplicas, svc.MaxReplicas)

	if len(replicas) > 0 {
		printServerlessReplicas(replicas, stale)
	}

	fmt.Printf("\nRequests (last %d):\n", window)
	fmt.Printf("  total       %d\n", stats.Total)
	fmt.Printf("  success     %d\n", stats.Success)
	fmt.Printf("  4xx         %d (404: %d)\n", stats.ClientErr, stats.NotFound)
	fmt.Printf("  5xx         %d\n", stats.ServerErr)
	fmt.Printf("  errored     %d\n", stats.Errored)
	fmt.Printf("  error rate  %.1f%%\n", stats.ErrorRate)
	if stats.AvgMS > 0 {
		fmt.Printf("  latency     avg %dms, p95 %dms\n", stats.AvgMS, stats.P95MS)
	}
	fmt.Printf("  queued now  %d\n", svc.QueuedRequestsCount)

	fmt.Printf("\nAutoscaling (last %d events):\n", as.WindowCount)
	fmt.Printf("  scale-ups   %d\n", as.ScaleUps)
	fmt.Printf("  scale-downs %d\n", as.ScaleDowns)
	fmt.Printf("  cold-starts %d\n", as.ColdStarts)
	fmt.Printf("  idle-tears  %d\n", as.IdleTeardown)
	fmt.Printf("  force-kills %d\n", as.ForceKills)
	fmt.Printf("  errors      %d\n", as.Errors)
	if as.PingPong {
		fmt.Printf("  %s\n", output.Warn("⚠ scaling ping-pong detected (frequent up+down)"))
	}
	if as.LastError != "" {
		fmt.Printf("  last error  %s\n", shortTime(as.LastError))
	}

	if len(issues) > 0 {
		fmt.Printf("\nDetected issues in replica logs:\n")
		for _, iss := range issues {
			fmt.Printf("  %s  x%d  %s\n", output.Warn(iss.Category), iss.Count, iss.Sample)
		}
	}
}
