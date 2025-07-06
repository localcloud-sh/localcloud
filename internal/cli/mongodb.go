// internal/cli/mongodb.go
package cli

import (
	"github.com/spf13/cobra"
)

var mongoCmd = &cobra.Command{
	Use:   "mongo",
	Short: "MongoDB management commands",
	Long:  `Manage MongoDB database operations. Use 'lc export mongo' for database exports.`,
}

func init() {
	// Add to root command
	rootCmd.AddCommand(mongoCmd)
}
