package tui

import (
	"bytes"
	"fmt"
	"os"
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

type previewTickMsg struct {
	gen  int
	path string
}

type previewDoneMsg struct {
	gen     int
	path    string
	content string
	meta    entryMeta
	err     error
}

type previewEntry struct {
	content string
	meta    entryMeta
	err     error
}

// entryMeta is everything the header line and detail column show about
// the selected entry, gathered once per preview render so displaying it
// costs no extra I/O.
type entryMeta struct {
	name     string
	width    int
	height   int
	frames   int
	length   time.Duration
	source   string
	filter   bool
	colored  bool
	complex  bool
	ramp     string
	path     string
	fileSize int64
	modTime  time.Time
}

// buildEntryMeta collects the animation's stored facts plus the on-disk
// size and mtime of its .frames file.
func buildEntryMeta(anim *frames.Animation, name, path string) entryMeta {
	var length time.Duration
	for _, d := range anim.Delays {
		length += d
	}
	m := entryMeta{
		name:    name,
		width:   anim.Width,
		height:  anim.Height,
		frames:  len(anim.Frames),
		length:  length,
		source:  anim.SourceName,
		filter:  anim.FilterBackground,
		colored: anim.Colored,
		complex: anim.Complex,
		ramp:    anim.CustomRamp,
		path:    path,
	}
	if info, err := os.Stat(path); err == nil {
		m.fileSize = info.Size()
		m.modTime = info.ModTime()
	}
	return m
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

// setSize sets the preview's content area and reports whether the size
// actually changed. A size change invalidates the cache, since cached
// previews are rendered to fit the old dimensions; the caller is
// responsible for forcing a fresh render of the current selection when
// that happens.
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
		p.cache[msg.path] = previewEntry{content: msg.content, meta: msg.meta, err: msg.err}
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
		content := renderPreviewContent(anim, width, height, st)
		return previewDoneMsg{gen: gen, path: path, content: content, meta: buildEntryMeta(anim, name, path)}
	}
}

// currentMeta returns the loaded metadata for the current selection,
// reporting false while nothing is selected or the render is pending.
func (p previewModel) currentMeta() (entryMeta, bool) {
	if p.path == "" {
		return entryMeta{}, false
	}
	entry, ok := p.cache[p.path]
	if !ok || entry.err != nil {
		return entryMeta{}, false
	}
	return entry.meta, true
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

// renderPreviewContent builds the preview column's frame area: the
// first frame centered in exactly width x height cells; the metadata
// that used to sit under the frame now lives in the header line and the
// detail column.
func renderPreviewContent(anim *frames.Animation, width, height int, st styles) string {
	frameW := max(1, width)
	frameH := max(1, height)

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

	return lipgloss.Place(frameW, frameH, lipgloss.Center, lipgloss.Center, frame)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
