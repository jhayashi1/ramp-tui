package engine

import (
	"fmt"
	"image"
	"strings"

	xdraw "golang.org/x/image/draw"
)

const (
	ansiReset = "\x1b[0m"

	// Pixels with alpha below this render as blank space.
	alphaThreshold = 26
)

// frameToASCII scales the image down to a cols x rows grid and maps
// each resulting pixel to a character from the ramp, optionally
// prefixed with a 24-bit ANSI foreground color.
func frameToASCII(img *image.RGBA, cols, rows int, colored bool, ramp []rune) string {
	small := image.NewRGBA(image.Rect(0, 0, cols, rows))
	xdraw.CatmullRom.Scale(small, small.Bounds(), img, img.Bounds(), xdraw.Src, nil)

	var b strings.Builder
	for y := range rows {
		if y > 0 {
			b.WriteByte('\n')
		}
		lastColor := [3]uint8{}
		hasColor := false
		for x := range cols {
			c := small.RGBAAt(x, y)
			if c.A < alphaThreshold {
				b.WriteByte(' ')
				continue
			}
			lum := (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000
			ch := ramp[lum*len(ramp)/256]
			if colored {
				rgb := [3]uint8{c.R, c.G, c.B}
				if !hasColor || rgb != lastColor {
					fmt.Fprintf(&b, "\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
					lastColor = rgb
					hasColor = true
				}
			}
			b.WriteRune(ch)
		}
		if colored && hasColor {
			b.WriteString(ansiReset)
		}
	}
	return b.String()
}
