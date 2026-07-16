// Package tui implements the interactive gallery and player.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ascii-tui/internal/config"
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
	screen      screen
	gallery     galleryModel
	render      renderModel
	player      playerModel
	st          styles
	cfg         config.Config
	helpVisible bool
	width       int
	height      int
}

// Run starts the interactive TUI over the given library directory,
// using cfg for the theme and playback/render defaults.
func Run(libraryDir string, cfg config.Config) error {
	st := newStyles(theme{
		Accent: cfg.Theme.Accent,
		Border: cfg.Theme.Border,
		Text:   cfg.Theme.Text,
		Dim:    cfg.Theme.Dim,
		Error:  cfg.Theme.Error,
	})
	gallery, err := newGallery(libraryDir, st)
	if err != nil {
		return err
	}
	m := model{gallery: gallery, st: st, cfg: cfg}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.helpVisible {
		// Any key closes the overlay and is swallowed; other messages
		// (ticks, resize, preview loads) still flow through underneath
		// so nothing freezes while it's open.
		if _, ok := msg.(tea.KeyMsg); ok {
			m.helpVisible = false
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case toggleHelpMsg:
		m.helpVisible = !m.helpVisible
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		galleryCmd := m.gallery.setSize(msg.Width, msg.Height)
		m.render.setSize(msg.Width, msg.Height)
		m.player.setSize(msg.Width, msg.Height)
		if m.screen == screenPlayer {
			return m, m.player.scheduleRefit()
		}
		return m, galleryCmd

	case startRenderMsg:
		m.screen = screenRendering
		m.render = newRender(m.gallery.dir, msg.gifPath, m.st, m.cfg.Render.FilterBackground, m.cfg.Render.Complex)
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
	if m.helpVisible {
		return renderHelpOverlay(m.width, m.height, m.helpKeyMap(), m.st)
	}
	switch m.screen {
	case screenRendering:
		return m.render.view()
	case screenPlayer:
		return m.player.view()
	default:
		return m.gallery.view()
	}
}

func (m model) helpKeyMap() help.KeyMap {
	if m.screen == screenPlayer {
		return m.player.keys
	}
	return m.gallery.keys
}

func (m model) startPlayer(entries []library.Entry, index int) (tea.Model, tea.Cmd) {
	player, cmd := newPlayer(entries, index, m.st, m.cfg.Playback.Speed)
	player.setSize(m.width, m.height)
	m.player = player
	m.screen = screenPlayer
	// The stored render may not match this window (e.g. it was rendered
	// in a differently sized terminal), so schedule an initial refit.
	return m, tea.Batch(cmd, m.player.scheduleRefit())
}

func indexOfPath(entries []library.Entry, path string) int {
	for i, e := range entries {
		if e.Path == path {
			return i
		}
	}
	return 0
}
