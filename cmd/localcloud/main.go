package main

import (
	"embed"

	"github.com/localcloud-sh/localcloud/internal/cli"
)

// Embed template files
//
//go:embed templates/*
var templatesFS embed.FS

func main() {
	// Add template commands
	cli.AddTemplateCommands(
		cli.SetupCmd(templatesFS),
		cli.TemplatesCmd(),
	)

	// Execute root command
	cli.Execute()
}
