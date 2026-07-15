// Package frames defines the portable file format for pre-rendered
// ASCII animations: a magic header followed by gzip-compressed gob data.
package frames

import (
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"time"
)

const version = 2

var magic = [8]byte{'A', 'S', 'C', 'I', 'I', 'T', 'U', 'I'}

// Animation is a fully rendered ASCII animation ready for playback.
type Animation struct {
	SourceName string
	Width      int
	Height     int
	Frames     []string
	Delays     []time.Duration

	// SourceGIF holds the original GIF bytes so the animation can be
	// re-rendered at a different size; nil in version-1 files, which
	// play back fine but cannot be resized.
	SourceGIF []byte
	// Render options used to produce Frames, so re-renders match.
	Colored          bool
	Complex          bool
	FilterBackground bool
	CustomRamp       string
}

// Encode writes the animation to w in the frames file format.
func Encode(w io.Writer, anim *Animation) error {
	if _, err := w.Write(magic[:]); err != nil {
		return fmt.Errorf("writing frames header: %w", err)
	}
	if _, err := w.Write([]byte{version}); err != nil {
		return fmt.Errorf("writing frames header: %w", err)
	}
	zw := gzip.NewWriter(w)
	if err := gob.NewEncoder(zw).Encode(anim); err != nil {
		return fmt.Errorf("encoding animation: %w", err)
	}
	return zw.Close()
}

// Decode reads an animation from r, validating the header first.
func Decode(r io.Reader) (*Animation, error) {
	var header [9]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, fmt.Errorf("reading frames header: %w", err)
	}
	if [8]byte(header[:8]) != magic {
		return nil, errors.New("not an ascii-tui frames file")
	}
	if header[8] < 1 || header[8] > version {
		return nil, fmt.Errorf("unsupported frames file version %d", header[8])
	}
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("decompressing animation: %w", err)
	}
	defer zr.Close()
	var anim Animation
	if err := gob.NewDecoder(zr).Decode(&anim); err != nil {
		return nil, fmt.Errorf("decoding animation: %w", err)
	}
	if len(anim.Frames) == 0 {
		return nil, errors.New("frames file contains no frames")
	}
	if len(anim.Delays) != len(anim.Frames) {
		return nil, fmt.Errorf("frames file is corrupt: %d frames but %d delays",
			len(anim.Frames), len(anim.Delays))
	}
	return &anim, nil
}
