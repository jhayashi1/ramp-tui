package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReturnsNotExistWhenFileAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	if _, err := load(path); !os.IsNotExist(err) {
		t.Errorf("err = %v, want a not-exist error", err)
	}
}

func TestLoadRoundTripsSampleFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	sample := `
[playback]
speed = 2.0

[render]
filter_background = true
complex = true

[theme]
accent = "99"
`
	if err := os.WriteFile(path, []byte(sample), 0o644); err != nil {
		t.Fatalf("writing sample config: %v", err)
	}

	cfg, err := load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Playback.Speed != 2.0 {
		t.Errorf("speed = %v, want 2.0", cfg.Playback.Speed)
	}
	if !cfg.Render.FilterBackground || !cfg.Render.Complex {
		t.Errorf("render = %+v, want both true", cfg.Render)
	}
	if cfg.Theme.Accent != "99" {
		t.Errorf("accent = %q, want 99", cfg.Theme.Accent)
	}

	def := Defaults()
	if cfg.Theme.Border != def.Theme.Border {
		t.Errorf("border = %q, want default %q (unset fields keep defaults)", cfg.Theme.Border, def.Theme.Border)
	}
	if cfg.Theme.Error != def.Theme.Error {
		t.Errorf("error = %q, want default %q (unset fields keep defaults)", cfg.Theme.Error, def.Theme.Error)
	}
	if cfg.Theme.Bg != def.Theme.Bg || cfg.Theme.SelectionBg != def.Theme.SelectionBg || cfg.Theme.ChipText != def.Theme.ChipText || cfg.Theme.AccentAlt != def.Theme.AccentAlt {
		t.Errorf("new theme fields = %q/%q/%q/%q, want defaults %q/%q/%q/%q (old configs keep working)",
			cfg.Theme.Bg, cfg.Theme.SelectionBg, cfg.Theme.ChipText, cfg.Theme.AccentAlt,
			def.Theme.Bg, def.Theme.SelectionBg, def.Theme.ChipText, def.Theme.AccentAlt)
	}
}

func TestLoadRejectsBadTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("not = [valid toml"), 0o644); err != nil {
		t.Fatalf("writing bad config: %v", err)
	}
	if _, err := load(path); err == nil {
		t.Fatal("load succeeded on invalid TOML, want an error")
	}
}
