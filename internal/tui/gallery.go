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

// minPreviewWidth and minPreviewHeight gate the two-panel layout; below
// either threshold the gallery falls back to a single full-width panel.
const (
	minPreviewWidth  = 56
	minPreviewHeight = 12
)

type entryItem struct{ library.Entry }

func (e entryItem) Title() string       { return e.Name }
func (e entryItem) Description() string { return e.Path }
func (e entryItem) FilterValue() string { return e.Name }

// compactDelegate renders each entry as a single line: an accent "▸ "
// marker on the selected row, plain otherwise. The bordered panel around
// the list carries the title, so there is no per-item description row.
type compactDelegate struct{ st styles }

func (d compactDelegate) Height() int                         { return 1 }
func (d compactDelegate) Spacing() int                        { return 0 }
func (d compactDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	e, ok := item.(entryItem)
	if !ok {
		return
	}
	marker := "  "
	style := d.st.text
	if index == m.Index() {
		marker = d.st.accent.Render("▸ ")
		style = d.st.selected
	}
	fmt.Fprint(w, marker+style.Render(truncateLabel(e.Name, max(1, m.Width()-2))))
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
	l := list.New(nil, compactDelegate{st: st}, 0, 0)
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

// panelDims computes the outer size of the library (and, when the
// terminal is large enough, preview) panels plus the shared footer row.
func (g galleryModel) panelDims() (leftW, rightW, panelH int, showPreview bool) {
	panelH = max(1, g.height-1)
	if g.width < minPreviewWidth || g.height < minPreviewHeight {
		return g.width, 0, panelH, false
	}
	leftW = max(24, g.width*2/5)
	rightW = max(1, g.width-leftW)
	return leftW, rightW, panelH, true
}

// setSize resizes the gallery's panels. It returns a command that forces
// the preview to re-render the current selection when the resize
// invalidated its cache; reconcilePreview alone would not catch this,
// since the selected path hasn't changed.
func (g *galleryModel) setSize(width, height int) tea.Cmd {
	g.width, g.height = width, height
	leftW, _, _, _ := g.panelDims()
	innerLeft := max(10, leftW-2-8)
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

// layout sizes the list and, when shown, the preview to their panels'
// interiors, reporting whether the preview's cache was invalidated.
// renderPanel pads or clips short/long content to fit, so picker and
// rename inputs simply render inside the same fixed-height left panel
// without any special-casing here.
func (g *galleryModel) layout() bool {
	leftW, rightW, panelH, showPreview := g.panelDims()
	g.list.SetSize(max(0, leftW-2), max(0, panelH-2))
	if !showPreview {
		return false
	}
	return g.preview.setSize(max(0, rightW-2), max(0, panelH-2))
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
	leftW, rightW, panelH, showPreview := g.panelDims()

	var left string
	switch g.mode {
	case inputAddGIF:
		left = g.picker.view()
	case inputRename:
		left = g.st.prompt.Render("rename to: " + g.input.View())
	default:
		left = g.list.View()
	}

	body := renderPanel("library", left, leftW, panelH, g.st)
	if showPreview {
		right := renderPanel("preview", g.preview.view(), rightW, panelH, g.st)
		body = lipgloss.JoinHorizontal(lipgloss.Top, body, right)
	}

	return body + "\n" + renderFooter(g.footerText(), g.width, g.footerStyle(), g.st)
}

func (g galleryModel) footerText() string {
	if g.status != "" {
		return g.status
	}
	switch g.mode {
	case inputAddGIF:
		return "enter render · tab complete · up/down select · esc cancel"
	case inputRename:
		return "enter rename · esc cancel"
	case inputConfirmDelete:
		return fmt.Sprintf("delete %q? [y/n]", g.deleteName)
	default:
		return "enter play · a add · r rename · d delete · / filter · ? help · q quit"
	}
}

func (g galleryModel) footerStyle() lipgloss.Style {
	if g.status == "" && g.mode == inputConfirmDelete {
		return g.st.warning
	}
	return g.st.help
}
