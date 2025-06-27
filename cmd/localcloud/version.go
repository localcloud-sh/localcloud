package main

import "fmt"

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func versionInfo() string {
	return fmt.Sprintf("LocalCloud %s (%s) built on %s", Version, Commit, BuildDate)
}
