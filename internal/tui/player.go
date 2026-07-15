package tui

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jhayashi1/ascii-tui/internal/engine"
	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
)

// refitDebounce is how long the player waits after a resize before
// re-rendering, so resize storms trigger a single render.
const refitDebounce = 250 * time.Millisecond

type frameTickMsg struct{ gen int }

// refitTickMsg fires when the resize debounce timer elapses.
type refitTickMsg struct{ gen int }

// refitDoneMsg carries the result of a background re-render.
type refitDoneMsg struct {
	gen  int
	anim *frames.Animation
	err  error
}

type playerModel struct {
	entries []library.Entry
	index   int
	anim    *frames.Animation
	frame   int
	gen     int
	paused  bool
	err     error
	width   int
	height  int
	// refitGen invalidates pending debounce timers and in-flight
	// re-renders whenever the viewport or entry changes again.
	refitGen  int
	refitting bool
}

func newPlayer(entries []library.Entry, index int) (playerModel, tea.Cmd) {
	p := playerModel{entries: entries, index: index}
	p.load()
	return p, p.tickCmd()
}

func (p *playerModel) load() {
	p.frame = 0
	p.gen++
	p.err = nil
	p.refitting = false
	if len(p.entries) == 0 {
		p.err = fmt.Errorf("library is empty")
		return
	}
	anim, err := library.Load(p.entries[p.index].Path)
	if err != nil {
		p.err = err
		return
	}
	p.anim = anim
}

func (p *playerModel) setSize(width, height int) {
	p.width, p.height = width, height
}

// viewport returns the area available for the animation, reserving one
// row for the status line.
func (p playerModel) viewport() (w, h int) {
	return p.width, p.height - 1
}

// needsRefit reports whether the animation should be re-rendered for
// the current viewport: it either overflows, or leaves slack on both
// axes. A fresh render always saturates one axis, so a refit at the
// current size never re-triggers.
func (p playerModel) needsRefit() bool {
	if p.anim == nil || p.anim.SourceGIF == nil {
		return false
	}
	vw, vh := p.viewport()
	if vw <= 0 || vh <= 0 {
		return false
	}
	tooBig := p.anim.Width > vw || p.anim.Height > vh
	underfilled := p.anim.Width < vw && p.anim.Height < vh
	return tooBig || underfilled
}

// scheduleRefit starts (or restarts) the resize debounce timer,
// cancelling any pending timer or in-flight re-render.
func (p *playerModel) scheduleRefit() tea.Cmd {
	if p.anim == nil || p.anim.SourceGIF == nil {
		return nil
	}
	p.refitGen++
	p.refitting = false
	gen := p.refitGen
	return tea.Tick(refitDebounce, func(time.Time) tea.Msg {
		return refitTickMsg{gen: gen}
	})
}

// refitCmd re-renders the animation from its embedded GIF to fit the
// current viewport, using the options it was originally rendered with.
func (p playerModel) refitCmd() tea.Cmd {
	gen := p.refitGen
	anim := p.anim
	opts := p.renderOptions()
	return func() tea.Msg {
		re, err := engine.Render(bytes.NewReader(anim.SourceGIF), opts, nil)
		return refitDoneMsg{gen: gen, anim: re, err: err}
	}
}

// toggleFilterCmd re-renders with background filtering flipped and saves
// the result back to the library so the choice survives restarts.
func (p playerModel) toggleFilterCmd() tea.Cmd {
	gen := p.refitGen
	anim := p.anim
	path := p.entries[p.index].Path
	opts := p.renderOptions()
	opts.FilterBackground = !anim.FilterBackground
	return func() tea.Msg {
		re, err := engine.Render(bytes.NewReader(anim.SourceGIF), opts, nil)
		if err != nil {
			return refitDoneMsg{gen: gen, err: err}
		}
		re.SourceGIF = anim.SourceGIF
		re.SourceName = anim.SourceName
		if err := library.Write(path, re); err != nil {
			return refitDoneMsg{gen: gen, err: err}
		}
		return refitDoneMsg{gen: gen, anim: re}
	}
}

