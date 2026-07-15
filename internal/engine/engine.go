// Package engine converts GIFs into colorized ASCII animation frames.
package engine

import (
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/jhayashi1/ascii-tui/internal/frames"
)

const (
	simpleRamp  = " .:-=+*#%@"
	complexRamp = " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$"

	// Terminal cells are roughly twice as tall as they are wide, so the
	// image is sampled at half vertical resolution to preserve aspect.
	cellAspect = 2
)

// Options control how a GIF is converted to ASCII frames.
type Options struct {
	Width      int
	Height     int
	Colored    bool
	Complex    bool
	CustomRamp string
}

// Render decodes a GIF from r and converts every frame to ASCII text.
// onProgress, if non-nil, is called after each converted frame.
func Render(r io.Reader, opts Options, onProgress func(done, total int)) (*frames.Animation, error) {
	g, err := gif.DecodeAll(r)
	if err != nil {
		return nil, fmt.Errorf("decoding gif: %w", err)
	}
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("gif contains no frames")
	}

	imgs := compositeFrames(g)
	bounds := imgs[0].Bounds()
	cols, rows := targetGrid(bounds.Dx(), bounds.Dy(), opts)
	ramp := []rune(opts.ramp())

	total := len(imgs)
	rendered := make([]string, total)
	delays := make([]time.Duration, total)
	for i, img := range imgs {
		rendered[i] = frameToASCII(img, cols, rows, opts.Colored, ramp)
		delays[i] = delayAt(g, i)
		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	return &frames.Animation{
		Width:  cols,
		Height: rows,
		Frames: rendered,
		Delays: delays,
	}, nil
}

func (o Options) ramp() string {
	switch {
	case o.CustomRamp != "":
		return o.CustomRamp
	case o.Complex:
		return complexRamp
	default:
		return simpleRamp
	}
}

// delayAt converts a GIF frame delay (centiseconds) to a duration.
// Delays under 20ms are normalized to 100ms, matching browser behavior
// for GIFs that specify no meaningful delay.
func delayAt(g *gif.GIF, i int) time.Duration {
	if i >= len(g.Delay) {
		return 100 * time.Millisecond
	}
	d := time.Duration(g.Delay[i]) * 10 * time.Millisecond
	if d < 20*time.Millisecond {
		return 100 * time.Millisecond
	}
	return d
}

// targetGrid picks the character grid size, preserving image aspect
// ratio when only one dimension (or neither) is specified.
func targetGrid(imgW, imgH int, opts Options) (cols, rows int) {
	switch {
	case opts.Width > 0 && opts.Height > 0:
		return opts.Width, opts.Height
	case opts.Width > 0:
		return opts.Width, max(1, opts.Width*imgH/(imgW*cellAspect))
	case opts.Height > 0:
		return max(1, opts.Height*imgW*cellAspect/imgH), opts.Height
	}

	termW, termH := 80, 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		termW, termH = w, h
	}
	cols = termW
	rows = max(1, cols*imgH/(imgW*cellAspect))
	if maxRows := termH - 1; rows > maxRows {
		rows = max(1, maxRows)
		cols = max(1, rows*imgW*cellAspect/imgH)
	}
	return cols, rows
}

// compositeFrames flattens the GIF's (possibly partial) frames onto a
// persistent canvas, honoring per-frame disposal methods, so that every
// returned image is a complete picture.
func compositeFrames(g *gif.GIF) []*image.RGBA {
	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	if bounds.Empty() {
		bounds = g.Image[0].Bounds()
	}
	canvas := image.NewRGBA(bounds)
	out := make([]*image.RGBA, 0, len(g.Image))
	for i, src := range g.Image {
		var backup *image.RGBA
		if disposalAt(g, i) == gif.DisposalPrevious {
			backup = cloneRGBA(canvas)
		}
		draw.Draw(canvas, src.Bounds(), src, src.Bounds().Min, draw.Over)
		out = append(out, cloneRGBA(canvas))
		switch disposalAt(g, i) {
		case gif.DisposalBackground:
			draw.Draw(canvas, src.Bounds(), image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = backup
		}
	}
	return out
}

func disposalAt(g *gif.GIF, i int) byte {
	if i >= len(g.Disposal) {
		return gif.DisposalNone
	}
	return g.Disposal[i]
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}
