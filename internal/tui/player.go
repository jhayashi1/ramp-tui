package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
)

type frameTickMsg struct{ gen int }

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
	return p, p.tickCmd()
}

func (p playerModel) view() string {
	if p.err != nil {
		return statusStyle.Render(fmt.Sprintf("error: %v", p.err)) + "\n" +
			helpStyle.Render("[esc] back  [q] quit")
	}
	if p.anim == nil {
		return ""
	}
	if p.anim.Width > p.width || p.anim.Height > p.height-1 {
		return promptStyle.Render(fmt.Sprintf(
			"animation is %dx%d but the terminal is %dx%d;\nenlarge the window or re-render with a smaller --width",
			p.anim.Width, p.anim.Height, p.width, p.height-1)) + "\n" +
			helpStyle.Render("[esc] back  [q] quit")
	}

	name := p.entries[p.index].Name
	state := ""
	if p.paused {
		state = "  paused"
	}
	status := fmt.Sprintf("%s  %d/%d%s  [space] pause  [<-/->] switch  [esc] back  [q] quit",
		name, p.frame+1, len(p.anim.Frames), state)

	var b strings.Builder
	b.WriteString(p.anim.Frames[p.frame])
	b.WriteByte('\n')
	b.WriteString(helpStyle.Render(status))
	return b.String()
}
