package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ramp-tui/internal/frames"
	"github.com/jhayashi1/ramp-tui/internal/library"
)

func saveTinyEntry(t *testing.T, dir, name string) library.Entry {
	t.Helper()
	anim := &frames.Animation{
		SourceName: name + ".gif",
		Width:      2,
		Height:     2,
		Frames:     []string{"ab\ncd"},
		Delays:     []time.Duration{50 * time.Millisecond},
	}
	path, err := library.Save(dir, anim)
	if err != nil {
		t.Fatalf("saving fixture: %v", err)
	}
	return library.Entry{Name: name, Path: path}
}

func TestPreviewSelectEntryDebouncesAndCaches(t *testing.T) {
	entry := saveTinyEntry(t, t.TempDir(), "a")

	p := newPreviewModel(defaultStyles())
	p.setSize(20, 10)

	cmd := p.selectEntry(entry)
	if cmd == nil {
		t.Fatal("selectEntry returned nil cmd for an uncached entry")
	}
	tick, ok := cmd().(previewTickMsg)
	if !ok || tick.gen != p.gen || tick.path != entry.Path {
		t.Fatalf("tick msg = %#v, want gen=%d path=%q", tick, p.gen, entry.Path)
	}

	var renderCmd tea.Cmd
	p, renderCmd = p.update(tick)
	if renderCmd == nil {
		t.Fatal("previewTickMsg produced no render command")
	}
	done, ok := renderCmd().(previewDoneMsg)
	if !ok || done.err != nil {
		t.Fatalf("render cmd produced %#v, want a successful previewDoneMsg", done)
	}
	p, _ = p.update(done)

	if _, cached := p.cache[entry.Path]; !cached {
		t.Fatal("entry not cached after previewDoneMsg")
	}
	if got := p.view(); got == "" {
		t.Error("view is empty after a successful render")
	}

	if again := p.selectEntry(entry); again != nil {
		t.Error("selectEntry restarted the debounce for an already-cached entry")
	}
}

func TestPreviewUpdateDropsStaleTick(t *testing.T) {
	entry := saveTinyEntry(t, t.TempDir(), "a")

	p := newPreviewModel(defaultStyles())
	p.setSize(20, 10)
	p.selectEntry(entry)

	stale := previewTickMsg{gen: p.gen - 1, path: entry.Path}
	_, cmd := p.update(stale)
	if cmd != nil {
		t.Error("stale tick produced a render command")
	}
}

func TestPreviewUpdateDropsStaleDone(t *testing.T) {
	p := newPreviewModel(defaultStyles())
	p.setSize(20, 10)
	p.cache = map[string]previewEntry{"x": {content: "kept"}}
	p.gen = 5

	stale := previewDoneMsg{gen: 4, path: "x", content: "overwritten"}
	p, _ = p.update(stale)
	if p.cache["x"].content != "kept" {
		t.Error("stale previewDoneMsg overwrote a cached entry")
	}
}

func TestPreviewSetSizeInvalidatesCacheOnlyOnChange(t *testing.T) {
	p := newPreviewModel(defaultStyles())
	p.setSize(20, 10)
	p.cache["x"] = previewEntry{content: "cached"}

	if changed := p.setSize(20, 10); changed {
		t.Error("setSize reported a change for identical dimensions")
	}
	if _, ok := p.cache["x"]; !ok {
		t.Error("cache cleared despite unchanged dimensions")
	}

	if changed := p.setSize(30, 10); !changed {
		t.Error("setSize did not report a change for new dimensions")
	}
	if _, ok := p.cache["x"]; ok {
		t.Error("cache not cleared after a size change")
	}
}

func TestPreviewClearResetsSelection(t *testing.T) {
	entry := saveTinyEntry(t, t.TempDir(), "a")
	p := newPreviewModel(defaultStyles())
	p.setSize(20, 10)
	p.selectEntry(entry)

	p.clear()
	if p.path != "" || p.name != "" {
		t.Errorf("path/name = %q/%q after clear, want empty", p.path, p.name)
	}
	if got := p.view(); got == "" {
		t.Error("view is empty after clear, want a placeholder message")
	}
}
