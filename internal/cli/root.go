// Package cli implements the ascii-tui command line interface.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jhayashi1/ascii-tui/internal/engine"
	"github.com/jhayashi1/ascii-tui/internal/library"
	"github.com/jhayashi1/ascii-tui/internal/player"
	"github.com/jhayashi1/ascii-tui/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "ascii-tui [gif]",
	Short: "Convert GIFs into colorized ASCII animations and play them in the terminal",
	Long: "With no arguments, opens the interactive gallery of rendered animations.\n" +
		"With a .gif argument, renders it into the library and plays it.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := library.DefaultDir()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			return tui.Run(dir)
		}
		return renderAndPlay(cmd, dir, args[0])
	},
}

// Execute runs the CLI and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// renderAndPlay is the shortcut path: render a gif into the library,
// then loop it in the raw player.
func renderAndPlay(cmd *cobra.Command, dir, path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".gif") {
		return fmt.Errorf("expected a .gif file, got %s", path)
	}
	anim, err := renderGif(path, engine.Options{Colored: true}, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	saved, err := library.Save(dir, anim)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "saved to library: %s\n", saved)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	player.EnableVirtualTerminal(os.Stdout)
	return player.Play(ctx, os.Stdout, anim, player.Options{Loop: true, Speed: 1})
}
