package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"os"
	"strings"
	"testing"
	"time"
)

// paletteFrame builds a single-color paletted frame at the given rect.
func paletteFrame(rect image.Rectangle, c color.RGBA) *image.Paletted {
	p := image.NewPaletted(rect, color.Palette{c})
	for i := range p.Pix {
		p.Pix[i] = 0
	}
	return p
}

func TestCompositeRetainsPreviousFramePixels(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	g := &gif.GIF{
		Image: []*image.Paletted{
			paletteFrame(image.Rect(0, 0, 4, 4), red),
			paletteFrame(image.Rect(0, 0, 1, 1), blue),
		},
		Delay:    []int{10, 10},
		Disposal: []byte{gif.DisposalNone, gif.DisposalNone},
		Config:   image.Config{Width: 4, Height: 4},
	}

	frames := compositeFrames(g)
	if len(frames) != 2 {
		t.Fatalf("frame count = %d, want 2", len(frames))
	}

	second := frames[1]
	if got := second.RGBAAt(0, 0); got.B != 255 {
		t.Errorf("pixel (0,0) = %v, want blue overlay", got)
	}
	if got := second.RGBAAt(3, 3); got.R != 255 {
		t.Errorf("pixel (3,3) = %v, want red retained from frame 1", got)
	}
}

func TestCompositeDisposalBackgroundClears(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	g := &gif.GIF{
		Image: []*image.Paletted{
			paletteFrame(image.Rect(0, 0, 2, 2), red),
			paletteFrame(image.Rect(0, 0, 1, 1), blue),
		},
		Delay:    []int{10, 10},
		Disposal: []byte{gif.DisposalBackground, gif.DisposalNone},
		Config:   image.Config{Width: 2, Height: 2},
	}

	frames := compositeFrames(g)
	if got := frames[1].RGBAAt(1, 1); got.R != 0 || got.A != 0 {
		t.Errorf("pixel (1,1) = %v, want cleared after background disposal", got)
	}
	if got := frames[1].RGBAAt(0, 0); got.B != 255 {
		t.Errorf("pixel (0,0) = %v, want blue", got)
	}
}

func TestRenderSampleGif(t *testing.T) {
	f, err := os.Open("../../testdata/giphy.gif")
	if err != nil {
		t.Fatalf("opening sample gif: %v", err)
	}
	defer f.Close()

	progressCalls := 0
	anim, err := Render(f, Options{Width: 80, Colored: true}, func(_, _ int) {
		progressCalls++
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(anim.Frames) < 2 {
		t.Fatalf("frame count = %d, want at least 2", len(anim.Frames))
	}
	if len(anim.Delays) != len(anim.Frames) {
		t.Fatalf("delays = %d, frames = %d, want equal", len(anim.Delays), len(anim.Frames))
	}
	if progressCalls != len(anim.Frames) {
		t.Errorf("progress calls = %d, want %d", progressCalls, len(anim.Frames))
	}
	if anim.Width != 80 {
		t.Errorf("width = %d, want 80", anim.Width)
	}

	wantLines := anim.Height
	for i, frame := range anim.Frames {
		if got := strings.Count(frame, "\n") + 1; got != wantLines {
			t.Errorf("frame %d has %d lines, want %d", i, got, wantLines)
		}
	}
	if !strings.Contains(anim.Frames[0], "\x1b[38;2;") {
		t.Error("first frame has no truecolor escape; colored render is broken")
	}
	for i, d := range anim.Delays {
		if d < 20*time.Millisecond {
			t.Errorf("delay %d = %v, want >= 20ms", i, d)
		}
	}
}

func TestRenderUncoloredHasNoEscapes(t *testing.T) {
	f, err := os.Open("../../testdata/giphy.gif")
	if err != nil {
		t.Fatalf("opening sample gif: %v", err)
	}
	defer f.Close()

	anim, err := Render(f, Options{Width: 40}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if bytes.ContainsRune([]byte(anim.Frames[0]), '\x1b') {
		t.Error("uncolored frame contains ANSI escapes")
	}
}
