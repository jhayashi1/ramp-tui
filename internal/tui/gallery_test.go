package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGalleryColumnDimsDegradesGracefully(t *testing.T) {
	if _, _, _, _, show, _ := (galleryModel{width: 50, height: 24}).columnDims(); show {
		t.Error("columnDims shows a preview below minPreviewWidth")
	}
	if _, _, _, _, show, _ := (galleryModel{width: 80, height: 10}).columnDims(); show {
		t.Error("columnDims shows a preview below minPreviewHeight")
	}

	// 80 cols: list + preview, no room for the detail column.
	leftW, midW, rightW, bodyH, show, detail := (galleryModel{width: 80, height: 24}).columnDims()
	if !show {
		t.Fatal("columnDims hides the preview at 80x24")
	}
	if detail || rightW != 0 {
		t.Errorf("detail shown at 80x24 (rightW=%d), want preview-only", rightW)
	}
	if leftW+colGutter+midW != 80 {
		t.Errorf("leftW+gutter+midW = %d, want 80", leftW+colGutter+midW)
	}
	if bodyH != 23 {
		t.Errorf("bodyH = %d, want 23", bodyH)
	}

	// 120 cols: all three columns.
	leftW, midW, rightW, _, show, detail = (galleryModel{width: 120, height: 30}).columnDims()
	if !show || !detail {
		t.Fatalf("columns at 120x30 = preview:%v detail:%v, want both", show, detail)
	}
	if rightW != detailWidth {
		t.Errorf("rightW = %d, want %d", rightW, detailWidth)
	}
	if leftW+midW+rightW+2*colGutter != 120 {
		t.Errorf("column widths sum to %d, want 120", leftW+midW+rightW+2*colGutter)
	}
}

// TestGalleryPreviewLoadsAndFollowsSelection drives the gallery through
// a real WindowSizeMsg and a cursor move, draining commands the way the
// Bubble Tea runtime would (see runCmd), to check the preview actually
// loads content and follows the list's selection.
func TestGalleryPreviewLoadsAndFollowsSelection(t *testing.T) {
	dir := t.TempDir()
	saveTinyEntry(t, dir, "first")
	saveTinyEntry(t, dir, "second")

	gallery, err := newGallery(dir, defaultStyles())
	if err != nil {
		t.Fatalf("newGallery: %v", err)
	}
	m := model{gallery: gallery}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(model)
	m = runCmd(t, m, cmd)

	firstPath := m.gallery.preview.path
	if firstPath == "" {
		t.Fatal("preview did not select the initial entry")
	}
	if meta, ok := m.gallery.preview.currentMeta(); !ok || meta.name != "first" {
		t.Errorf("preview meta = %+v (loaded=%v), want name %q", meta, ok, "first")
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.gallery.preview.path == firstPath {
		t.Fatal("preview did not follow the selection change")
	}
	if meta, ok := m.gallery.preview.currentMeta(); !ok || meta.name != "second" {
		t.Errorf("preview meta = %+v (loaded=%v), want name %q", meta, ok, "second")
	}
}

// TestGalleryPreviewReloadsAfterReturningFromPlayer guards against a
// regression where returning from the player left the preview stuck
// showing "loading preview..." forever: reload() cleared the cache but
// not the tracked path, so reconcilePreview saw no change and never
// re-fetched, and backToGalleryMsg didn't even call reconcilePreview.
func TestGalleryPreviewReloadsAfterReturningFromPlayer(t *testing.T) {
	dir := t.TempDir()
	saveTinyEntry(t, dir, "first")

	gallery, err := newGallery(dir, defaultStyles())
	if err != nil {
		t.Fatalf("newGallery: %v", err)
	}
	m := model{gallery: gallery}
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(model)
	m = runCmd(t, m, cmd)

	if _, ok := m.gallery.preview.currentMeta(); !ok {
		t.Fatalf("preview not loaded initially: %q", m.gallery.preview.view())
	}

	m = step(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenPlayer {
		t.Fatalf("screen = %v, want player", m.screen)
	}

	updated, cmd = m.Update(backToGalleryMsg{})
	m = updated.(model)
	m = runCmd(t, m, cmd)

	if m.screen != screenGallery {
		t.Fatalf("screen = %v, want gallery", m.screen)
	}
	if meta, ok := m.gallery.preview.currentMeta(); !ok || meta.name != "first" {
		t.Errorf("preview meta after returning from player = %+v (loaded=%v), want name %q", meta, ok, "first")
	}
}

func TestGalleryDeleteConfirmYDeletes(t *testing.T) {
	m := fixtureModel(t)
	targetPath := m.gallery.entries()[0].Path

	m = step(t, m, keyRune('d'))
	if m.gallery.mode != inputConfirmDelete {
		t.Fatalf("mode after d = %v, want inputConfirmDelete", m.gallery.mode)
	}

	m = step(t, m, keyRune('y'))
	if m.gallery.mode != inputNone {
		t.Errorf("mode after y = %v, want inputNone", m.gallery.mode)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Errorf("entry file still exists after confirming delete: %v", err)
	}
	if got := len(m.gallery.entries()); got != 1 {
		t.Errorf("entries after delete = %d, want 1", got)
	}
}

func TestGalleryDeleteConfirmNKeeps(t *testing.T) {
	m := fixtureModel(t)
	targetPath := m.gallery.entries()[0].Path

	m = step(t, m, keyRune('d'))
	m = step(t, m, keyRune('n'))

	if m.gallery.mode != inputNone {
		t.Errorf("mode after n = %v, want inputNone", m.gallery.mode)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("entry file removed despite cancelling: %v", err)
	}
	if got := len(m.gallery.entries()); got != 2 {
		t.Errorf("entries after cancelled delete = %d, want 2", got)
	}
}

func TestGalleryDeleteConfirmQDoesNotQuit(t *testing.T) {
	m := fixtureModel(t)
	targetPath := m.gallery.entries()[0].Path
	m = step(t, m, keyRune('d'))

	updated, cmd := m.Update(keyRune('q'))
	m = updated.(model)
	if cmd != nil {
		t.Fatal("q during delete confirmation produced a command (want cancel-only, no quit)")
	}
	if m.gallery.mode != inputNone {
		t.Errorf("mode after q = %v, want inputNone (cancelled)", m.gallery.mode)
	}
	if m.screen != screenGallery {
		t.Errorf("screen after q during confirm = %v, want gallery", m.screen)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Errorf("entry file removed despite q cancelling: %v", err)
	}
}
