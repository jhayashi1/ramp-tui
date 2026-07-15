package engine

import (
	"image"
	"image/color"
)

const (
	// bgTolerance is the maximum per-channel distance for a pixel to
	// count as part of the detected background color.
	bgTolerance = 24

	// bgMinBorderShare is the fraction of border pixels that must match
	// one color for the border to count as a solid background.
	bgMinBorderShare = 0.9
)

// detectBackground reports the dominant border color of a composited
// frame when the border is nearly uniform and opaque. A GIF with such a
// border is treated as having a solid background that can be filtered
// out; a border that is transparent or varied yields ok == false.
func detectBackground(img *image.RGBA) (bg color.RGBA, ok bool) {
	counts := make(map[color.RGBA]int)
	total := 0
	forEachBorderPixel(img, func(c color.RGBA) {
		total++
		if c.A >= alphaThreshold {
			counts[c]++
		}
	})

	var mode color.RGBA
	best := 0
	for c, n := range counts {
		if n > best {
			mode, best = c, n
		}
	}
	if best == 0 {
		return color.RGBA{}, false
	}

	matched := 0
	forEachBorderPixel(img, func(c color.RGBA) {
		if c.A >= alphaThreshold && colorClose(c, mode) {
			matched++
		}
	})
	if float64(matched) < bgMinBorderShare*float64(total) {
		return color.RGBA{}, false
	}
	return mode, true
}

func forEachBorderPixel(img *image.RGBA, fn func(color.RGBA)) {
	b := img.Bounds()
	for x := b.Min.X; x < b.Max.X; x++ {
		fn(img.RGBAAt(x, b.Min.Y))
		if b.Dy() > 1 {
			fn(img.RGBAAt(x, b.Max.Y-1))
		}
	}
	for y := b.Min.Y + 1; y < b.Max.Y-1; y++ {
		fn(img.RGBAAt(b.Min.X, y))
		if b.Dx() > 1 {
			fn(img.RGBAAt(b.Max.X-1, y))
		}
	}
}

func colorClose(a, b color.RGBA) bool {
	return absDiff(a.R, b.R) <= bgTolerance &&
		absDiff(a.G, b.G) <= bgTolerance &&
		absDiff(a.B, b.B) <= bgTolerance
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// maskBackground copies src into dst, turning every pixel close to bg
// fully transparent so downstream scaling and rendering leave it blank.
// dst and src must have identical bounds.
func maskBackground(dst, src *image.RGBA, bg color.RGBA) {
	copy(dst.Pix, src.Pix)
	for i := 0; i+3 < len(dst.Pix); i += 4 {
		c := color.RGBA{R: dst.Pix[i], G: dst.Pix[i+1], B: dst.Pix[i+2], A: dst.Pix[i+3]}
		if c.A >= alphaThreshold && colorClose(c, bg) {
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = 0, 0, 0, 0
		}
	}
}
