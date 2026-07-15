// Package cli implements the ascii-tui command line interface.
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ascii-tui",
	Short: "Convert GIFs into colorized ASCII animations and play them in the terminal",
}

// Execute runs the CLI and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
