package tui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jhayashi1/ascii-tui/internal/config"
	"github.com/jhayashi1/ascii-tui/internal/frames"
	"github.com/jhayashi1/ascii-tui/internal/library"
)

// newTestPlayer builds a playerModel over a single entry with the given
// per-frame delays, for exercising seek/step/speed math directly.
func newTestPlayer(t *testing.T, delays []time.Duration) playerModel {
	t.Helper()
	dir := t.TempDir()
	anim := &frames.Animation{
		SourceName: "test.gif",
		Width:      2,
		Height:     2,
		Frames:     make([]string, len(delays)),
		Delays:     delays,
	}
	for i := range anim.Frames {
		anim.Frames[i] = fmt.Sprintf("f%d", i)
	}
	path, err := library.Save(dir, anim)
	if err != nil {
		t.Fatalf("saving fixture: %v", err)
	}
	p, _ := newPlayer([]library.Entry{{Name: "test", Path: path}}, 0, defaultStyles(), 1, config.DefaultKeys())
	return p
}

func TestPlayerSeekFramesSingleTapStepsAndPauses(t *testing.T) {
	p := newTestPlayer(t, make([]time.Duration, 10))
	now := time.Now()
	p.seekFrames(1, now)
	if p.frame != 1 || !p.paused {
		t.Errorf("frame=%d paused=%v, want frame=1 paused=true", p.frame, p.paused)
	}
}

func TestPlayerSeekFramesDirectionChangeResets(t *testing.T) {
	p := newTestPlayer(t, make([]time.Duration, 32))
	now := time.Now()
	// A backward tap right after forward taps must step by 1, not by an
	// accelerated amount carried over from the forward run.
	for i := range 10 {
		p.seekFrames(1, now.Add(time.Duration(i)*10*time.Millisecond))
	}
	before := p.frame
	p.seekFrames(-1, now.Add(11*10*time.Millisecond))
	if p.frame != before-1 {
		t.Errorf("frame = %d, want %d (direction change should reset to a 1-frame step)", p.frame, before-1)
	}
}

func TestPlayerSeekFramesAcceleratesOnHold(t *testing.T) {
	// Enough frames that 16 accelerating steps never wrap the loop.
	p := newTestPlayer(t, make([]time.Duration, 512))
	now := time.Now()
	for i := range 16 {
		p.seekFrames(1, now.Add(time.Duration(i)*10*time.Millisecond))
	}
	// seekRun 0..7 step by 1 (8 frames), 8..15 step by 2 (16 frames).
	if want := 8*1 + 8*2; p.frame != want {
		t.Errorf("frame after 16 held presses = %d, want %d (acceleration ramp)", p.frame, want)
	}
}

func TestPlayerSeekFramesGapResets(t *testing.T) {
	p := newTestPlayer(t, make([]time.Duration, 32))
	now := time.Now()
	for i := range 10 {
		p.seekFrames(1, now.Add(time.Duration(i)*10*time.Millisecond))
	}
	before := p.frame
	// A press after the hold window is a fresh tap: step by 1.
	p.seekFrames(1, now.Add(10*10*time.Millisecond+2*seekHoldWindow))
	if p.frame != before+1 {
		t.Errorf("frame = %d, want %d (gap past seekHoldWindow should reset the run)", p.frame, before+1)
	}
}

func TestPlayerStepFramePausesAndWraps(t *testing.T) {
	p := newTestPlayer(t, []time.Duration{time.Second, time.Second, time.Second})
	p.stepFrame(1)
	if p.frame != 1 || !p.paused {
		t.Errorf("frame=%d paused=%v, want frame=1 paused=true", p.frame, p.paused)
	}
	p.stepFrame(-1)
	if p.frame != 0 {
		t.Errorf("frame = %d, want 0", p.frame)
	}
	p.stepFrame(-1)
	if p.frame != 2 {
		t.Errorf("frame = %d, want 2 (wrapped backward)", p.frame)
	}
}

func TestPlayerAdjustSpeedClampsRange(t *testing.T) {
	p := newTestPlayer(t, []time.Duration{time.Second})
	for range 20 {
		p.adjustSpeed(speedStep)
	}
	if p.speed != maxSpeed {
		t.Errorf("speed = %v, want clamped to %v", p.speed, maxSpeed)
	}
	for range 40 {
		p.adjustSpeed(1 / speedStep)
	}
	if p.speed != minSpeed {
		t.Errorf("speed = %v, want clamped to %v", p.speed, minSpeed)
	}
}

func TestScaleDelayFloorsAtMinTick(t *testing.T) {
	if got := scaleDelay(100*time.Millisecond, 100); got != minTickDelay {
		t.Errorf("scaleDelay = %v, want floor %v", got, minTickDelay)
	}
	if got := scaleDelay(time.Second, 1); got != time.Second {
		t.Errorf("scaleDelay at 1x = %v, want unchanged %v", got, time.Second)
	}
	if got := scaleDelay(time.Second, 2); got != 500*time.Millisecond {
		t.Errorf("scaleDelay at 2x = %v, want 500ms", got)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                "00:00",
		3 * time.Second:  "00:03",
		65 * time.Second: "01:05",
	}
	for d, want := range cases {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	cases := map[float64]string{
		1.5:  "1.5x",
		0.25: "0.25x",
		2:    "2x",
	}
	for s, want := range cases {
		if got := formatSpeed(s); got != want {
			t.Errorf("formatSpeed(%v) = %q, want %q", s, got, want)
		}
	}
}

func TestPlayerSeekKeyStepsAndPauses(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	before := m.player.frame
	m = step(t, m, keyRune('l'))
	if !m.player.paused {
		t.Error("seeking forward did not pause playback")
	}
	if got := m.player.frame; got != before+1 {
		t.Errorf("frame = %d, want %d (one-frame step)", got, before+1)
	}
}

func TestPlayerStepKeyPauses(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('.'))
	if !m.player.paused {
		t.Error("step-forward key did not pause playback")
	}
}

func TestPlayerSpeedKeysAdjustSpeed(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	m = step(t, m, keyRune('+'))
	if m.player.speed <= 1 {
		t.Errorf("speed after '+' = %v, want > 1", m.player.speed)
	}
	m = step(t, m, keyRune('-'))
	m = step(t, m, keyRune('-'))
	if m.player.speed >= 1 {
		t.Errorf("speed after '+' '-' '-' = %v, want < 1", m.player.speed)
	}
}
