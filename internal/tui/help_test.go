package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpOverlayTogglesAndSwallowsClosingKey(t *testing.T) {
	m := fixtureModel(t)
	m = step(t, m, keyRune('?'))
	if !m.helpVisible {
		t.Fatal("help overlay did not open on '?'")
	}
	if !strings.Contains(m.View(), "play") {
		t.Errorf("help view = %q, want it to mention the gallery's play binding", m.View())
	}

	// Any key closes it, and that same key must not also reach the
	// gallery underneath (e.g. it should not open the add-gif prompt).
	m = step(t, m, keyRune('a'))
	if m.helpVisible {
		t.Error("help overlay still open after a keypress")
	}
	if m.gallery.mode != inputNone {
		t.Error("the closing keypress leaked through to the gallery")
	}
}

func TestHelpOverlayOpensFromPlayer(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('?'))
	if !m.helpVisible {
		t.Fatal("help overlay did not open from the player screen")
	}
	if got := m.View(); !strings.Contains(got, "scrub") {
		t.Errorf("help view = %q, want the player's scrub binding description", got)
	}
}

func TestHelpOverlayKeepsTickingUnderneath(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('?'))

	updated, _ := m.Update(frameTickMsg{gen: m.player.gen})
	m = updated.(model)
	if m.player.frame != 1 {
		t.Errorf("frame = %d, want 1 (playback should advance while help is open)", m.player.frame)
	}
}

func TestFormatFullHelpSkipsUnboundEntries(t *testing.T) {
	got := formatFullHelp(newGalleryKeyMap(), defaultStyles())
	if !strings.Contains(got, "play") || !strings.Contains(got, "delete") {
		t.Errorf("formatted help = %q, want it to mention play and delete", got)
	}
}
