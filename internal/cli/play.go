package cli

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jhayashi1/ascii-tui/internal/engine"
	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
	"github.com/jhayashi1/ascii-tui/internal/player"
)

func init() {
	var once bool
	opts := player.Options{}

	playCmd := &cobra.Command{
		Use:   "play <file>",
		Short: "Play a frames file, or render a GIF on the fly and play it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			anim, err := loadAnimation(args[0], cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			opts.Loop = !once

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			player.EnableVirtualTerminal(os.Stdout)
			return player.Play(ctx, os.Stdout, anim, opts)
		},
	}

	playCmd.Flags().BoolVar(&once, "once", false, "play a single pass instead of looping")
	playCmd.Flags().Float64Var(&opts.Speed, "speed", 1, "playback speed multiplier")
	rootCmd.AddCommand(playCmd)
}

// loadAnimation reads a frames file, or renders a .gif in memory with
// default options.
func loadAnimation(path string, progressOut io.Writer) (*frames.Animation, error) {
	if strings.EqualFold(filepath.Ext(path), ".gif") {
		return renderGif(path, engine.Options{Colored: true}, progressOut)
	}
	return library.Load(path)
}
