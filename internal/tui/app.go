// Package tui implements the interactive gallery and player.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ascii-tui/internal/library"
)

type screen int

const (
	screenGallery screen = iota
	screenRendering
	screenPlayer
)

// Messages exchanged between screens.
type (
	playEntryMsg struct {
		entries []library.Entry
		index   int
	}
	startRenderMsg struct{ gifPath string }
	renderDoneMsg  struct {
		savedPath string
		err       error
	}
	backToGalleryMsg struct{}
)

type model struct {
	screen  screen
	gallery galleryModel
	render  renderModel
	player  playerModel
	width   int
	height  int
}

// Run starts the interactive TUI over the given library directory.
func Run(libraryDir string) error {
	gallery, err := newGallery(libraryDir)
	if err != nil {
		return err
	}
	m := model{gallery: gallery}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.gallery.setSize(msg.Width, msg.Height)
		m.render.setSize(msg.Width, msg.Height)
		m.player.setSize(msg.Width, msg.Height)
		return m, nil

	case startRenderMsg:
		m.screen = screenRendering
		m.render = newRender(m.gallery.dir, msg.gifPath)
		m.render.setSize(m.width, m.height)
		return m, m.render.start()

	case renderDoneMsg:
		if msg.err != nil {
			m.screen = screenGallery
			m.gallery.status = fmt.Sprintf("render failed: %v", msg.err)
			return m, nil
		}
		if err := m.gallery.reload(); err != nil {
			m.screen = screenGallery
			m.gallery.status = err.Error()
			return m, nil
		}
		entries := m.gallery.entries()
		index := indexOfPath(entries, msg.savedPath)
		return m.startPlayer(entries, index)

	case playEntryMsg:
		return m.startPlayer(msg.entries, msg.index)

	case backToGalleryMsg:
		m.screen = screenGallery
		if err := m.gallery.reload(); err != nil {
			m.gallery.status = err.Error()
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenGallery:
		m.gallery, cmd = m.gallery.update(msg)
	case screenRendering:
		m.render, cmd = m.render.update(msg)
	case screenPlayer:
		m.player, cmd = m.player.update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	switch m.screen {
	case screenRendering:
		return m.render.view()
	case screenPlayer:
		return m.player.view()
	default:
		return m.gallery.view()
	}
}

func (m model) startPlayer(entries []library.Entry, index int) (tea.Model, tea.Cmd) {
	player, cmd := newPlayer(entries, index)
	player.setSize(m.width, m.height)
	m.player = player
	m.screen = screenPlayer
	return m, cmd
}

func indexOfPath(entries []library.Entry, path string) int {
	for i, e := range entries {
		if e.Path == path {
			return i
		}
	}
	return 0
}
