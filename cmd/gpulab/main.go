package main

import (
	"github.com/gpulab/gpulab-cli/internal/commands"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	commands.SetVersionInfo(version, commit, date)
	commands.Execute()
}
