// Package engine converts GIFs into colorized ASCII animation frames.
package engine

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"io"
	"time"

	"github.com/jhayashi1/ramp-tui/internal/frames"
)

const (
	simpleRamp  = " .:-=+*#%@"
	complexRamp = " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$"

	// Terminal cells are roughly twice as tall as they are wide, so the
	// image is sampled at half vertical resolution to preserve aspect.
	cellAspect = 2

	defaultMaxWidth  = 80
	defaultMaxHeight = 23
)

// Options control how a GIF is converted to ASCII frames.
type Options struct {
	// Width and Height set the exact character grid; when only one is
	// set the other is derived from the image aspect ratio.
	Width  int
	Height int
	// MaxWidth and MaxHeight bound the auto-fitted grid when Width and
	// Height are both zero. Callers resolve these from their display;
	// the engine never inspects the terminal itself.
	MaxWidth  int
	MaxHeight int
	Colored   bool
	Complex   bool
	// FilterBackground detects a solid background color from the first
	// frame's border and renders matching pixels as blank space.
	FilterBackground bool
	CustomRamp       string
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

	bounds := canvasBounds(g)
	cols, rows := targetGrid(bounds.Dx(), bounds.Dy(), opts)
	ramp := []rune(opts.ramp())
	comp := newCompositor(bounds)

	total := len(g.Image)
	rendered := make([]string, total)
	delays := make([]time.Duration, total)
	var (
		bg      color.RGBA
		bgFound bool
		masked  *image.RGBA
	)
	for i, src := range g.Image {
		frame := comp.compose(src, disposalAt(g, i))
		if opts.FilterBackground {
			if i == 0 {
				if bg, bgFound = detectBackground(frame); bgFound {
					masked = image.NewRGBA(bounds)
				}
			}
			if bgFound {
				maskBackground(masked, frame, bg)
				frame = masked
			}
		}
		rendered[i] = frameToASCII(frame, cols, rows, opts.Colored, ramp)
		delays[i] = delayAt(g, i)
		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	return &frames.Animation{
		Width:            cols,
		Height:           rows,
		Frames:           rendered,
		Delays:           delays,
		Colored:          opts.Colored,
		Complex:          opts.Complex,
		FilterBackground: opts.FilterBackground,
		CustomRamp:       opts.CustomRamp,
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
	imgW, imgH = max(1, imgW), max(1, imgH)
	switch {
	case opts.Width > 0 && opts.Height > 0:
		return opts.Width, opts.Height
	case opts.Width > 0:
		return opts.Width, max(1, opts.Width*imgH/(imgW*cellAspect))
	case opts.Height > 0:
		return max(1, opts.Height*imgW*cellAspect/imgH), opts.Height
	}

	maxW, maxH := opts.MaxWidth, opts.MaxHeight
	if maxW <= 0 {
		maxW = defaultMaxWidth
	}
	if maxH <= 0 {
		maxH = defaultMaxHeight
	}
	cols = maxW
	rows = max(1, cols*imgH/(imgW*cellAspect))
	if rows > maxH {
		rows = maxH
		cols = max(1, rows*imgW*cellAspect/imgH)
	}
	return cols, rows
}

func canvasBounds(g *gif.GIF) image.Rectangle {
	bounds := image.Rect(0, 0, g.Config.Width, g.Config.Height)
	if bounds.Empty() {
		bounds = g.Image[0].Bounds()
	}
	return bounds
}

// compositor flattens successive, possibly partial GIF frames onto a
// persistent canvas, honoring per-frame disposal methods, so that each
// composed result is a complete picture.
type compositor struct {
	canvas *image.RGBA
	backup *image.RGBA
	// disposal and rect describe the previously composed frame; its
	// disposal is applied lazily at the start of the next compose.
	disposal byte
	rect     image.Rectangle
}

func newCompositor(bounds image.Rectangle) *compositor {
	return &compositor{canvas: image.NewRGBA(bounds)}
}

// compose draws src onto the canvas and returns the full composited
// frame. The returned image is reused by the next compose call and
// must not be retained.
func (c *compositor) compose(src *image.Paletted, disposal byte) *image.RGBA {
	switch c.disposal {
	case gif.DisposalBackground:
		draw.Draw(c.canvas, c.rect, image.Transparent, image.Point{}, draw.Src)
	case gif.DisposalPrevious:
		c.canvas = c.backup
	}
	if disposal == gif.DisposalPrevious {
		c.backup = cloneRGBA(c.canvas)
	}
	draw.Draw(c.canvas, src.Bounds(), src, src.Bounds().Min, draw.Over)
	c.disposal, c.rect = disposal, src.Bounds()
	return c.canvas
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
