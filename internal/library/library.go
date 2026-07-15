// Package library manages the on-disk store of rendered animations.
package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhayashi1/ascii-tui/internal/frames"
)

const fileExt = ".frames"

// Entry is a stored animation in the library directory.
type Entry struct {
	Name string
	Path string
}

// DefaultDir returns the platform cache location for the library,
// creating it if needed.
func DefaultDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving cache dir: %w", err)
	}
	dir := filepath.Join(base, "ascii-tui", "library")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating library dir: %w", err)
	}
	return dir, nil
}

// List returns all stored animations in dir, sorted by name.
func List(dir string) ([]Entry, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*"+fileExt))
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(matches))
	for _, path := range matches {
		entries = append(entries, Entry{
			Name: strings.TrimSuffix(filepath.Base(path), fileExt),
			Path: path,
		})
	}
	return entries, nil
}

// Save writes the animation into dir under a name derived from its
// source, appending a numeric suffix rather than overwriting an
// existing entry. It returns the file path.
func Save(dir string, anim *frames.Animation) (string, error) {
	name := sanitize(anim.SourceName)
	if name == "" {
		name = "animation"
	}
	path := filepath.Join(dir, name+fileExt)
	for n := 2; ; n++ {
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("checking library entry: %w", err)
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%d%s", name, n, fileExt))
	}
	if err := Write(path, anim); err != nil {
		return "", err
	}
	return path, nil
}

// Write encodes the animation to an arbitrary file path.
func Write(path string, anim *frames.Animation) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating frames file: %w", err)
	}
	if err := frames.Encode(f, anim); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing frames file: %w", err)
	}
	return nil
}

// Load reads a frames file from any path.
func Load(path string) (*frames.Animation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening frames file: %w", err)
	}
	defer f.Close()
	return frames.Decode(f)
}

func sanitize(name string) string {
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, name)
}
