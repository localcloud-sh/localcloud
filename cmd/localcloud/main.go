package main

import (
	"embed"

	"github.com/localcloud-sh/localcloud/internal/cli"
)

// Embed template files
var templatesFS embed.FS

func main() {
	// Set version information (Version, Commit, BuildDate are defined in version.go)
	cli.SetVersionInfo(Version, Commit, BuildDate)
	
	// Initialize template filesystem for setup command
	cli.InitializeTemplateFS(templatesFS)

	// Execute root command
	cli.Execute()
}
