package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
)

func fixtureModel(t *testing.T) model {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"first", "second"} {
		anim := &frames.Animation{
			SourceName: name + ".gif",
			Width:      2,
			Height:     2,
			Frames:     []string{"ab\ncd", "ef\ngh"},
			Delays:     []time.Duration{50 * time.Millisecond, 50 * time.Millisecond},
		}
		if _, err := library.Save(dir, anim); err != nil {
			t.Fatalf("saving fixture: %v", err)
		}
	}

	gallery, err := newGallery(dir)
	if err != nil {
		t.Fatalf("newGallery: %v", err)
	}
	m := model{gallery: gallery}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(model)
}

// step applies a message and then keeps applying any messages produced
// by returned non-tick commands, mimicking the Bubble Tea runtime.
// Frame ticks and cursor blinks are self-rescheduling, so following
// them would loop forever.
func step(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	updated, cmd := m.Update(msg)
	m = updated.(model)
	if cmd == nil {
		return m
	}
	if produced := cmd(); produced != nil {
		switch produced.(type) {
		case frameTickMsg, cursor.BlinkMsg:
			return m
		}
		return step(t, m, produced)
	}
	return m
}

func TestGalleryEnterOpensPlayer(t *testing.T) {
	m := fixtureModel(t)
	if m.screen != screenGallery {
		t.Fatalf("initial screen = %v, want gallery", m.screen)
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenPlayer {
		t.Fatalf("screen after enter = %v, want player", m.screen)
	}
	if !strings.Contains(m.View(), "ab") {
		t.Error("player view does not contain first frame content")
	}
}

func TestPlayerNavigationAndBack(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})

	if got := m.player.entries[m.player.index].Name; got != "first" {
		t.Fatalf("initial entry = %q, want first", got)
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if got := m.player.entries[m.player.index].Name; got != "second" {
		t.Errorf("entry after right = %q, want second", got)
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if got := m.player.entries[m.player.index].Name; got != "first" {
		t.Errorf("entry after wraparound = %q, want first", got)
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != screenGallery {
		t.Errorf("screen after esc = %v, want gallery", m.screen)
	}
}

func TestPlayerPauseAndTick(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})

	m = step(t, m, frameTickMsg{gen: m.player.gen})
	if m.player.frame != 1 {
		t.Errorf("frame after tick = %d, want 1", m.player.frame)
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if !m.player.paused {
		t.Error("player not paused after space")
	}
	staleGen := m.player.gen - 1
	m = step(t, m, frameTickMsg{gen: staleGen})
	if m.player.frame != 1 {
		t.Errorf("stale tick advanced frame to %d", m.player.frame)
	}
}

func TestGalleryAddPromptTypesAndCancels(t *testing.T) {
	m := fixtureModel(t)

	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !m.gallery.typing {
		t.Fatal("gallery not in typing mode after 'a'")
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if m.screen != screenGallery || !m.gallery.typing {
		t.Fatal("'q' while typing quit the prompt instead of inserting text")
	}
	if got := m.gallery.picker.input.Value(); got != "q" {
		t.Errorf("input value = %q, want %q", got, "q")
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.gallery.typing {
		t.Error("gallery still typing after esc")
	}
}

func TestGalleryAddPromptExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("~/nope.gif")})
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.screen != screenRendering {
		t.Fatalf("screen after enter = %v, want rendering", m.screen)
	}
	if want := filepath.Join(home, "nope.gif"); m.render.gifPath != want {
		t.Errorf("render path = %q, want %q", m.render.gifPath, want)
	}
}

func TestGalleryTooSmallTerminalShowsNotice(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 1, Height: 1})
	m = updated.(model)
	if !strings.Contains(m.View(), "enlarge the window") {
		t.Error("undersized terminal does not show resize notice")
	}
}
