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

const version = 1

var magic = [8]byte{'A', 'S', 'C', 'I', 'I', 'T', 'U', 'I'}

// Animation is a fully rendered ASCII animation ready for playback.
type Animation struct {
	SourceName string
	Width      int
	Height     int
	Frames     []string
	Delays     []time.Duration
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
	if header[8] != version {
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
	return &anim, nil
}
