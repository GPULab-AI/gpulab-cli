package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatQuiet Format = "quiet"
)

func PrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func PrintTable(headers []string, rows [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(rows)
	table.Render()
}

func PrintQuiet(values []string) {
	for _, v := range values {
		fmt.Println(v)
	}
}

func StatusColor(status string) string {
	lower := strings.ToLower(status)
	switch {
	case lower == "running" || lower == "healthy":
		return color.GreenString(status)
	case lower == "stopped" || lower == "paused":
		return color.YellowString(status)
	case lower == "failed" || lower == "timeout" || lower == "unhealthy":
		return color.RedString(status)
	case lower == "deploying" || lower == "redeploying" || lower == "pulling" || lower == "creating" || lower == "starting":
		return color.CyanString(status)
	default:
		return status
	}
}

func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", color.RedString("Error:"), msg)
}

func PrintSuccess(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", color.GreenString("Success:"), msg)
}

// FormatBytes converts bytes to human readable format
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
