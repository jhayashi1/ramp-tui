// Package cli implements the ramp command line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jhayashi1/ramp-tui/internal/config"
	"github.com/jhayashi1/ramp-tui/internal/engine"
	"github.com/jhayashi1/ramp-tui/internal/library"
	"github.com/jhayashi1/ramp-tui/internal/pathutil"
	"github.com/jhayashi1/ramp-tui/internal/player"
	"github.com/jhayashi1/ramp-tui/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "ramp [gif]",
	Short: "Convert GIFs into colorized ASCII animations and play them in the terminal",
	Long: "With no arguments, opens the interactive gallery of rendered animations.\n" +
		"With a .gif argument, renders it into the library and plays it.",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := library.DefaultDir()
		if err != nil {
			return err
		}
		cfg := config.Load()
		if len(args) == 0 {
			return tui.Run(dir, cfg)
		}
		return renderAndPlay(cmd, dir, pathutil.ExpandTilde(args[0]), cfg)
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
func renderAndPlay(cmd *cobra.Command, dir, path string, cfg config.Config) error {
	if !isGif(path) {
		return fmt.Errorf("expected a .gif file, got %s", path)
	}
	opts := engine.Options{
		Colored:          true,
		FilterBackground: cfg.Render.FilterBackground,
		Complex:          cfg.Render.Complex,
	}
	anim, err := renderGif(path, opts, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	saved, err := library.Save(dir, anim)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "saved to library: %s\n", saved)

	return player.Run(anim, player.Options{Loop: true, Speed: cfg.Playback.Speed})
}
