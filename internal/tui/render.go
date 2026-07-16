package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	dir              string
	gifPath          string
	bar              progress.Model
	events           chan renderEventMsg
	percent          float64
	st               styles
	filterBackground bool
	complex          bool
	width            int
	height           int
}

func newRender(dir, gifPath string, st styles, filterBackground, useComplex bool) renderModel {
	return renderModel{
		dir:              dir,
		gifPath:          gifPath,
		bar:              progress.New(progress.WithSolidFill(st.theme.Accent)),
		events:           make(chan renderEventMsg, 16),
		st:               st,
		filterBackground: filterBackground,
		complex:          useComplex,
	}
}

func (r *renderModel) setSize(width, height int) {
	r.width, r.height = width, height
	r.bar.Width = max(10, min(60, width-10))
}

// start kicks off the render in the background and begins pumping its
// progress events into the update loop.
func (r renderModel) start() tea.Cmd {
	return tea.Batch(r.runRender(), r.nextEvent())
}

func (r renderModel) runRender() tea.Cmd {
	return func() tea.Msg {
		data, err := os.ReadFile(r.gifPath)
		if err != nil {
			r.events <- renderEventMsg{finished: true, err: err}
			return nil
		}

		// Size the render to the TUI's own viewport (minus the progress
		// bar and status rows) so the player's fit check always agrees.
		opts := engine.Options{
			Colored:          true,
			FilterBackground: r.filterBackground,
			Complex:          r.complex,
			MaxWidth:         r.width,
			MaxHeight:        max(1, r.height-2),
		}
		anim, err := engine.Render(bytes.NewReader(data), opts, func(done, total int) {
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
		anim.SourceGIF = data

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
	body := r.st.text.Render(fmt.Sprintf("rendering %s", filepath.Base(r.gifPath))) +
		"\n\n" + r.bar.ViewAs(r.percent)
	return lipgloss.Place(max(1, r.width), max(1, r.height-1), lipgloss.Center, lipgloss.Center, body) +
		"\n" + renderStatusBar(r.st.chip, "RENDER", "", r.st.help, "", r.width, r.st)
}
