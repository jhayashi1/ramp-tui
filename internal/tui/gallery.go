package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jhayashi1/ramp-tui/internal/library"
)

// statusTimeout is how long a transient footer message (a theme switch,
// a rename/delete result, a keybind warning) stays up before the footer
// reverts to its normal key hints. It is a var only so tests can shorten
// it; nothing mutates it at runtime.
var statusTimeout = 3 * time.Second

// clearGalleryStatusMsg and clearKeybindsStatusMsg clear their screen's
// footer message after statusTimeout. gen is the value of that model's
// statusGen when the timer was armed, so a newer message (which bumps
// statusGen) is never wiped by an older timer.
type (
	clearGalleryStatusMsg  struct{ gen int }
	clearKeybindsStatusMsg struct{ gen int }
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
	dir          string
	list         list.Model
	picker       pathInput
	input        textinput.Model
	preview      previewModel
	mode         inputMode
	renamePath   string
	deletePath   string
	deleteName   string
	deleteCursor int
	status       string
	statusGen    int
	st           styles
	keys         galleryKeyMap
	width        int
	height       int
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

// setStyles swaps in a new theme's styles across the gallery's
// long-lived sub-components (the list delegate, path picker, and
// preview) so a runtime theme change takes effect without rebuilding the
// screen. Other screens are constructed on demand from the app's styles,
// so they pick up the new theme on their next open.
func (g *galleryModel) setStyles(st styles) {
	g.st = st
	g.list.SetDelegate(barDelegate{st: st})
	g.picker.st = st
	g.preview.st = st
}

// flashStatus shows a transient footer message and returns a command
// that clears it after statusTimeout, reverting the footer to its normal
// hints. statusGen guards against an older timer wiping a newer message.
func (g *galleryModel) flashStatus(msg string) tea.Cmd {
	g.status = msg
	g.statusGen++
	gen := g.statusGen
	return tea.Tick(statusTimeout, func(time.Time) tea.Msg {
		return clearGalleryStatusMsg{gen: gen}
	})
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
				g.deleteCursor = 0
				g.status = ""
				g.layout()
			}
			return g, nil
		case key.Matches(keyMsg, g.keys.Theme):
			return g, func() tea.Msg { return cycleThemeMsg{} }
		case key.Matches(keyMsg, g.keys.Keybinds):
			return g, func() tea.Msg { return openKeybindsMsg{} }
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

// deleteConfirmOptions are the choices in the centered delete menu, in
// cursor order. Cancel is first so it is the default highlight and a
// stray enter never deletes.
var deleteConfirmOptions = []string{"cancel", "delete"}

// updateConfirmDelete drives the centered delete menu: arrow keys (or
// j/k, tab) move between Cancel and Delete, enter commits the highlighted
// choice, and esc cancels. Every other key — notably "q" — is swallowed
// so a destructive action is never triggered nor the app quit by accident
// while the confirmation is up.
func (g galleryModel) updateConfirmDelete(msg tea.Msg) (galleryModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return g, nil
	}
	switch keyMsg.String() {
	case "up", "k", "left", "h", "shift+tab":
		g.deleteCursor = (g.deleteCursor - 1 + len(deleteConfirmOptions)) % len(deleteConfirmOptions)
	case "down", "j", "right", "l", "tab":
		g.deleteCursor = (g.deleteCursor + 1) % len(deleteConfirmOptions)
	case "esc":
		g.stopTyping()
	case "enter":
		confirmed := deleteConfirmOptions[g.deleteCursor] == "delete"
		path := g.deletePath
		g.stopTyping()
		if !confirmed {
			return g, nil
		}
		if err := os.Remove(path); err != nil {
			return g, (&g).flashStatus(fmt.Sprintf("delete failed: %v", err))
		}
		if err := g.reload(); err != nil {
			return g, (&g).flashStatus(err.Error())
		}
		// reload keeps the old cursor index, which now dangles past the end
		// when the deleted entry was the last one; clamp it so a row stays
		// selected (the entry that shifted up into the freed slot).
		if n := len(g.list.Items()); n > 0 && g.list.Index() >= n {
			g.list.Select(n - 1)
		}
	}
	return g, nil
}

// commitRename renames the selected entry and keeps it selected, since
// entries are listed by name and the rename can reorder them.
func (g galleryModel) commitRename(newName string) (galleryModel, tea.Cmd) {
	newPath, err := library.Rename(g.renamePath, newName)
	if err != nil {
		return g, (&g).flashStatus(fmt.Sprintf("rename failed: %v", err))
	}
	if err := g.reload(); err != nil {
		return g, (&g).flashStatus(err.Error())
	}
	for i, item := range g.list.Items() {
		if item.(entryItem).Path == newPath {
			g.list.Select(i)
			break
		}
	}
	return g, nil
}

// deleteMenuWidth fits the delete question and both option rows
// comfortably; long entry names are truncated to it.
const deleteMenuWidth = 40

func (g galleryModel) view() string {
	if g.mode == inputConfirmDelete {
		return g.deleteMenuView()
	}

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

// deleteMenuView centers the confirmation panel over the body, keeping
// the shared status bar on the bottom row.
func (g galleryModel) deleteMenuView() string {
	bodyH := max(1, g.height-1)
	panel := lipgloss.Place(max(1, g.width), bodyH, lipgloss.Center, lipgloss.Center, g.deleteMenuPanel())
	return panel + "\n" + g.statusBar()
}

// deleteMenuPanel builds the centered menu: a warning-styled question
// above the Cancel/Delete rows, the highlighted row barred like the list
// selection. Rows share one width so lipgloss.Place keeps them
// left-aligned as a block rather than centering each on its own.
func (g galleryModel) deleteMenuPanel() string {
	panelW := min(deleteMenuWidth, max(1, g.width))
	question := g.st.warning.Render(truncateLabel(fmt.Sprintf("delete %q?", g.deleteName), panelW))
	rows := []string{fitLine(question, panelW), ""}
	for i, opt := range deleteConfirmOptions {
		if i == g.deleteCursor {
			rows = append(rows, g.st.selBarText.Render(fitLine("▸ "+opt, panelW)))
		} else {
			rows = append(rows, fitLine("  "+g.st.text.Render(opt), panelW))
		}
	}
	return strings.Join(rows, "\n")
}

// middleContent stacks the header line (selection glyph and name on the
// left, app wordmark on the right) above the centered preview frame.
func (g galleryModel) middleContent(width int) string {
	name := ""
	if entry, ok := g.selectedEntry(); ok {
		name = entry.Name
	}
	return headerLine(name, width, g.st) + "\n\n" + g.preview.view()
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
		middle = "↑/↓ select · enter confirm · esc cancel"
	case g.list.FilterState() == list.Filtering:
		chipLabel = "FILTER"
		middle = "type to filter · enter apply · esc cancel"
	case g.list.FilterState() == list.FilterApplied:
		chipLabel = "FILTER"
		middle = "enter play · esc clear · a add · r rename · d delete · t theme · k keybinds · ? help · ctrl+c quit"
	default:
		middle = "enter play · a add · r rename · d delete · / filter · t theme · k keybinds · ? help · ctrl+c quit"
	}
	if g.status != "" {
		middle, middleStyle = g.status, g.st.status
	}
	status := plural(len(g.list.Items()), "animation") + " · " + brandName
	return renderStatusBar(chipStyle, chipLabel, middle, middleStyle, status, g.width, g.st)
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
