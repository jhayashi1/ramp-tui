package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jhayashi1/ascii-tui/internal/library"
)

// minPreviewWidth and minPreviewHeight gate the preview column; below
// either threshold the gallery falls back to a single full-width list.
// The detail column additionally needs the preview to keep at least
// minMiddleForDetail columns after giving up detailWidth.
const (
	minPreviewWidth    = 56
	minPreviewHeight   = 12
	detailWidth        = 26
	minMiddleForDetail = 32
	colGutter          = 2
)

type entryItem struct{ library.Entry }

func (e entryItem) Title() string       { return e.Name }
func (e entryItem) Description() string { return e.Path }
func (e entryItem) FilterValue() string { return e.Name }

// barDelegate renders each entry as a single line; the selected row is
// marked with "▸ " and a background bar spanning the full list width,
// tuxedo-style. The row is truncated and padded as plain text first so
// the bar has no unstyled holes.
type barDelegate struct{ st styles }

func (d barDelegate) Height() int                         { return 1 }
func (d barDelegate) Spacing() int                        { return 0 }
func (d barDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d barDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	e, ok := item.(entryItem)
	if !ok {
		return
	}
	width := m.Width()
	if width <= 0 {
		fmt.Fprint(w, e.Name)
		return
	}
	name := truncateLabel(e.Name, max(1, width-2))
	if index != m.Index() {
		fmt.Fprint(w, "  "+d.st.text.Render(name))
		return
	}
	fmt.Fprint(w, d.st.selBarText.Render(fitLine("▸ "+name, width)))
}

// inputMode says what the gallery is collecting text for: a gif path
// (through the completing picker) or a new entry name (plain input).
type inputMode int

const (
	inputNone inputMode = iota
	inputAddGIF
	inputRename
	inputConfirmDelete
)

type galleryModel struct {
	dir        string
	list       list.Model
	picker     pathInput
	input      textinput.Model
	preview    previewModel
	mode       inputMode
	renamePath string
	deletePath string
	deleteName string
	status     string
	st         styles
	keys       galleryKeyMap
	width      int
	height     int
}

func newGallery(dir string, st styles) (galleryModel, error) {
	l := list.New(nil, barDelegate{st: st}, 0, 0)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.DisableQuitKeybindings()

	g := galleryModel{
		dir:     dir,
		list:    l,
		picker:  newPathInput(st),
		input:   textinput.New(),
		preview: newPreviewModel(st),
		st:      st,
		keys:    newGalleryKeyMap(),
	}
	if err := g.reload(); err != nil {
		return g, err
	}
	return g, nil
}

func (g *galleryModel) reload() error {
	entries, err := library.List(g.dir)
	if err != nil {
		return err
	}
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = entryItem{e}
	}
	g.list.SetItems(items)
	g.preview.reset()
	return nil
}

func (g *galleryModel) entries() []library.Entry {
	items := g.list.Items()
	entries := make([]library.Entry, len(items))
	for i, item := range items {
		entries[i] = item.(entryItem).Entry
	}
	return entries
}

func (g galleryModel) selectedEntry() (library.Entry, bool) {
	item, ok := g.list.SelectedItem().(entryItem)
	if !ok {
		return library.Entry{}, false
	}
	return item.Entry, true
}

// columnDims computes the widths of the library, preview, and detail
// columns plus the shared body height (one row is reserved for the
// status bar). Space is given up gracefully: the detail column drops
// first, then the preview, leaving a full-width list.
func (g galleryModel) columnDims() (leftW, midW, rightW, bodyH int, showPreview, showDetail bool) {
	bodyH = max(1, g.height-1)
	if g.width < minPreviewWidth || g.height < minPreviewHeight {
		return g.width, 0, 0, bodyH, false, false
	}
	leftW = min(34, max(24, g.width*22/100))
	midW = max(1, g.width-leftW-colGutter)
	if midW-detailWidth-colGutter >= minMiddleForDetail {
		rightW = detailWidth
		midW -= detailWidth + colGutter
		showDetail = true
	}
	return leftW, midW, rightW, bodyH, true, showDetail
}

// setSize resizes the gallery's panels. It returns a command that forces
// the preview to re-render the current selection when the resize
// invalidated its cache; reconcilePreview alone would not catch this,
// since the selected path hasn't changed.
func (g *galleryModel) setSize(width, height int) tea.Cmd {
	g.width, g.height = width, height
	leftW, _, _, _, _, _ := g.columnDims()
	innerLeft := max(10, leftW-14)
	g.picker.setWidth(innerLeft)
	g.input.Width = innerLeft
	if !g.layout() {
		return nil
	}
	if entry, ok := g.selectedEntry(); ok {
		return g.preview.selectEntry(entry)
	}
	return nil
}

