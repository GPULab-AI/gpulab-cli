package commands

import (
	"fmt"
	"regexp"
	"strings"
)

// grepLogs keeps only the log lines matching pattern (RE2 syntax). An empty
// pattern returns the input unchanged. The result keeps a trailing newline when
// non-empty so it composes with printLogChunk.
func grepLogs(logs, pattern string) (string, error) {
	if pattern == "" || logs == "" {
		return logs, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid --grep pattern %q: %w", pattern, err)
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(logs, "\n"), "\n") {
		if re.MatchString(line) {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return "", nil
	}
	return strings.Join(kept, "\n") + "\n", nil
}

// tailLines returns the last n lines of logs (n <= 0 returns everything). Used
// for client-side tailing of deploy logs, which the API returns whole.
func tailLines(logs string, n int) string {
	if n <= 0 || logs == "" {
		return logs
	}
	lines := strings.Split(strings.TrimRight(logs, "\n"), "\n")
	if len(lines) <= n {
		return logs
	}
	return strings.Join(lines[len(lines)-n:], "\n") + "\n"
}
