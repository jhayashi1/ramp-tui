package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ascii-tui/internal/engine"
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
func step(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	updated, cmd := m.Update(msg)
	m = updated.(model)
	if cmd == nil {
		return m
	}
	if produced := cmd(); produced != nil {
		switch produced.(type) {
		case frameTickMsg, refitTickMsg, cursor.BlinkMsg:
			return m
		}
		return step(t, m, produced)
	}
	return m
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
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

// TestGalleryTooSmallTerminalShowsNotice covers legacy entries without
// an embedded source GIF: they cannot refit, so the player can only
// ask for a bigger window.
func TestGalleryTooSmallTerminalShowsNotice(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 1, Height: 1})
	m = updated.(model)
	if !strings.Contains(m.View(), "enlarge the window") {
		t.Error("undersized terminal does not show resize notice")
	}
}

func TestGalleryRenameFlow(t *testing.T) {
	m := step(t, fixtureModel(t), keyRune('r'))
	if m.gallery.mode != inputRename {
		t.Fatalf("mode after r = %v, want inputRename", m.gallery.mode)
	}
	if got := m.gallery.input.Value(); got != "first" {
		t.Fatalf("prefilled input = %q, want first", got)
	}

	m.gallery.input.SetValue("zebra")
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.gallery.mode != inputNone {
		t.Errorf("mode after enter = %v, want inputNone", m.gallery.mode)
	}
	names := make([]string, 0, 2)
	for _, e := range m.gallery.entries() {
		names = append(names, e.Name)
	}
	if len(names) != 2 || names[0] != "second" || names[1] != "zebra" {
		t.Errorf("entries after rename = %v, want [second zebra]", names)
	}
	if got := m.gallery.list.SelectedItem().(entryItem).Name; got != "zebra" {
		t.Errorf("selection after rename = %q, want zebra", got)
	}
}

func TestGalleryRenameEscCancels(t *testing.T) {
	m := step(t, fixtureModel(t), keyRune('r'))
	m.gallery.input.SetValue("zebra")
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.gallery.mode != inputNone {
		t.Errorf("mode after esc = %v, want inputNone", m.gallery.mode)
	}
	if got := m.gallery.entries()[0].Name; got != "first" {
		t.Errorf("first entry = %q, want first (unchanged)", got)
	}
}

func TestGalleryRenameCollisionShowsStatus(t *testing.T) {
	m := step(t, fixtureModel(t), keyRune('r'))
	m.gallery.input.SetValue("second")
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m.gallery.status, "rename failed") {
		t.Errorf("status = %q, want rename failure", m.gallery.status)
	}
	names := make([]string, 0, 2)
	for _, e := range m.gallery.entries() {
		names = append(names, e.Name)
	}
	if len(names) != 2 || names[0] != "first" || names[1] != "second" {
		t.Errorf("entries after failed rename = %v, want [first second]", names)
	}
}

// tinyGIF encodes a 2-frame 8x4 checkerboard GIF in memory.
func tinyGIF(t *testing.T) []byte {
	t.Helper()
	palette := []color.Color{color.Black, color.White}
	g := &gif.GIF{}
	for i := 0; i < 2; i++ {
		img := image.NewPaletted(image.Rect(0, 0, 8, 4), palette)
		for y := 0; y < 4; y++ {
			for x := 0; x < 8; x++ {
				img.SetColorIndex(x, y, uint8((x+y+i)%2))
			}
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 5)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatalf("encoding gif: %v", err)
	}
	return buf.Bytes()
}

// fixtureModelResizable builds a model whose single library entry
// embeds its source GIF and can therefore refit on resize.
func fixtureModelResizable(t *testing.T) model {
	t.Helper()
	dir := t.TempDir()
	data := tinyGIF(t)
	anim, err := engine.Render(bytes.NewReader(data),
		engine.Options{MaxWidth: 20, MaxHeight: 10}, nil)
	if err != nil {
		t.Fatalf("rendering fixture gif: %v", err)
	}
	anim.SourceName = "tiny.gif"
	anim.SourceGIF = data
	if _, err := library.Save(dir, anim); err != nil {
		t.Fatalf("saving fixture: %v", err)
	}

	gallery, err := newGallery(dir)
	if err != nil {
		t.Fatalf("newGallery: %v", err)
	}
	m := model{gallery: gallery}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(model)
}

func TestPlayerRefitsOnResize(t *testing.T) {
	m := step(t, fixtureModelResizable(t), tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenPlayer {
		t.Fatalf("screen = %v, want player", m.screen)
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m = updated.(model)

	player, cmd := m.player.update(refitTickMsg{gen: m.player.refitGen})
	m.player = player
	if !m.player.refitting {
		t.Fatal("player not refitting after debounce tick")
	}
	if cmd == nil {
		t.Fatal("debounce tick produced no render command")
	}
	player, _ = m.player.update(cmd())
	m.player = player

	if m.player.refitting {
		t.Error("player still refitting after render completed")
	}
	vw, vh := 40, 11
	w, h := m.player.anim.Width, m.player.anim.Height
	if w > vw || h > vh {
		t.Errorf("refitted to %dx%d, exceeds viewport %dx%d", w, h, vw, vh)
	}
	if w != vw && h != vh {
		t.Errorf("refitted to %dx%d, want one axis saturating %dx%d", w, h, vw, vh)
	}
	if m.player.anim.SourceGIF == nil {
		t.Error("SourceGIF not carried onto the refitted animation")
	}
	if m.player.needsRefit() {
		t.Error("player still wants a refit at the size it just fitted")
	}
}

func TestPlayerDropsStaleRefitResult(t *testing.T) {
	m := step(t, fixtureModelResizable(t), tea.KeyMsg{Type: tea.KeyEnter})
	before := m.player.anim

	stale := refitDoneMsg{
		gen:  m.player.refitGen - 1,
		anim: &frames.Animation{Width: 1, Height: 1, Frames: []string{"!"}, Delays: []time.Duration{time.Millisecond}},
	}
	player, _ := m.player.update(stale)
	m.player = player
	if m.player.anim != before {
		t.Error("stale refit result replaced the animation")
	}
}

func TestPlayerTooSmallResizableShowsFitting(t *testing.T) {
	m := step(t, fixtureModelResizable(t), tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 10, Height: 4})
	m = updated.(model)
	if !strings.Contains(m.View(), "fitting to") {
		t.Error("undersized resizable animation does not show fitting notice")
	}
}

func TestPlayerCentersFrame(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	lines := strings.Split(m.View(), "\n")

	if strings.Contains(lines[0], "ab") {
		t.Error("frame starts on the first line, want vertical centering")
	}
	found := false
	for _, line := range lines {
		if strings.Contains(line, "ab") {
			found = true
			if !strings.HasPrefix(line, " ") {
				t.Errorf("frame line %q starts at column 0, want horizontal centering", line)
			}
		}
	}
	if !found {
		t.Fatal("player view does not contain the frame")
	}
}
