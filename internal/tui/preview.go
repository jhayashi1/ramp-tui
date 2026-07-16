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

// previewDebounce mirrors the player's refitDebounce: it keeps a held
// arrow key from triggering a decode+render per keystroke.
const previewDebounce = 150 * time.Millisecond

// previewMetaRows is the number of metadata lines drawn under the frame
// (name, dimensions/frame count, source/filter state).
const previewMetaRows = 3

type previewTickMsg struct {
	gen  int
	path string
}

type previewDoneMsg struct {
	gen     int
	path    string
	content string
	err     error
}

type previewEntry struct {
	content string
	err     error
}

// previewModel renders a cheap first-frame preview of the selected
// gallery entry, debounced and cached the same way the player debounces
// resize-triggered re-renders (see refitGen in player.go).
type previewModel struct {
	st     styles
	cache  map[string]previewEntry
	path   string
	name   string
	gen    int
	width  int
	height int
}

func newPreviewModel(st styles) previewModel {
	return previewModel{st: st, cache: make(map[string]previewEntry)}
}

// setSize sets the preview's content area (already excluding panel
// borders) and reports whether the size actually changed. A size change
// invalidates the cache, since cached previews are rendered to fit the
// old dimensions; the caller is responsible for forcing a fresh render
// of the current selection when that happens.
func (p *previewModel) setSize(width, height int) bool {
	changed := width != p.width || height != p.height
	if changed {
		p.cache = make(map[string]previewEntry)
	}
	p.width, p.height = width, height
	return changed
}

// selectEntry starts (or restarts) the debounce timer for a newly
// selected entry. Cached entries render immediately; nothing is
// scheduled when there is no entry.
func (p *previewModel) selectEntry(entry library.Entry) tea.Cmd {
	p.path, p.name = entry.Path, entry.Name
	p.gen++
	if entry.Path == "" {
		return nil
	}
	if _, ok := p.cache[entry.Path]; ok {
		return nil
	}
	gen, path := p.gen, entry.Path
	return tea.Tick(previewDebounce, func(time.Time) tea.Msg {
		return previewTickMsg{gen: gen, path: path}
	})
}

// reset clears the cache and the current selection, so that after e.g.
// a reload() the next reconcilePreview reselects and re-renders from
// scratch instead of assuming the still-selected path is unchanged.
func (p *previewModel) reset() {
	p.cache = make(map[string]previewEntry)
	p.clear()
}

// clear drops the current selection, e.g. when the library is empty.
func (p *previewModel) clear() {
	p.path, p.name = "", ""
	p.gen++
}

func (p previewModel) update(msg tea.Msg) (previewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case previewTickMsg:
		if msg.gen != p.gen {
			return p, nil
		}
		return p, p.renderCmd(msg.gen, msg.path, p.name)

	case previewDoneMsg:
		if msg.gen != p.gen {
			return p, nil
		}
		p.cache[msg.path] = previewEntry{content: msg.content, err: msg.err}
	}
	return p, nil
}

// renderCmd loads the entry and renders it in the background, sized to
// the preview area as of the call (the debounce delay is long enough
// that a just-landed resize has already updated width/height).
func (p previewModel) renderCmd(gen int, path, name string) tea.Cmd {
	width, height, st := p.width, p.height, p.st
	return func() tea.Msg {
		anim, err := library.Load(path)
		if err != nil {
			return previewDoneMsg{gen: gen, path: path, err: err}
		}
		content := renderPreviewContent(anim, name, width, height, st)
		return previewDoneMsg{gen: gen, path: path, content: content}
	}
}

func (p previewModel) view() string {
	if p.path == "" {
		return p.st.dim.Render("no animations yet")
	}
	entry, ok := p.cache[p.path]
	if !ok {
		return p.st.dim.Render("loading preview...")
	}
	if entry.err != nil {
		return p.st.status.Render(fmt.Sprintf("preview failed: %v", entry.err))
	}
	return entry.content
}

// renderPreviewContent builds the preview panel's interior: the first
// frame centered above metadata, padded/clipped to exactly width x
// height lines by the caller's renderPanel.
func renderPreviewContent(anim *frames.Animation, name string, width, height int, st styles) string {
	meta := strings.Join([]string{
		st.selected.Render(name),
		st.dim.Render(fmt.Sprintf("%dx%d - %d frames", anim.Width, anim.Height, len(anim.Frames))),
		st.dim.Render(fmt.Sprintf("source: %s  filter: %s", anim.SourceName, onOff(anim.FilterBackground))),
	}, "\n")

	frameW := max(1, width)
	frameH := max(1, height-previewMetaRows-1)

	var frame string
	switch {
	case anim.SourceGIF != nil:
		text, _, _, err := engine.RenderPreview(bytes.NewReader(anim.SourceGIF), engine.Options{
			Colored:          true,
			FilterBackground: anim.FilterBackground,
			MaxWidth:         frameW,
			MaxHeight:        frameH,
		})
		if err != nil {
			frame = st.status.Render(fmt.Sprintf("preview failed: %v", err))
		} else {
			frame = text
		}
	case len(anim.Frames) > 0 && anim.Width <= frameW && anim.Height <= frameH:
		frame = anim.Frames[0]
	default:
		frame = st.dim.Render("no preview available")
	}

	placed := lipgloss.Place(frameW, frameH, lipgloss.Center, lipgloss.Center, frame)
	return placed + "\n\n" + meta
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
