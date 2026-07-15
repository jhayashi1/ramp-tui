package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
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
	input  textinput.Model
	typing bool
	status string
}

func newGallery(dir string) (galleryModel, error) {
	delegate := list.NewDefaultDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "ascii-tui library"
	l.SetShowHelp(false)
	l.SetStatusBarItemName("animation", "animations")
	l.DisableQuitKeybindings()

	input := textinput.New()
	input.Placeholder = "path/to/animation.gif"

	g := galleryModel{dir: dir, list: l, input: input}
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
	g.list.SetSize(width, max(0, height-3))
	g.input.Width = max(10, width-8)
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
			if index := g.list.Index(); index >= 0 && len(g.list.Items()) > 0 {
				entries := g.entries()
				return g, func() tea.Msg { return playEntryMsg{entries: entries, index: index} }
			}
		case "a":
			g.typing = true
			g.status = ""
			g.input.SetValue("")
			return g, g.input.Focus()
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
			path := strings.TrimSpace(g.input.Value())
			g.typing = false
			g.input.Blur()
			if path == "" {
				return g, nil
			}
			return g, func() tea.Msg { return startRenderMsg{gifPath: path} }
		case "esc":
			g.typing = false
			g.input.Blur()
			return g, nil
		}
	}

	var cmd tea.Cmd
	g.input, cmd = g.input.Update(msg)
	return g, cmd
}

func (g galleryModel) view() string {
	var b strings.Builder
	b.WriteString(g.list.View())
	b.WriteByte('\n')
	if g.typing {
		b.WriteString(promptStyle.Render("render gif: "+g.input.View()) + "\n")
		b.WriteString(helpStyle.Render("[enter] render  [esc] cancel"))
		return b.String()
	}
	if g.status != "" {
		b.WriteString(statusStyle.Render(g.status) + "\n")
	}
	b.WriteString(helpStyle.Render("[enter] play  [a] add gif  [d] delete  [/] filter  [q] quit"))
	return b.String()
}
