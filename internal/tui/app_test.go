package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"path/filepath"
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

	gallery, err := newGallery(dir, defaultStyles())
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
	return runCmd(t, m, cmd)
}

// runCmd executes cmd and feeds its result back through step, unwrapping
// tea.Batch the same way the real runtime does: each sub-command runs
// and its result is dispatched independently, rather than the raw
// tea.BatchMsg slice ever reaching a model's Update.
func runCmd(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		return m
	}
	produced := cmd()
	if produced == nil {
		return m
	}
	if batch, ok := produced.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = runCmd(t, m, c)
		}
		return m
	}
	switch produced.(type) {
	case frameTickMsg, refitTickMsg, cursor.BlinkMsg:
		return m
	}
	return step(t, m, produced)
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

	m = step(t, m, keyRune('n'))
	if got := m.player.entries[m.player.index].Name; got != "second" {
		t.Errorf("entry after n = %q, want second", got)
	}

	m = step(t, m, keyRune('n'))
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
	if m.gallery.mode != inputAddGIF {
		t.Fatal("gallery not in add-gif mode after 'a'")
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if m.screen != screenGallery || m.gallery.mode != inputAddGIF {
		t.Fatal("'q' while typing quit the prompt instead of inserting text")
	}
	if got := m.gallery.picker.input.Value(); got != "q" {
		t.Errorf("input value = %q, want %q", got, "q")
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.gallery.mode != inputNone {
		t.Error("gallery still typing after esc")
	}
}

func TestGalleryAddPromptExpandsTilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("~/nope.gif")})

	// Chase the Enter keypress's command by hand for exactly one hop
	// (gallery's startRenderMsg cmd -> the startRenderMsg case, which
	// expands the path and switches screens synchronously) without
	// draining further: the render screen's own command actually reads
	// the gif from disk, and "nope.gif" doesn't exist, which would bounce
	// back to the gallery before this test gets to make its assertion.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd != nil {
		if produced := cmd(); produced != nil {
			updated, _ = m.Update(produced)
			m = updated.(model)
		}
	}

	if m.screen != screenRendering {
		t.Fatalf("screen after enter = %v, want rendering", m.screen)
	}
	if want := filepath.Join(home, "nope.gif"); m.render.gifPath != want {
		t.Errorf("render path = %q, want %q", m.render.gifPath, want)
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

	gallery, err := newGallery(dir, defaultStyles())
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
	vw, vh := 40, 10
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

// TestPlayerRefitPersistsToLibrary guards against a regression where a
// resize-triggered refit only updated the in-memory animation: every
// future play of the same entry would refit again from the stale
// on-disk size, showing "fitting to..." every single time instead of
// just once.
func TestPlayerRefitPersistsToLibrary(t *testing.T) {
	m := step(t, fixtureModelResizable(t), tea.KeyMsg{Type: tea.KeyEnter})
	path := m.player.entries[m.player.index].Path

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	m = updated.(model)

	player, cmd := m.player.update(refitTickMsg{gen: m.player.refitGen})
	m.player = player
	if cmd == nil {
		t.Fatal("debounce tick produced no render command")
	}
	player, _ = m.player.update(cmd())
	m.player = player

	saved, err := library.Load(path)
	if err != nil {
		t.Fatalf("loading saved entry: %v", err)
	}
	if saved.Width != m.player.anim.Width || saved.Height != m.player.anim.Height {
		t.Errorf("saved dims = %dx%d, want %dx%d (refit not persisted to disk)",
			saved.Width, saved.Height, m.player.anim.Width, m.player.anim.Height)
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

// solidBGGIF encodes a single-frame GIF with a white background and a
// red square in the center.
func solidBGGIF(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{
		color.RGBA{255, 255, 255, 255},
		color.RGBA{200, 0, 0, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, 40, 40), pal)
	for y := 14; y < 26; y++ {
		for x := 14; x < 26; x++ {
			img.SetColorIndex(x, y, 1)
		}
	}
	var buf bytes.Buffer
	err := gif.EncodeAll(&buf, &gif.GIF{Image: []*image.Paletted{img}, Delay: []int{10}})
	if err != nil {
		t.Fatalf("encoding gif: %v", err)
	}
	return buf.Bytes()
}

func TestPlayerToggleFilterBackground(t *testing.T) {
	dir := t.TempDir()
	data := solidBGGIF(t)
	anim, err := engine.Render(bytes.NewReader(data),
		engine.Options{MaxWidth: 20, MaxHeight: 10}, nil)
	if err != nil {
		t.Fatalf("rendering fixture gif: %v", err)
	}
	anim.SourceName = "solid.gif"
	anim.SourceGIF = data
	path, err := library.Save(dir, anim)
	if err != nil {
		t.Fatalf("saving fixture: %v", err)
	}

	gallery, err := newGallery(dir, defaultStyles())
	if err != nil {
		t.Fatalf("newGallery: %v", err)
	}
	m := model{gallery: gallery}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(model)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	topRow := func() string {
		return strings.TrimSpace(strings.Split(m.player.anim.Frames[0], "\n")[0])
	}
	if m.player.anim.FilterBackground {
		t.Fatal("filter enabled before toggle")
	}
	if topRow() == "" {
		t.Fatal("background already blank before toggle")
	}

	m = step(t, m, keyRune('f'))
	if !m.player.anim.FilterBackground {
		t.Fatal("filter not enabled after f")
	}
	if topRow() != "" {
		t.Errorf("top row = %q after enabling filter, want blank", topRow())
	}
	saved, err := library.Load(path)
	if err != nil {
		t.Fatalf("loading saved entry: %v", err)
	}
	if !saved.FilterBackground {
		t.Error("enabled filter not persisted to the frames file")
	}

	m = step(t, m, keyRune('f'))
	if m.player.anim.FilterBackground {
		t.Fatal("filter still enabled after second f")
	}
	if topRow() == "" {
		t.Error("top row blank after disabling filter, want background restored")
	}
	saved, err = library.Load(path)
	if err != nil {
		t.Fatalf("loading saved entry: %v", err)
	}
	if saved.FilterBackground {
		t.Error("disabled filter not persisted to the frames file")
	}
}

func TestPlayerFilterToggleIgnoredForLegacyEntries(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('f'))
	if m.player.refitting {
		t.Error("legacy entry without SourceGIF started a re-render")
	}
	if m.player.anim.FilterBackground {
		t.Error("legacy entry toggled FilterBackground")
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
