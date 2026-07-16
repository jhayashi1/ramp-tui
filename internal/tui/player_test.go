package tui

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	p, _ := newPlayer([]library.Entry{{Name: "test", Path: path}}, 0, defaultStyles(), 1)
	return p
}

func TestPlayerSeekByMovesForward(t *testing.T) {
	p := newTestPlayer(t, []time.Duration{time.Second, time.Second, time.Second, time.Second})
	p.seekBy(2500 * time.Millisecond)
	if p.frame != 2 {
		t.Errorf("frame = %d, want 2", p.frame)
	}
}

func TestPlayerSeekByWrapsForward(t *testing.T) {
	p := newTestPlayer(t, []time.Duration{time.Second, time.Second, time.Second, time.Second})
	p.frame = 3 // elapsed = 3s, total = 4s
	p.seekBy(2 * time.Second)
	if p.frame != 1 {
		t.Errorf("frame = %d, want 1 (wrapped past the loop end)", p.frame)
	}
}

func TestPlayerSeekByWrapsBackward(t *testing.T) {
	p := newTestPlayer(t, []time.Duration{time.Second, time.Second, time.Second, time.Second})
	p.seekBy(-1500 * time.Millisecond)
	if p.frame != 2 {
		t.Errorf("frame = %d, want 2 (wrapped before the loop start)", p.frame)
	}
}

func TestPlayerSeekByPreservesPauseState(t *testing.T) {
	p := newTestPlayer(t, []time.Duration{time.Second, time.Second})
	p.paused = true
	genBefore := p.gen
	p.seekBy(time.Second)
	if !p.paused {
		t.Error("seekBy unpaused the player")
	}
	if p.gen == genBefore {
		t.Error("seekBy did not bump gen")
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

func TestPlayerSeekKeyKeepsPlaying(t *testing.T) {
	m := step(t, fixtureModel(t), tea.KeyMsg{Type: tea.KeyEnter})
	before := m.player.frame
	m = step(t, m, keyRune('l'))
	if m.player.paused {
		t.Error("seeking forward paused playback")
	}
	if m.player.frame == before && m.player.totalDuration() > time.Second {
		t.Error("seek key did not move the frame")
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