// layout sizes the list and, when shown, the preview to their columns'
// interiors, reporting whether the preview's cache was invalidated.
// The left column spends three rows on chrome (title, spacer, section
// rule); the middle spends three on its title-less header (header line,
// spacer) plus renderColumn's clipping. renderColumn pads or clips
// short/long content to fit, so picker and rename inputs simply render
// inside the same fixed-height left column without special-casing here.
func (g *galleryModel) layout() bool {
	leftW, midW, _, bodyH, showPreview, _ := g.columnDims()
	g.list.SetSize(max(0, leftW), max(0, bodyH-3))
	if !showPreview {
		return false
	}
	return g.preview.setSize(max(0, midW), max(0, bodyH-2))
}

func (g *galleryModel) stopTyping() {
	g.mode = inputNone
	g.picker.blur()
	g.input.Blur()
	g.layout()
}

// reconcilePreview keeps the preview in sync with the list's current
// selection, restarting the debounce only when the selection actually
// changed. Resize-driven cache invalidation is handled separately by
// setSize, which forces a refresh regardless of whether the path moved.
func (g *galleryModel) reconcilePreview() tea.Cmd {
	entry, ok := g.selectedEntry()
	if !ok {
		if g.preview.path != "" {
			g.preview.clear()
		}
		return nil
	}
	if entry.Path == g.preview.path {
		return nil
	}
	return g.preview.selectEntry(entry)
}

func (g galleryModel) update(msg tea.Msg) (galleryModel, tea.Cmd) {
	switch msg.(type) {
	case previewTickMsg, previewDoneMsg:
		var cmd tea.Cmd
		g.preview, cmd = g.preview.update(msg)
		return g, cmd
	}

	g2, cmd := g.updateInner(msg)
	if preCmd := g2.reconcilePreview(); preCmd != nil {
		cmd = tea.Batch(cmd, preCmd)
	}
	return g2, cmd
}

func (g galleryModel) updateInner(msg tea.Msg) (galleryModel, tea.Cmd) {
	if g.mode != inputNone {
		return g.updateTyping(msg)
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok && g.list.FilterState() != list.Filtering {
		switch {
		case key.Matches(keyMsg, g.keys.Quit):
			return g, tea.Quit
		case key.Matches(keyMsg, g.keys.Play):
			// GlobalIndex maps the selection back into the full item
			// slice even when a filter is applied.
			if index := g.list.GlobalIndex(); index >= 0 && index < len(g.list.Items()) {
				entries := g.entries()
				return g, func() tea.Msg { return playEntryMsg{entries: entries, index: index} }
			}
		case key.Matches(keyMsg, g.keys.Add):
			g.mode = inputAddGIF
			g.status = ""
			g.layout()
			return g, g.picker.focus()
		case key.Matches(keyMsg, g.keys.Rename):
			if item, ok := g.list.SelectedItem().(entryItem); ok {
				g.mode = inputRename
				g.renamePath = item.Path
				g.status = ""
				g.input.SetValue(item.Name)
				g.input.CursorEnd()
				g.layout()
				return g, g.input.Focus()
			}
			return g, nil
		case key.Matches(keyMsg, g.keys.Delete):
			if item, ok := g.list.SelectedItem().(entryItem); ok {
				g.mode = inputConfirmDelete
				g.deletePath = item.Path
				g.deleteName = item.Name
				g.status = ""
				g.layout()
			}
			return g, nil
		case key.Matches(keyMsg, g.keys.Help):
			return g, func() tea.Msg { return toggleHelpMsg{} }
		}
	}

	var cmd tea.Cmd
	g.list, cmd = g.list.Update(msg)
	return g, cmd
}

func (g galleryModel) updateTyping(msg tea.Msg) (galleryModel, tea.Cmd) {
	switch g.mode {
	case inputRename:
		return g.updateRenaming(msg)
	case inputConfirmDelete:
		return g.updateConfirmDelete(msg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			path, ok := g.picker.accept()
			if !ok {
				return g, nil
			}
			g.stopTyping()
			return g, func() tea.Msg { return startRenderMsg{gifPath: path} }
		case "tab":
			g.picker.complete()
			return g, nil
		case "down", "ctrl+n":
			g.picker.moveSelection(1)
			return g, nil
		case "up", "ctrl+p", "shift+tab":
			g.picker.moveSelection(-1)
			return g, nil
		case "esc":
			g.stopTyping()
			return g, nil
		}
	}

	var cmd tea.Cmd
	g.picker, cmd = g.picker.update(msg)
	return g, cmd
}

func (g galleryModel) updateRenaming(msg tea.Msg) (galleryModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			name := strings.TrimSpace(g.input.Value())
			g.stopTyping()
			if name == "" {
				return g, nil
			}
			return g.commitRename(name)
		case "esc":
			g.stopTyping()
			return g, nil
		}
	}

	var cmd tea.Cmd
	g.input, cmd = g.input.Update(msg)
	return g, cmd
}

