package engine

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"strings"
	"testing"
)

func solidRGBA(rect image.Rectangle, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(rect)
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestDetectBackgroundUniformBorder(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	img := solidRGBA(image.Rect(0, 0, 10, 10), white)
	img.SetRGBA(5, 5, color.RGBA{255, 0, 0, 255})

	bg, ok := detectBackground(img)
	if !ok {
		t.Fatal("detectBackground = false, want solid white border detected")
	}
	if bg != white {
		t.Errorf("bg = %v, want %v", bg, white)
	}
}

func TestDetectBackgroundToleratesSlightNoise(t *testing.T) {
	img := solidRGBA(image.Rect(0, 0, 10, 10), color.RGBA{250, 250, 250, 255})
	img.SetRGBA(0, 0, color.RGBA{240, 245, 255, 255})

	if _, ok := detectBackground(img); !ok {
		t.Error("detectBackground = false, want near-uniform border detected")
	}
}

func TestDetectBackgroundRejectsVariedBorder(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	red := color.RGBA{255, 0, 0, 255}
	blue := color.RGBA{0, 0, 255, 255}
	for y := range 10 {
		for x := range 10 {
			if (x+y)%2 == 0 {
				img.SetRGBA(x, y, red)
			} else {
				img.SetRGBA(x, y, blue)
			}
		}
	}

	if _, ok := detectBackground(img); ok {
		t.Error("detectBackground = true, want checkerboard border rejected")
	}
}

func TestDetectBackgroundRejectsTransparentBorder(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.SetRGBA(5, 5, color.RGBA{255, 0, 0, 255})

	if _, ok := detectBackground(img); ok {
		t.Error("detectBackground = true, want transparent border rejected")
	}
}

func TestMaskBackgroundBlanksMatchesOnly(t *testing.T) {
	white := color.RGBA{255, 255, 255, 255}
	red := color.RGBA{200, 0, 0, 255}
	src := solidRGBA(image.Rect(0, 0, 4, 4), white)
	src.SetRGBA(2, 2, red)

	dst := image.NewRGBA(src.Bounds())
	maskBackground(dst, src, white)

	if got := dst.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("background pixel = %v, want fully transparent", got)
	}
	if got := dst.RGBAAt(2, 2); got != red {
		t.Errorf("subject pixel = %v, want %v preserved", got, red)
	}
	if got := src.RGBAAt(0, 0); got != white {
		t.Errorf("src pixel = %v, want untouched", got)
	}
}

// solidBGGif encodes a white GIF with a red square in the center.
func solidBGGif(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{
		color.RGBA{255, 255, 255, 255},
		color.RGBA{200, 0, 0, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, 40, 40), pal)
	for y := 14; y < 26; y++ {
		for x := 14; x < 26; x++ {
			img.SetColorIndex(x, y, 1)
		}
	}
	var buf bytes.Buffer
	err := gif.EncodeAll(&buf, &gif.GIF{Image: []*image.Paletted{img}, Delay: []int{10}})
	if err != nil {
		t.Fatalf("encoding gif: %v", err)
	}
	return buf.Bytes()
}

func TestRenderFilterBackgroundBlanksSolidBackground(t *testing.T) {
	data := solidBGGif(t)

	anim, err := Render(bytes.NewReader(data), Options{Width: 20, Height: 20, FilterBackground: true}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	lines := strings.Split(anim.Frames[0], "\n")
	if got := strings.TrimSpace(lines[0]); got != "" {
		t.Errorf("top row = %q, want blank background", got)
	}
	if got := strings.TrimSpace(lines[len(lines)/2]); got == "" {
		t.Error("center row is blank, want subject preserved")
	}
	if !anim.FilterBackground {
		t.Error("anim.FilterBackground = false, want option recorded for re-renders")
	}
}

func TestRenderWithoutFilterKeepsSolidBackground(t *testing.T) {
	data := solidBGGif(t)

	anim, err := Render(bytes.NewReader(data), Options{Width: 20, Height: 20}, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if got := strings.TrimSpace(strings.Split(anim.Frames[0], "\n")[0]); got == "" {
		t.Error("top row is blank without the filter, want background characters")
	}
}
