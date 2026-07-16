// Package config loads user-configurable defaults (theme colors,
// playback speed, render options) from a TOML file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config is the flat, TOML-serializable shape of the user's settings.
type Config struct {
	Playback Playback `toml:"playback"`
	Render   Render   `toml:"render"`
	Theme    Theme    `toml:"theme"`
}

// Playback holds player defaults.
type Playback struct {
	Speed float64 `toml:"speed"`
}

// Render holds defaults for gifs rendered through the gallery's "add"
// flow or the `ascii-tui <gif>` shortcut.
type Render struct {
	FilterBackground bool `toml:"filter_background"`
	Complex          bool `toml:"complex"`
}

// Theme holds named colors; any value lipgloss.Color accepts (an ANSI
// index, hex code, or name) is valid here.
type Theme struct {
	Accent string `toml:"accent"`
	Border string `toml:"border"`
	Text   string `toml:"text"`
	Dim    string `toml:"dim"`
	Error  string `toml:"error"`
}

// Defaults returns the built-in configuration, used whenever no config
// file is present or it fails to parse.
func Defaults() Config {
	return Config{
		Playback: Playback{Speed: 1},
		Theme: Theme{
			Accent: "212",
			Border: "240",
			Text:   "252",
			Dim:    "241",
			Error:  "203",
		},
	}
}

// Path returns the config file's location: os.UserConfigDir()/ascii-tui/config.toml.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolving config dir: %w", err)
	}
	return filepath.Join(dir, "ascii-tui", "config.toml"), nil
}

// Load reads the user's config file, falling back to Defaults() when
// it's absent (silently) or fails to parse (with a warning on stderr).
// Either way, Load never returns an error: a broken config should not
// stop the program from starting.
func Load() Config {
	path, err := Path()
	if err != nil {
		return Defaults()
	}
	cfg, err := load(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "ascii-tui: %v; using defaults\n", err)
		}
		return Defaults()
	}
	return cfg
}

// load reads and parses the TOML file at path. Fields absent from the
// file keep their default values, so a partial file only overrides
// what it explicitly sets.
func load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg := Defaults()
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}
