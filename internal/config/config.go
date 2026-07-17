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
	Keys     Keys     `toml:"keys"`
}

// Playback holds player defaults.
type Playback struct {
	Speed float64 `toml:"speed"`
}

// Keys maps each player action to the keys that trigger it. Values are
// Bubble Tea key names ("n", ",", "left", "ctrl+n") with one exception:
// the space bar is spelled "space" so hand-edited files don't need a
// bare " ". An empty list falls back to the action's default binding.
type Keys struct {
	Pause       []string `toml:"pause"`
	SeekBack    []string `toml:"seek_back"`
	SeekForward []string `toml:"seek_forward"`
	StepBack    []string `toml:"step_back"`
	StepForward []string `toml:"step_forward"`
	SpeedUp     []string `toml:"speed_up"`
	SpeedDown   []string `toml:"speed_down"`
	Next        []string `toml:"next"`
	Prev        []string `toml:"prev"`
	Filter      []string `toml:"filter"`
}

// DefaultKeys returns the built-in player bindings, used both as the
// Defaults() value and as the per-action fallback when a config entry
// is empty or reset.
func DefaultKeys() Keys {
	return Keys{
		Pause:       []string{"space"},
		SeekBack:    []string{"left", "h"},
		SeekForward: []string{"right", "l"},
		StepBack:    []string{","},
		StepForward: []string{"."},
		SpeedUp:     []string{"+", "="},
		SpeedDown:   []string{"-"},
		Next:        []string{"n"},
		Prev:        []string{"p"},
		Filter:      []string{"f"},
	}
}

// Render holds defaults for gifs rendered through the gallery's "add"
// flow or the `ascii-tui <gif>` shortcut.
type Render struct {
	FilterBackground bool `toml:"filter_background"`
	Complex          bool `toml:"complex"`
}

// Theme holds named colors; any value lipgloss.Color accepts (an ANSI
// index, hex code, or name) is valid here. Bg, SelectionBg, and
// ChipText additionally feed raw background escapes, which support ANSI
// indexes and hex codes.
type Theme struct {
	Accent      string `toml:"accent"`
	AccentAlt   string `toml:"accent_alt"`
	Border      string `toml:"border"`
	Text        string `toml:"text"`
	Dim         string `toml:"dim"`
	Error       string `toml:"error"`
	Bg          string `toml:"bg"`
	SelectionBg string `toml:"selection_bg"`
	ChipText    string `toml:"chip_text"`
}

// Defaults returns the built-in configuration, used whenever no config
// file is present or it fails to parse.
func Defaults() Config {
	return Config{
		Playback: Playback{Speed: 1},
		Keys:     DefaultKeys(),
		Theme: Theme{
			Accent:      "212",
			AccentAlt:   "179",
			Border:      "240",
			Text:        "252",
			Dim:         "243",
			Error:       "203",
			Bg:          "234",
			SelectionBg: "237",
			ChipText:    "234",
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
	cfg.Keys.FillEmpty()
	return cfg, nil
}

// Save writes cfg to the config file, creating its directory if needed.
// It is used by the TUI's keybinds screen to persist rebinds.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return save(path, cfg)
}

func save(path string, cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// FillEmpty restores the default binding for any action whose key list
// is empty, so an explicit `pause = []` in the file (or a zero-value
// Keys) can never leave an action unreachable.
func (k *Keys) FillEmpty() {
	def := DefaultKeys()
	for _, pair := range []struct{ dst, src *[]string }{
		{&k.Pause, &def.Pause},
		{&k.SeekBack, &def.SeekBack},
		{&k.SeekForward, &def.SeekForward},
		{&k.StepBack, &def.StepBack},
		{&k.StepForward, &def.StepForward},
		{&k.SpeedUp, &def.SpeedUp},
		{&k.SpeedDown, &def.SpeedDown},
		{&k.Next, &def.Next},
		{&k.Prev, &def.Prev},
		{&k.Filter, &def.Filter},
	} {
		if len(*pair.dst) == 0 {
			*pair.dst = *pair.src
		}
	}
}
