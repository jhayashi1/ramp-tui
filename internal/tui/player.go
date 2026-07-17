package tui

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jhayashi1/ascii-tui/internal/config"
	"github.com/jhayashi1/ascii-tui/internal/engine"
	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
)

// refitDebounce is how long the player waits after a resize before
// re-rendering, so resize storms trigger a single render.
const refitDebounce = 250 * time.Millisecond

// Speed is adjusted in 0.25 increments and clamped to this range.
const (
	minSpeed  = 0.25
	maxSpeed  = 8.0
	speedStep = 0.25
)

// minTickDelay floors the scaled frame delay so a high speed multiplier
// can't schedule an effectively-immediate, CPU-spinning tick loop.
const minTickDelay = 10 * time.Millisecond

// A held arrow accelerates: seekRun counts consecutive same-direction
// presses arriving within seekHoldWindow (longer than the OS repeat
// interval, shorter than a deliberate re-tap).
const seekHoldWindow = 150 * time.Millisecond

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
	speed   float64
	// elapsed[i] is the source-time position at which frame i starts;
	// a prefix sum over anim.Delays, rebuilt whenever anim is loaded.
	elapsed []time.Duration
	err     error
	st      styles
	keys    playerKeyMap
	bar     progress.Model
	width   int
	height  int
	// refitGen invalidates pending debounce timers and in-flight
	// re-renders whenever the viewport or entry changes again.
	refitGen  int
	refitting bool
	// Arrow-seek hold tracking. Because Bubble Tea v1 has no key-repeat
	// event, a held arrow is inferred from the OS auto-repeat stream:
	// consecutive same-direction seeks within seekHoldWindow grow seekRun,
	// which accelerates the per-press step.
	seekDir  int
	seekRun  int
	seekLast time.Time
}

func newPlayer(entries []library.Entry, index int, st styles, speed float64, keys config.Keys) (playerModel, tea.Cmd) {
	if speed <= 0 {
		speed = 1
	}
	p := playerModel{
		entries: entries,
		index:   index,
		speed:   clampSpeed(speed),
		st:      st,
		keys:    newPlayerKeyMap(keys),
		bar:     progress.New(progress.WithSolidFill(st.theme.Accent)),
	}
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
	p.buildElapsed()
}

// buildElapsed recomputes the prefix-sum start times used by the HUD's
// progress bar and elapsed/total display.
func (p *playerModel) buildElapsed() {
	p.elapsed = make([]time.Duration, len(p.anim.Delays))
	var sum time.Duration
	for i, d := range p.anim.Delays {
		p.elapsed[i] = sum
		sum += d
	}
}

// totalDuration is the full source-time loop length.
func (p playerModel) totalDuration() time.Duration {
	if p.anim == nil || len(p.anim.Delays) == 0 {
		return 0
	}
	return p.elapsed[len(p.elapsed)-1] + p.anim.Delays[len(p.anim.Delays)-1]
}

// seekFrames steps by an accelerating number of frames in direction dir
// (-1 or +1) and pauses. Holding the arrow produces an OS auto-repeat
// stream; same-direction presses within seekHoldWindow grow seekRun so
// the step size ramps up, while a gap or direction change resets it.
func (p *playerModel) seekFrames(dir int, now time.Time) {
	if dir == p.seekDir && now.Sub(p.seekLast) <= seekHoldWindow {
		p.seekRun++
	} else {
		p.seekRun = 0
	}
	p.seekDir = dir
	p.seekLast = now
	p.stepFrame(dir * seekStep(p.seekRun))
}

// seekStep maps how long the arrow has been held (in repeat count) to a
// per-press frame step: fine control on a tap, faster on a sustained hold.
func seekStep(run int) int {
	switch {
	case run < 8:
		return 1
	case run < 16:
		return 2
	case run < 24:
		return 4
	default:
		return 8
	}
}

// stepFrame moves by delta frames and pauses, since single-stepping
// while still auto-advancing would be confusing.
func (p *playerModel) stepFrame(delta int) {
	if p.anim == nil {
		return
	}
	n := len(p.anim.Frames)
	if n == 0 {
		return
	}
	p.frame = ((p.frame+delta)%n + n) % n
	p.paused = true
	p.gen++
}

// adjustSpeed adds delta to the current speed, clamps it, and bumps
// gen so the caller's follow-up tickCmd reschedules at the new rate
// immediately rather than waiting out the current frame's old delay.
func (p *playerModel) adjustSpeed(delta float64) {
	p.speed = clampSpeed(p.speed + delta)
	p.gen++
}

func clampSpeed(s float64) float64 {
	switch {
	case s < minSpeed:
		return minSpeed
	case s > maxSpeed:
		return maxSpeed
	default:
		return s
	}
}

func (p *playerModel) setSize(width, height int) {
	p.width, p.height = width, height
	p.bar.Width = max(1, width)
}

