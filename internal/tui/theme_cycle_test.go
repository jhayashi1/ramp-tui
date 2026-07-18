package tui

import (
	"strings"
	"testing"

	"github.com/jhayashi1/ramp-tui/internal/config"
)

// TestFooterStatusAutoClears checks that the timer fired after a footer
// message reverts the footer to normal, and that a stale timer (from an
// earlier message) never wipes a newer one.
func TestFooterStatusAutoClears(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := fixtureModel(t)

	m = step(t, m, keyRune('t'))
	if m.gallery.status == "" {
		t.Fatal("theme switch set no footer message")
	}
	staleGen := m.gallery.statusGen

	// The matching clear timer blanks the footer.
	updated, _ := m.Update(clearGalleryStatusMsg{gen: staleGen})
	if got := updated.(model).gallery.status; got != "" {
		t.Errorf("status after matching clear = %q, want empty", got)
	}

	// A second message, then the first message's (stale) timer must not
	// clear it.
	m = step(t, m, keyRune('t'))
	current := m.gallery.status
	updated, _ = m.Update(clearGalleryStatusMsg{gen: staleGen})
	if got := updated.(model).gallery.status; got != current {
		t.Errorf("stale timer cleared newer message: got %q, want %q", got, current)
	}
}

// TestGalleryThemeKeyCyclesAndPersists checks that "t" on the home screen
// advances to the next preset, repaints the gallery's sub-components, and
// writes the choice to disk so it survives a restart.
func TestGalleryThemeKeyCyclesAndPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := fixtureModel(t)

	before := m.st.theme.Accent
	next := themePresets[(m.themeIndex+1)%len(themePresets)]

	m = step(t, m, keyRune('t'))

	if m.st.theme.Accent == before {
		t.Fatalf("accent unchanged after t: %q", m.st.theme.Accent)
	}
	if m.st.theme != next.theme {
		t.Errorf("app theme = %+v, want %+v", m.st.theme, next.theme)
	}
	// The long-lived gallery sub-components must adopt the new styles too.
	if m.gallery.st.theme != next.theme {
		t.Error("gallery styles not updated after theme change")
	}
	if m.gallery.picker.st.theme != next.theme {
		t.Error("path picker styles not updated after theme change")
	}
	if m.gallery.preview.st.theme != next.theme {
		t.Error("preview styles not updated after theme change")
	}
	if !strings.Contains(m.gallery.status, "theme: "+next.name) {
		t.Errorf("status = %q, want theme name %q", m.gallery.status, next.name)
	}

	saved := config.Load()
	if saved.Theme.Name != next.name {
		t.Errorf("saved theme name = %q, want %q", saved.Theme.Name, next.name)
	}
	if got := themeFromConfig(saved.Theme); got != next.theme {
		t.Errorf("saved theme colors = %+v, want %+v", got, next.theme)
	}
}

// TestGalleryThemeKeyWrapsAround presses "t" once per preset and expects
// to land back on the palette it started from.
func TestGalleryThemeKeyWrapsAround(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := fixtureModel(t)

	// The fixture starts on a zero (non-preset) theme, so land on a preset
	// first, then a full lap should return to it.
	m = step(t, m, keyRune('t'))
	start := m.st.theme

	for range themePresets {
		m = step(t, m, keyRune('t'))
	}
	if m.st.theme != start {
		t.Errorf("theme after full cycle = %+v, want start %+v", m.st.theme, start)
	}
}