// updateConfirmDelete handles the y/n prompt raised by the delete key:
// "y" or "enter" deletes; any other key cancels, including "q" (which
// must not quit while a destructive action is pending confirmation).
func (g galleryModel) updateConfirmDelete(msg tea.Msg) (galleryModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return g, nil
	}
	path := g.deletePath
	g.stopTyping()
	switch keyMsg.String() {
	case "y", "enter":
		if err := os.Remove(path); err != nil {
			g.status = fmt.Sprintf("delete failed: %v", err)
		} else if err := g.reload(); err != nil {
			g.status = err.Error()
		}
	}
	return g, nil
}

// commitRename renames the selected entry and keeps it selected, since
// entries are listed by name and the rename can reorder them.
func (g galleryModel) commitRename(newName string) (galleryModel, tea.Cmd) {
	newPath, err := library.Rename(g.renamePath, newName)
	if err != nil {
		g.status = fmt.Sprintf("rename failed: %v", err)
		return g, nil
	}
	if err := g.reload(); err != nil {
		g.status = err.Error()
		return g, nil
	}
	for i, item := range g.list.Items() {
		if item.(entryItem).Path == newPath {
			g.list.Select(i)
			break
		}
	}
	return g, nil
}

func (g galleryModel) view() string {
	leftW, midW, rightW, bodyH, showPreview, showDetail := g.columnDims()

	var left string
	switch g.mode {
	case inputAddGIF:
		left = g.picker.view()
	case inputRename:
		left = g.st.prompt.Render("rename to: " + g.input.View())
	default:
		left = sectionRule(fmt.Sprintf("animations · %d", len(g.list.Items())), leftW, g.st) + "\n" + g.list.View()
	}

	body := renderColumn("library", left, leftW, bodyH, g.st)
	gutter := strings.Repeat(" ", colGutter)
	if showPreview {
		mid := renderColumn("", g.middleContent(midW), midW, bodyH, g.st)
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, gutter, mid)
	}
	if showDetail {
		detail := renderColumn("detail", g.detailContent(rightW), rightW, bodyH, g.st)
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, gutter, detail)
	}

	return body + "\n" + g.statusBar()
}

// middleContent stacks the tuxedo-style header line (glyph, name, dim
// metadata) above the centered preview frame.
func (g galleryModel) middleContent(width int) string {
	var header string
	if entry, ok := g.selectedEntry(); ok {
		metaText := ""
		if meta, ok := g.preview.currentMeta(); ok {
			metaText = fmt.Sprintf("%dx%d · %s · %s", meta.width, meta.height, plural(meta.frames, "frame"), meta.source)
		}
		header = headerLine("▸", entry.Name, metaText, width, g.st)
	}
	return header + "\n\n" + g.preview.view()
}

func (g galleryModel) detailContent(width int) string {
	if _, ok := g.selectedEntry(); !ok {
		return g.st.dim.Render("no selection")
	}
	meta, ok := g.preview.currentMeta()
	if !ok {
		return g.st.dim.Render("loading...")
	}
	return renderDetail(meta, width, g.st)
}

// statusBar builds the footer: a mode chip, mode-specific key hints (or
// a pending error) in the middle, and the library summary on the right.
func (g galleryModel) statusBar() string {
	chipStyle, chipLabel := g.st.chip, "NORMAL"
	middleStyle := g.st.help
	var middle string
	switch {
	case g.mode == inputAddGIF:
		chipLabel = "ADD"
		middle = "enter render · tab complete · ↑/↓ select · esc cancel"
	case g.mode == inputRename:
		chipLabel = "RENAME"
		middle = "enter rename · esc cancel"
	case g.mode == inputConfirmDelete:
		chipStyle, chipLabel = g.st.chipAlert, "DELETE"
		middleStyle = g.st.warning
		middle = fmt.Sprintf("delete %q? [y/n]", g.deleteName)
	case g.list.FilterState() == list.Filtering:
		chipLabel = "FILTER"
		middle = "type to filter · enter apply · esc cancel"
	case g.list.FilterState() == list.FilterApplied:
		chipLabel = "FILTER"
		middle = "enter play · esc clear · a add · r rename · d delete · ? help · q quit"
	default:
		middle = "enter play · a add · r rename · d delete · / filter · ? help · q quit"
	}
	if g.status != "" {
		middle, middleStyle = g.status, g.st.status
	}
	status := plural(len(g.list.Items()), "animation") + " · ascii-tui"
	return renderStatusBar(chipStyle, chipLabel, middle, middleStyle, status, g.width, g.st)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
