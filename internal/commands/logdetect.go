package commands

import (
	"sort"
	"strings"
)

// logIssuePattern maps a substring to detect to a short category label. Order
// matters only for which representative line is kept first.
type logIssuePattern struct {
	needle   string
	category string
}

var logIssuePatterns = []logIssuePattern{
	{"Traceback (most recent call last)", "python-traceback"},
	{"'NoneType' object", "nonetype"},
	{"NoneType", "nonetype"},
	{"CUDA out of memory", "cuda-oom"},
	{"OutOfMemoryError", "cuda-oom"},
	{"CUDA error: out of memory", "cuda-oom"},
	{"torch.cuda.OutOfMemoryError", "cuda-oom"},
	{"OOMKilled", "oom-killed"},
	{"Segmentation fault", "segfault"},
	{"core dumped", "segfault"},
	{"ModuleNotFoundError", "import-error"},
	{"ImportError", "import-error"},
	{"Address already in use", "port-conflict"},
	{"Connection refused", "connection-refused"},
	{"RuntimeError", "runtime-error"},
	{"AssertionError", "assertion-error"},
	{"KeyError", "key-error"},
}

// logIssue summarizes one detected problem category in a log blob.
type logIssue struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
	Sample   string `json:"sample"`
}

// detectLogIssues scans log text for worker-failure signatures (tracebacks,
// NoneType, CUDA OOM, OOM-kills, ...) and returns a per-category count with a
// representative line, most frequent first.
func detectLogIssues(logs string) []logIssue {
	if logs == "" {
		return nil
	}
	counts := map[string]int{}
	samples := map[string]string{}
	for _, line := range strings.Split(logs, "\n") {
		for _, p := range logIssuePatterns {
			if strings.Contains(line, p.needle) {
				counts[p.category]++
				if samples[p.category] == "" {
					samples[p.category] = strings.TrimSpace(line)
				}
			}
		}
	}
	if len(counts) == 0 {
		return nil
	}
	issues := make([]logIssue, 0, len(counts))
	for cat, n := range counts {
		sample := samples[cat]
		if len(sample) > 200 {
			sample = sample[:197] + "..."
		}
		issues = append(issues, logIssue{Category: cat, Count: n, Sample: sample})
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Count != issues[j].Count {
			return issues[i].Count > issues[j].Count
		}
		return issues[i].Category < issues[j].Category
	})
	return issues
}
