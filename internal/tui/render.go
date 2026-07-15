package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ascii-tui/internal/engine"
	"github.com/jhayashi1/ascii-tui/internal/library"
)

type renderEventMsg struct {
	done      int
	total     int
	savedPath string
	err       error
	finished  bool
}

type renderModel struct {
	dir     string
	gifPath string
	bar     progress.Model
	events  chan renderEventMsg
	percent float64
	width   int
}

func newRender(dir, gifPath string) renderModel {
	return renderModel{
		dir:     dir,
		gifPath: gifPath,
		bar:     progress.New(progress.WithDefaultGradient()),
		events:  make(chan renderEventMsg, 16),
	}
}

func (r *renderModel) setSize(width, _ int) {
	r.width = width
	r.bar.Width = max(10, min(60, width-8))
}

// start kicks off the render in the background and begins pumping its
// progress events into the update loop.
func (r renderModel) start() tea.Cmd {
	return tea.Batch(r.runRender(), r.nextEvent())
}

func (r renderModel) runRender() tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(r.gifPath)
		if err != nil {
			r.events <- renderEventMsg{finished: true, err: err}
			return nil
		}
		defer f.Close()

		anim, err := engine.Render(f, engine.Options{Colored: true}, func(done, total int) {
			select {
			case r.events <- renderEventMsg{done: done, total: total}:
			default:
			}
		})
		if err != nil {
			r.events <- renderEventMsg{finished: true, err: err}
			return nil
		}
		anim.SourceName = filepath.Base(r.gifPath)

		path, err := library.Save(r.dir, anim)
		r.events <- renderEventMsg{finished: true, savedPath: path, err: err}
		return nil
	}
}

func (r renderModel) nextEvent() tea.Cmd {
	return func() tea.Msg { return <-r.events }
}

func (r renderModel) update(msg tea.Msg) (renderModel, tea.Cmd) {
	event, ok := msg.(renderEventMsg)
	if !ok {
		return r, nil
	}
	if event.finished {
		return r, func() tea.Msg {
			return renderDoneMsg{savedPath: event.savedPath, err: event.err}
		}
	}
	if event.total > 0 {
		r.percent = float64(event.done) / float64(event.total)
	}
	return r, r.nextEvent()
}

func (r renderModel) view() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(promptStyle.Render(fmt.Sprintf("rendering %s", filepath.Base(r.gifPath))))
	b.WriteString("\n\n")
	b.WriteString(promptStyle.Render(r.bar.ViewAs(r.percent)))
	return b.String()
}