// renderOptions rebuilds the engine options the animation was rendered
// with, bounded by the current viewport.
func (p playerModel) renderOptions() engine.Options {
	vw, vh := p.viewport()
	return engine.Options{
		Colored:          p.anim.Colored,
		Complex:          p.anim.Complex,
		FilterBackground: p.anim.FilterBackground,
		CustomRamp:       p.anim.CustomRamp,
		MaxWidth:         vw,
		MaxHeight:        max(1, vh),
	}
}

func (p playerModel) tickCmd() tea.Cmd {
	if p.err != nil || p.anim == nil || p.paused {
		return nil
	}
	gen := p.gen
	return tea.Tick(p.anim.Delays[p.frame], func(time.Time) tea.Msg {
		return frameTickMsg{gen: gen}
	})
}

func (p playerModel) update(msg tea.Msg) (playerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case frameTickMsg:
		if msg.gen != p.gen || p.paused || p.anim == nil {
			return p, nil
		}
		p.frame = (p.frame + 1) % len(p.anim.Frames)
		return p, p.tickCmd()

	case refitTickMsg:
		if msg.gen != p.refitGen || !p.needsRefit() {
			return p, nil
		}
		p.refitting = true
		return p, p.refitCmd()

	case refitDoneMsg:
		if msg.gen != p.refitGen {
			return p, nil
		}
		p.refitting = false
		if msg.err != nil {
			return p, nil
		}
		// Carry the source and name forward so later refits still work;
		// frame count and delays match the old render (same GIF), so
		// playback position, pause state, and the tick loop carry over.
		msg.anim.SourceGIF = p.anim.SourceGIF
		msg.anim.SourceName = p.anim.SourceName
		p.anim = msg.anim
		if p.frame >= len(p.anim.Frames) {
			p.frame = 0
		}
		return p, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return p, tea.Quit
		case "esc":
			return p, func() tea.Msg { return backToGalleryMsg{} }
		case " ":
			p.paused = !p.paused
			p.gen++
			return p, p.tickCmd()
		case "left", "h":
			return p.switchTo(p.index - 1)
		case "right", "l":
			return p.switchTo(p.index + 1)
		case "f":
			if p.anim == nil || p.anim.SourceGIF == nil {
				return p, nil
			}
			p.refitGen++
			p.refitting = true
			return p, p.toggleFilterCmd()
		}
	}
	return p, nil
}

func (p playerModel) switchTo(index int) (playerModel, tea.Cmd) {
	if len(p.entries) == 0 {
		return p, nil
	}
	p.index = ((index % len(p.entries)) + len(p.entries)) % len(p.entries)
	p.paused = false
	p.load()
	return p, tea.Batch(p.tickCmd(), (&p).scheduleRefit())
}

func (p playerModel) view() string {
	if p.err != nil {
		return statusStyle.Render(fmt.Sprintf("error: %v", p.err)) + "\n" +
			helpStyle.Render("[esc] back  [q] quit")
	}
	if p.anim == nil {
		return ""
	}
	vw, vh := p.viewport()
	if p.anim.Width > vw || p.anim.Height > vh {
		// The frame cannot be placed without breaking layout. Resizable
		// entries are about to be refitted; legacy ones can only ask
		// the user for more room.
		if p.anim.SourceGIF != nil {
			return lipgloss.Place(vw, max(0, vh), lipgloss.Center, lipgloss.Center,
				fmt.Sprintf("fitting to %dx%d...", vw, vh)) + "\n" +
				helpStyle.Render("[esc] back  [q] quit")
		}
		return promptStyle.Render(fmt.Sprintf(
			"animation is %dx%d but the terminal is %dx%d;\nenlarge the window or re-render with a smaller --width",
			p.anim.Width, p.anim.Height, vw, vh)) + "\n" +
			helpStyle.Render("[esc] back  [q] quit")
	}

	name := p.entries[p.index].Name
	state := ""
	if p.paused {
		state = "  paused"
	}
	if p.refitting {
		state += "  fitting..."
	}
	status := fmt.Sprintf("%s  %d/%d%s  [space] pause  [<-/->] switch  [f] filter bg  [esc] back  [q] quit",
		name, p.frame+1, len(p.anim.Frames), state)

	var b strings.Builder
	b.WriteString(lipgloss.Place(vw, max(0, vh), lipgloss.Center, lipgloss.Center,
		p.anim.Frames[p.frame]))
	b.WriteByte('\n')
	b.WriteString(helpStyle.Render(status))
	return b.String()
}
