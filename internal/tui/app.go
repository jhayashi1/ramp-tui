// Package tui implements the interactive gallery and player.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ramp-tui/internal/config"
	"github.com/jhayashi1/ramp-tui/internal/library"
)

type screen int

const (
	screenGallery screen = iota
	screenRendering
	screenPlayer
	screenKeybinds
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
	// cycleThemeMsg asks the app to advance to the next built-in theme;
	// themeSavedMsg reports whether persisting that choice succeeded.
	cycleThemeMsg struct{}
	themeSavedMsg struct{ err error }
)

type model struct {
	screen      screen
	gallery     galleryModel
	render      renderModel
	player      playerModel
	keybinds    keybindsModel
	st          styles
	cfg         config.Config
	themeIndex  int
	helpVisible bool
	width       int
	height      int
}

// Run starts the interactive TUI over the given library directory,
// using cfg for the theme and playback/render defaults.
func Run(libraryDir string, cfg config.Config) error {
	st := newStyles(themeFromConfig(cfg.Theme))
	gallery, err := newGallery(libraryDir, st)
	if err != nil {
		return err
	}
	m := model{gallery: gallery, st: st, cfg: cfg, themeIndex: themeIndexByName(cfg.Theme.Name)}
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
		m.keybinds.setSize(msg.Width, msg.Height)
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
			return m, m.gallery.flashStatus(fmt.Sprintf("render failed: %v", msg.err))
		}
		if err := m.gallery.reload(); err != nil {
			m.screen = screenGallery
			return m, m.gallery.flashStatus(err.Error())
		}
		entries := m.gallery.entries()
		index := indexOfPath(entries, msg.savedPath)
		return m.startPlayer(entries, index)

	case playEntryMsg:
		return m.startPlayer(msg.entries, msg.index)

	case openKeybindsMsg:
		m.screen = screenKeybinds
		m.keybinds = newKeybinds(m.cfg.Keys, m.st)
		m.keybinds.setSize(m.width, m.height)
		return m, nil

	case keysChangedMsg:
		// Adopt the edited bindings immediately (the next player launch
		// uses them) and persist them in the background; the save result
		// flows back to the keybinds screen's status bar.
		m.cfg.Keys = msg.keys
		cfg := m.cfg
		return m, func() tea.Msg { return keysSavedMsg{err: config.Save(cfg)} }

	case cycleThemeMsg:
		// Advance to the next preset (an unknown/custom starting theme has
		// index -1, so the first press lands on preset 0), repaint the
		// gallery live, and persist the choice so it survives restarts.
		m.themeIndex = (m.themeIndex + 1) % len(themePresets)
		preset := themePresets[m.themeIndex]
		m.st = newStyles(preset.theme)
		m.cfg.Theme = preset.configTheme()
		m.gallery.setStyles(m.st)
		flash := m.gallery.flashStatus("theme: " + preset.name)
		cfg := m.cfg
		return m, tea.Batch(flash, func() tea.Msg { return themeSavedMsg{err: config.Save(cfg)} })

	case themeSavedMsg:
		if msg.err != nil {
			return m, m.gallery.flashStatus(fmt.Sprintf("theme save failed: %v", msg.err))
		}
		return m, nil

	case clearGalleryStatusMsg:
		if msg.gen == m.gallery.statusGen {
			m.gallery.status = ""
		}
		return m, nil

	case clearKeybindsStatusMsg:
		if msg.gen == m.keybinds.statusGen {
			m.keybinds.status = ""
		}
		return m, nil

	case backToGalleryMsg:
		m.screen = screenGallery
		if err := m.gallery.reload(); err != nil {
			return m, m.gallery.flashStatus(err.Error())
		}
		return m, m.gallery.reconcilePreview()
	}

	var cmd tea.Cmd
	switch m.screen {
	case screenGallery:
		m.gallery, cmd = m.gallery.update(msg)
	case screenRendering:
		m.render, cmd = m.render.update(msg)
	case screenPlayer:
		m.player, cmd = m.player.update(msg)
	case screenKeybinds:
		m.keybinds, cmd = m.keybinds.update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	var view string
	switch {
	case m.helpVisible:
		view = renderHelpOverlay(m.width, m.height, m.helpKeyMap(), m.st)
	case m.screen == screenRendering:
		view = m.render.view()
	case m.screen == screenPlayer:
		view = m.player.view()
	case m.screen == screenKeybinds:
		view = m.keybinds.view()
	default:
		view = m.gallery.view()
	}
	return paintBackground(view, m.width, m.height, m.st)
}

func (m model) helpKeyMap() help.KeyMap {
	switch m.screen {
	case screenPlayer:
		return m.player.keys
	case screenKeybinds:
		return m.keybinds.menu
	default:
		return m.gallery.keys
	}
}

func (m model) startPlayer(entries []library.Entry, index int) (tea.Model, tea.Cmd) {
	player, cmd := newPlayer(entries, index, m.st, m.cfg.Playback.Speed, m.cfg.Keys)
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
