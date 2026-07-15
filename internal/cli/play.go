package cli

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jhayashi1/ascii-tui/internal/engine"
	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
	"github.com/jhayashi1/ascii-tui/internal/pathutil"
	"github.com/jhayashi1/ascii-tui/internal/player"
)

func init() {
	var once bool
	opts := player.Options{}
	renderOpts := engine.Options{Colored: true}

	playCmd := &cobra.Command{
		Use:   "play <file>",
		Short: "Play a frames file, or render a GIF on the fly and play it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			anim, err := loadAnimation(pathutil.ExpandTilde(args[0]), renderOpts, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			opts.Loop = !once
			return player.Run(anim, opts)
		},
	}

	playCmd.Flags().BoolVar(&once, "once", false, "play a single pass instead of looping")
	playCmd.Flags().Float64Var(&opts.Speed, "speed", 1, "playback speed multiplier")
	playCmd.Flags().BoolVar(&renderOpts.FilterBackground, "filter-bg", false, "when rendering a gif, render a detected solid background as blank space")
	rootCmd.AddCommand(playCmd)
}

func isGif(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".gif")
}

// loadAnimation reads a frames file, or renders a .gif in memory with
// the given options.
func loadAnimation(path string, renderOpts engine.Options, progressOut io.Writer) (*frames.Animation, error) {
	if isGif(path) {
		return renderGif(path, renderOpts, progressOut)
	}
	return library.Load(path)
}