// viewport returns the area available for the animation, reserving one
// row for the progress bar and one for the status line.
func (p playerModel) viewport() (w, h int) {
	return p.width, p.height - 2
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
// current viewport, using the options it was originally rendered with,
// and saves the result back to the library so an entry whose stored
// size no longer matches the viewport (e.g. after a terminal resize, or
// a change to how much space the UI reserves) only needs to refit once
// rather than on every future play.
func (p playerModel) refitCmd() tea.Cmd {
	gen := p.refitGen
	anim := p.anim
	path := p.entries[p.index].Path
	opts := p.renderOptions()
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
	delay := scaleDelay(p.anim.Delays[p.frame], p.speed)
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return frameTickMsg{gen: gen}
	})
}

// scaleDelay applies the playback speed to a frame delay, flooring the
// result so a high multiplier can't spin the tick loop.
func scaleDelay(d time.Duration, speed float64) time.Duration {
	if speed <= 0 {
		speed = 1
	}
	scaled := time.Duration(float64(d) / speed)
	if scaled < minTickDelay {
		return minTickDelay
	}
	return scaled
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
		switch {
		case key.Matches(msg, p.keys.Quit):
			return p, tea.Quit
		case key.Matches(msg, p.keys.Back):
			return p, func() tea.Msg { return backToGalleryMsg{} }
		case key.Matches(msg, p.keys.Help):
			return p, func() tea.Msg { return toggleHelpMsg{} }
		case key.Matches(msg, p.keys.Pause):
			p.paused = !p.paused
			p.gen++
			return p, p.tickCmd()
		case key.Matches(msg, p.keys.SeekBack):
			p.seekFrames(-1, time.Now())
			return p, nil
		case key.Matches(msg, p.keys.SeekForward):
			p.seekFrames(1, time.Now())
			return p, nil
		case key.Matches(msg, p.keys.StepBack):
			p.stepFrame(-1)
			return p, nil
		case key.Matches(msg, p.keys.StepForward):
			p.stepFrame(1)
			return p, nil
		case key.Matches(msg, p.keys.SpeedUp):
			p.adjustSpeed(speedStep)
			return p, p.tickCmd()
		case key.Matches(msg, p.keys.SpeedDown):
			p.adjustSpeed(-speedStep)
			return p, p.tickCmd()
		case key.Matches(msg, p.keys.Prev):
			return p.switchTo(p.index - 1)
		case key.Matches(msg, p.keys.Next):
			return p.switchTo(p.index + 1)
		case key.Matches(msg, p.keys.Filter):
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
		return lipgloss.Place(max(1, p.width), max(1, p.height-1), lipgloss.Center, lipgloss.Center,
			p.st.status.Render(fmt.Sprintf("error: %v", p.err))) + "\n" +
			renderStatusBar(p.st.chipAlert, "ERROR", "esc back", p.st.help, "", p.width, p.st)
	}
	if p.anim == nil {
		return ""
	}
	vw, vh := p.viewport()
	if p.anim.Width > vw || p.anim.Height > vh {
		// The frame cannot be placed without breaking layout. Resizable
		// entries are about to be refitted; legacy ones can only ask
		// the user for more room.
		body := p.st.dim.Render(fmt.Sprintf("fitting to %dx%d...", vw, vh))
		if p.anim.SourceGIF == nil {
			body = p.st.warning.Render(fmt.Sprintf(
				"animation is %dx%d but the terminal is %dx%d;\nenlarge the window or re-render with a smaller --width",
				p.anim.Width, p.anim.Height, vw, vh))
		}
		return lipgloss.Place(max(1, p.width), max(1, p.height-1), lipgloss.Center, lipgloss.Center, body) +
			"\n" + p.statusBar()
	}

	var b strings.Builder
	b.WriteString(lipgloss.Place(vw, max(0, vh), lipgloss.Center, lipgloss.Center,
		p.anim.Frames[p.frame]))
	b.WriteByte('\n')
	b.WriteString(p.progressRow())
	b.WriteByte('\n')
	b.WriteString(p.statusBar())
	return b.String()
}

func (p playerModel) progressRow() string {
	total := p.totalDuration()
	var frac float64
	if total > 0 {
		frac = float64(p.elapsed[p.frame]) / float64(total)
	}
	return p.bar.ViewAs(frac)
}

// statusBar renders the player footer: a PLAYING/PAUSED mode chip, the
// key hints in the middle, and "name · frame/total · elapsed/total ·
// speed" right-aligned. renderStatusBar truncates the hints first as
// the terminal narrows, keeping the playback facts visible.
func (p playerModel) statusBar() string {
	chipLabel := "PLAYING"
	if p.paused {
		chipLabel = "PAUSED"
	}
	status := fmt.Sprintf("%s · %d/%d · %s/%s",
		p.entries[p.index].Name, p.frame+1, len(p.anim.Frames),
		formatDuration(p.elapsed[p.frame]), formatDuration(p.totalDuration()))
	if p.speed != 1 {
		status += " · " + formatSpeed(p.speed)
	}
	if p.refitting {
		status += " · fitting..."
	}
	return renderStatusBar(p.st.chip, chipLabel, shortHelpLine(p.keys.ShortHelp()), p.st.help, status, p.width, p.st)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d / time.Minute)
	s := int(d%time.Minute) / int(time.Second)
	return fmt.Sprintf("%02d:%02d", m, s)
}

func formatSpeed(speed float64) string {
	s := strconv.FormatFloat(speed, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return s + "x"
}
