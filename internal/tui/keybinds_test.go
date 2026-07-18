package tui

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ramp-tui/internal/config"
)

// fixtureKeybinds opens the keybinds screen from the gallery, isolating
// the config file writes that saves perform under a temp dir.
func fixtureKeybinds(t *testing.T) model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	m := step(t, fixtureModel(t), keyRune('k'))
	if m.screen != screenKeybinds {
		t.Fatalf("screen after k = %v, want keybinds", m.screen)
	}
	return m
}

func TestKeybindsRebindUpdatesConfigAndPlayer(t *testing.T) {
	m := fixtureKeybinds(t)

	// Cursor starts on "pause"; enter arms capture, then x replaces space.
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.keybinds.capture != captureReplace {
		t.Fatal("enter did not arm rebind capture")
	}
	m = step(t, m, keyRune('x'))
	if got := m.cfg.Keys.Pause; len(got) != 1 || got[0] != "x" {
		t.Fatalf("cfg pause keys = %v, want [x]", got)
	}

	// The rebind must be persisted, and reload with defaults elsewhere.
	saved := config.Load()
	if got := saved.Keys.Pause; len(got) != 1 || got[0] != "x" {
		t.Errorf("saved pause keys = %v, want [x]", got)
	}
	if got := saved.Keys.Next; len(got) != 1 || got[0] != ">" {
		t.Errorf("saved next keys = %v, want default [>]", got)
	}

	// A player launched after the rebind pauses on x, not space.
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenGallery {
		t.Fatalf("screen after esc = %v, want gallery", m.screen)
	}
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if m.player.paused {
		t.Error("space still pauses after rebinding pause to x")
	}
	m = step(t, m, keyRune('x'))
	if !m.player.paused {
		t.Error("x does not pause after rebind")
	}
}

func TestKeybindsAddKeyKeepsExisting(t *testing.T) {
	m := fixtureKeybinds(t)
	m = step(t, m, keyRune('a'))
	if m.keybinds.capture != captureAppend {
		t.Fatal("a did not arm add-key capture")
	}
	m = step(t, m, keyRune('x'))
	if got := m.cfg.Keys.Pause; len(got) != 2 || got[0] != "space" || got[1] != "x" {
		t.Errorf("cfg pause keys = %v, want [space x]", got)
	}
}

func TestKeybindsRejectsReservedKey(t *testing.T) {
	m := fixtureKeybinds(t)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('?'))
	if m.screen != screenKeybinds {
		t.Fatal("'?' during capture opened help or left the screen")
	}
	if !strings.Contains(m.keybinds.status, "reserved") {
		t.Errorf("status = %q, want reserved-key rejection", m.keybinds.status)
	}
	if got := m.cfg.Keys.Pause; len(got) != 0 {
		t.Errorf("cfg pause keys = %v, want unchanged (empty until first edit)", got)
	}
	if got := m.keybinds.keys.Pause; len(got) != 1 || got[0] != "space" {
		t.Errorf("menu pause keys = %v, want [space]", got)
	}
}

func TestKeybindsRejectsTakenKey(t *testing.T) {
	m := fixtureKeybinds(t)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('>')) // taken by "next"
	if !strings.Contains(m.keybinds.status, "taken by next") {
		t.Errorf("status = %q, want conflict with next", m.keybinds.status)
	}
	if got := m.keybinds.keys.Pause; len(got) != 1 || got[0] != "space" {
		t.Errorf("menu pause keys = %v, want [space]", got)
	}
}

func TestKeybindsCaptureEscCancels(t *testing.T) {
	m := fixtureKeybinds(t)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenKeybinds {
		t.Fatal("esc during capture left the screen instead of cancelling")
	}
	if m.keybinds.capture != captureNone {
		t.Error("capture still armed after esc")
	}
}

func TestKeybindsResetAllRestoresDefaults(t *testing.T) {
	m := fixtureKeybinds(t)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('x'))
	m = step(t, m, keyRune('D'))
	def := config.DefaultKeys()
	if got := m.cfg.Keys.Pause; len(got) != 1 || got[0] != def.Pause[0] {
		t.Errorf("pause keys after reset all = %v, want %v", got, def.Pause)
	}
}

func TestKeybindsResetSelectedRestoresDefault(t *testing.T) {
	m := fixtureKeybinds(t)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('x'))
	m = step(t, m, keyRune('d'))
	if got := m.keybinds.keys.Pause; len(got) != 1 || got[0] != "space" {
		t.Errorf("pause keys after reset = %v, want [space]", got)
	}
}

func TestKeybindsViewListsActionsAndKeys(t *testing.T) {
	m := fixtureKeybinds(t)
	view := m.View()
	for _, want := range []string{"PLAYER KEYBINDS", "pause", "space", "scrub back", "←", "KEYBINDS"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

// lipgloss.Place centers each line on its own, so rows only left-align
// as a block when every row shares the same width; a row rendered at
// its natural width drifts toward the middle.
func TestKeybindsViewRowsAlign(t *testing.T) {
	m := fixtureKeybinds(t)
	mm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	ansiRe := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	lines := strings.Split(ansiRe.ReplaceAllString(mm.(model).View(), ""), "\n")
	want := -1
	for _, action := range keybindActions {
		col := -1
		for _, line := range lines {
			if i := strings.Index(line, action.label); i >= 0 {
				col = utf8.RuneCountInString(line[:i])
				break
			}
		}
		if col < 0 {
			t.Fatalf("view missing action %q", action.label)
		}
		if want == -1 {
			want = col
		} else if col != want {
			t.Errorf("%q starts at column %d, want %d", action.label, col, want)
		}
	}
}

func TestKeybindsNavigationWraps(t *testing.T) {
	m := fixtureKeybinds(t)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyUp})
	if m.keybinds.cursor != len(keybindActions)-1 {
		t.Errorf("cursor after up from top = %d, want %d", m.keybinds.cursor, len(keybindActions)-1)
	}
	m = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.keybinds.cursor != 0 {
		t.Errorf("cursor after wrapping down = %d, want 0", m.keybinds.cursor)
	}
}
