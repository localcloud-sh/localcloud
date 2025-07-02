// internal/cli/doctor.go
package cli

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/localcloud-sh/localcloud/internal/config"
	"github.com/localcloud-sh/localcloud/internal/diagnostics"
	"github.com/spf13/cobra"
)

var (
	doctorFix bool
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and fix common issues",
	Long: `Run diagnostics to check for common issues and get solutions.
	
Doctor checks:
  - Docker daemon status
  - Docker version compatibility
  - System resources (CPU, memory, disk)
  - Port availability
  - Network connectivity
  - Configuration validity
  - Service dependencies`,
	Example: `  lc doctor         # Run diagnostics
  lc doctor --fix   # Run diagnostics and attempt fixes`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Attempt to fix issues automatically")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	// Get config (may be nil if not initialized)
	cfg := config.Get()

	// Create doctor
	doctor := diagnostics.NewDoctor(cfg)

	// Run diagnostics
	fmt.Println(infoColor("LocalCloud Doctor"))
	fmt.Println(strings.Repeat("━", 50))
	fmt.Println()

	if err := doctor.RunDiagnostics(); err != nil {
		return fmt.Errorf("failed to run diagnostics: %w", err)
	}

	// Display results
	results := doctor.GetResults()

	var (
		okCount      int
		warningCount int
		errorCount   int
		skippedCount int
	)

	for _, result := range results {
		displayDiagnosticResult(result)

		switch result.Status {
		case diagnostics.CheckStatusOK:
			okCount++
		case diagnostics.CheckStatusWarning:
			warningCount++
		case diagnostics.CheckStatusError:
			errorCount++
		case diagnostics.CheckStatusSkipped:
			skippedCount++
		}
	}

	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("━", 50))
	fmt.Printf("\n%s ", infoColor("Summary:"))

	if errorCount == 0 && warningCount == 0 {
		fmt.Println(successColor("All checks passed!"))
		fmt.Println("\nYour system is ready for LocalCloud.")
	} else {
		parts := []string{}
		if okCount > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", okCount, successColor("passed")))
		}
		if warningCount > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", warningCount, warningColor("warnings")))
		}
		if errorCount > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", errorCount, errorColor("errors")))
		}
		if skippedCount > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped", skippedCount))
		}

		fmt.Println(strings.Join(parts, ", "))

		if errorCount > 0 {
			fmt.Println("\n" + errorColor("Critical issues found. Please resolve them before continuing."))
		} else if warningCount > 0 {
			fmt.Println("\n" + warningColor("Some warnings detected. LocalCloud may work but with limitations."))
		}
	}

	// Show quick fixes
	if doctor.HasIssues() && !doctorFix {
		fmt.Println("\nRun 'lc doctor --fix' to attempt automatic fixes")
	}

	// Attempt fixes if requested
	if doctorFix && doctor.HasIssues() {
		fmt.Println("\n" + infoColor("Attempting automatic fixes..."))
		// This would implement automatic fixes
		// For now, just show a message
		fmt.Println(warningColor("Automatic fixes not yet implemented"))
	}

	return nil
}

func displayDiagnosticResult(result diagnostics.DiagnosticResult) {
	// Choose icon and color based on status
	var icon string
	var statusColorFunc func(a ...interface{}) string

	switch result.Status {
	case diagnostics.CheckStatusOK:
		icon = "✓"
		statusColorFunc = successColor
	case diagnostics.CheckStatusWarning:
		icon = "!"
		statusColorFunc = warningColor
	case diagnostics.CheckStatusError:
		icon = "✗"
		statusColorFunc = errorColor
	case diagnostics.CheckStatusSkipped:
		icon = "○"
		statusColorFunc = color.New(color.FgWhite).SprintFunc()
	}

	// Display check result
	fmt.Printf("%s %s: %s\n",
		statusColorFunc(icon),
		result.Check,
		result.Message)

	// Show solution if there's an error or warning
	if result.Status == diagnostics.CheckStatusError || result.Status == diagnostics.CheckStatusWarning {
		if result.Solution != "" {
			// Format solution with indentation
			lines := strings.Split(result.Solution, "\n")
			for _, line := range lines {
				if line != "" {
					fmt.Printf("  %s %s\n", color.New(color.FgYellow).Sprint("→"), line)
				}
			}
		}
	}

	// Show details in verbose mode
	if verbose && result.Details != nil && len(result.Details) > 0 {
		fmt.Println("  Details:")
		for key, value := range result.Details {
			fmt.Printf("    %s: %v\n", key, value)
		}
	}
}
