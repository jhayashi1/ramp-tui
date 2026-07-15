package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jhayashi1/ascii-tui/internal/library"
)

var (
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 2)
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Padding(0, 2)
	promptStyle = lipgloss.NewStyle().Padding(0, 2)
)

type entryItem struct{ library.Entry }

func (e entryItem) Title() string       { return e.Name }
func (e entryItem) Description() string { return e.Path }
func (e entryItem) FilterValue() string { return e.Name }

type galleryModel struct {
	dir    string
	list   list.Model
	picker pathInput
	typing bool
	status string
	width  int
	height int
}

func newGallery(dir string) (galleryModel, error) {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "ascii-tui library"
	l.SetShowHelp(false)
	l.SetStatusBarItemName("animation", "animations")
	l.DisableQuitKeybindings()

	g := galleryModel{dir: dir, list: l, picker: newPathInput()}
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

func (g *galleryModel) setSize(width, height int) {
	g.width, g.height = width, height
	g.picker.setWidth(max(10, width-8))
	g.layout()
}

// layout sizes the list, reserving room for the completion rows while
// the path prompt is open so the view height stays constant.
func (g *galleryModel) layout() {
	reserved := 3
	if g.typing {
		reserved += maxVisibleSuggestions
	}
	g.list.SetSize(g.width, max(0, g.height-reserved))
}

func (g *galleryModel) stopTyping() {
	g.typing = false
	g.picker.blur()
	g.layout()
}

func (g galleryModel) update(msg tea.Msg) (galleryModel, tea.Cmd) {
	if g.typing {
		return g.updateTyping(msg)
	}

	if key, ok := msg.(tea.KeyMsg); ok && g.list.FilterState() != list.Filtering {
		switch key.String() {
		case "q", "ctrl+c":
			return g, tea.Quit
		case "enter":
			// GlobalIndex maps the selection back into the full item
			// slice even when a filter is applied.
			if index := g.list.GlobalIndex(); index >= 0 && index < len(g.list.Items()) {
				entries := g.entries()
				return g, func() tea.Msg { return playEntryMsg{entries: entries, index: index} }
			}
		case "a":
			g.typing = true
			g.status = ""
			g.layout()
			return g, g.picker.focus()
		case "d":
			if item, ok := g.list.SelectedItem().(entryItem); ok {
				if err := os.Remove(item.Path); err != nil {
					g.status = fmt.Sprintf("delete failed: %v", err)
				} else if err := g.reload(); err != nil {
					g.status = err.Error()
				}
			}
			return g, nil
		}
	}

	var cmd tea.Cmd
	g.list, cmd = g.list.Update(msg)
	return g, cmd
}

func (g galleryModel) updateTyping(msg tea.Msg) (galleryModel, tea.Cmd) {
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

func (g galleryModel) view() string {
	var b strings.Builder
	b.WriteString(g.list.View())
	b.WriteByte('\n')
	if g.typing {
		b.WriteString(g.picker.view())
		b.WriteString(helpStyle.Render("[enter] render/open  [tab] complete  [↑/↓] select  [esc] cancel"))
		return b.String()
	}
	if g.status != "" {
		b.WriteString(statusStyle.Render(g.status) + "\n")
	}
	b.WriteString(helpStyle.Render("[enter] play  [a] add gif  [d] delete  [/] filter  [q] quit"))
	return b.String()
}
