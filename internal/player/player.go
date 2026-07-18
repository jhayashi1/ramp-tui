// Package player renders pre-rendered animations to a raw terminal
// using the alternate screen buffer and cursor home positioning.
package player

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jhayashi1/ramp-tui/internal/frames"
)

const (
	enterAltScreen = "\x1b[?1049h"
	exitAltScreen  = "\x1b[?1049l"
	hideCursor     = "\x1b[?25l"
	showCursor     = "\x1b[?25h"
	cursorHome     = "\x1b[H"
	clearScreen    = "\x1b[2J"
)

// Options control playback behavior.
type Options struct {
	Loop  bool
	Speed float64
}

// Run plays the animation on stdout, enabling Windows VT processing
// and restoring the terminal on Ctrl+C or SIGTERM.
func Run(anim *frames.Animation, opts Options) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	EnableVirtualTerminal(os.Stdout)
	return Play(ctx, os.Stdout, anim, opts)
}

// Play writes the animation to w frame by frame until ctx is canceled
// or, when not looping, one pass completes. The terminal state is
// restored before returning.
func Play(ctx context.Context, w io.Writer, anim *frames.Animation, opts Options) error {
	if len(anim.Frames) == 0 {
		return errors.New("animation has no frames")
	}
	if len(anim.Delays) != len(anim.Frames) {
		return fmt.Errorf("animation is corrupt: %d frames but %d delays",
			len(anim.Frames), len(anim.Delays))
	}

	speed := opts.Speed
	if speed <= 0 {
		speed = 1
	}

	// Frames are stored with bare newlines; explicit CRLF keeps the
	// column at zero regardless of terminal line-discipline settings.
	prepared := make([]string, len(anim.Frames))
	for i, f := range anim.Frames {
		prepared[i] = cursorHome + strings.ReplaceAll(f, "\n", "\r\n")
	}

	// Write errors are sticky in bufio and surface at the checked
	// Flush inside the playback loop.
	bw := bufio.NewWriterSize(w, 1<<16)
	_, _ = bw.WriteString(enterAltScreen + hideCursor + clearScreen)
	defer func() {
		_, _ = bw.WriteString(showCursor + exitAltScreen)
		_ = bw.Flush()
	}()

	for {
		for i, frame := range prepared {
			if err := ctx.Err(); err != nil {
				return nil
			}
			if _, err := bw.WriteString(frame); err != nil {
				return err
			}
			if err := bw.Flush(); err != nil {
				return err
			}

			delay := time.Duration(float64(anim.Delays[i]) / speed)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
		}
		if !opts.Loop {
			return nil
		}
	}
}
