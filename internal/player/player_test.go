package player

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jhayashi1/ascii-tui/internal/frames"
)

func TestPlaySinglePass(t *testing.T) {
	anim := &frames.Animation{
		Width:  3,
		Height: 2,
		Frames: []string{"ab\ncd", "ef\ngh"},
		Delays: []time.Duration{time.Millisecond, time.Millisecond},
	}

	var buf bytes.Buffer
	err := Play(context.Background(), &buf, anim, Options{Loop: false})
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	out := buf.String()

	if !strings.HasPrefix(out, enterAltScreen+hideCursor+clearScreen) {
		t.Error("output does not start with terminal setup sequence")
	}
	if !strings.HasSuffix(out, showCursor+exitAltScreen) {
		t.Error("output does not end with terminal restore sequence")
	}
	if got := strings.Count(out, cursorHome); got != len(anim.Frames) {
		t.Errorf("cursor home count = %d, want %d", got, len(anim.Frames))
	}
	if !strings.Contains(out, "ab\r\ncd") {
		t.Error("newlines were not converted to CRLF")
	}
}

func TestPlayRejectsInvalidAnimation(t *testing.T) {
	var buf bytes.Buffer
	empty := &frames.Animation{}
	if err := Play(context.Background(), &buf, empty, Options{Loop: true}); err == nil {
		t.Error("want error for empty animation, got nil")
	}

	mismatched := &frames.Animation{Frames: []string{"x"}}
	if err := Play(context.Background(), &buf, mismatched, Options{}); err == nil {
		t.Error("want error for mismatched delays, got nil")
	}
}

func TestPlayStopsOnCancel(t *testing.T) {
	anim := &frames.Animation{
		Frames: []string{"x"},
		Delays: []time.Duration{time.Hour},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var buf bytes.Buffer
	go func() {
		done <- Play(ctx, &buf, anim, Options{Loop: true})
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Play: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Play did not stop after context cancel")
	}
}
