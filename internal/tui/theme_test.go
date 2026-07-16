package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderColumnDimensions(t *testing.T) {
	st := defaultStyles()
	col := renderColumn("library", "line one\nline two", 24, 6, st)
	lines := strings.Split(col, "\n")
	if len(lines) != 6 {
		t.Fatalf("column has %d lines, want 6", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 24 {
			t.Errorf("line %d width = %d, want 24 (%q)", i, w, line)
		}
	}
	if !strings.Contains(lines[0], "LIBRARY") {
		t.Errorf("title row missing uppercase title: %q", lines[0])
	}
	if !strings.Contains(lines[2], "line one") {
		t.Errorf("content should start below the title and spacer: %q", lines[2])
	}
}

func TestRenderColumnEmptyTitleSkipsChrome(t *testing.T) {
	st := defaultStyles()
	col := renderColumn("", "first row", 20, 4, st)
	lines := strings.Split(col, "\n")
	if len(lines) != 4 {
		t.Fatalf("column has %d lines, want 4", len(lines))
	}
	if !strings.Contains(lines[0], "first row") {
		t.Errorf("titleless column should start with content: %q", lines[0])
	}
}

func TestRenderColumnClampsOverflow(t *testing.T) {
	st := defaultStyles()
	long := strings.Repeat("x", 100) + "\n" + strings.Repeat("y", 100)
	for i, line := range strings.Split(renderColumn("t", long, 10, 3, st), "\n") {
		if w := lipgloss.Width(line); w != 10 {
			t.Errorf("overflow line %d width = %d, want 10 (%q)", i, w, line)
		}
	}
}

func TestHeaderLineFitsWidth(t *testing.T) {
	st := defaultStyles()
	got := headerLine("▸", "bongo-cat", "80x24 · 42 frames · bongo.gif", 40, st)
	if w := lipgloss.Width(got); w > 40 {
		t.Errorf("header width = %d, want <= 40 (%q)", w, got)
	}
	if !strings.Contains(got, "bongo-cat") {
		t.Errorf("header missing name: %q", got)
	}

	tight := headerLine("▸", strings.Repeat("n", 50), "meta", 12, st)
	if w := lipgloss.Width(tight); w > 12 {
		t.Errorf("tight header width = %d, want <= 12 (%q)", w, tight)
	}
}

func TestSectionRuleExactWidth(t *testing.T) {
	st := defaultStyles()
	got := sectionRule("animations · 3", 30, st)
	if w := lipgloss.Width(got); w != 30 {
		t.Errorf("rule width = %d, want 30 (%q)", w, got)
	}
	if !strings.Contains(got, "ANIMATIONS · 3") {
		t.Errorf("rule label not uppercased: %q", got)
	}
	if !strings.Contains(got, "─") {
		t.Errorf("rule missing line fill: %q", got)
	}
}

func TestKvRowFitsWidth(t *testing.T) {
	st := defaultStyles()
	got := kvRow("modified", "2026-07-15 10:00", 26, st)
	if w := lipgloss.Width(got); w > 26 {
		t.Errorf("kv row width = %d, want <= 26 (%q)", w, got)
	}
	long := kvRow("name", strings.Repeat("v", 60), 20, st)
	if w := lipgloss.Width(long); w > 20 {
		t.Errorf("long kv row width = %d, want <= 20 (%q)", w, long)
	}
}

func TestRenderStatusBarExactWidth(t *testing.T) {
	st := defaultStyles()
	bar := renderStatusBar(st.chip, "NORMAL", "a add · d delete · q quit", st.help, "2 animations · ascii-tui", 80, st)
	if w := lipgloss.Width(bar); w != 80 {
		t.Errorf("status bar width = %d, want 80 (%q)", w, bar)
	}
	for _, part := range []string{"NORMAL", "a add", "ascii-tui"} {
		if !strings.Contains(bar, part) {
			t.Errorf("status bar missing %q: %q", part, bar)
		}
	}
}

func TestRenderStatusBarTruncatesMiddleFirst(t *testing.T) {
	st := defaultStyles()
	bar := renderStatusBar(st.chip, "NORMAL", strings.Repeat("hint ", 20), st.help, "3 · app", 30, st)
	if w := lipgloss.Width(bar); w != 30 {
		t.Errorf("narrow status bar width = %d, want 30 (%q)", w, bar)
	}
	if !strings.Contains(bar, "NORMAL") {
		t.Errorf("chip lost under pressure: %q", bar)
	}
	if !strings.Contains(bar, "3 · app") {
		t.Errorf("right status truncated before the middle hints: %q", bar)
	}
}

func TestRenderStatusBarChipSurvivesTinyWidth(t *testing.T) {
	st := defaultStyles()
	bar := renderStatusBar(st.chip, "NORMAL", "hints", st.help, "status", 6, st)
	if w := lipgloss.Width(bar); w != 6 {
		t.Errorf("tiny status bar width = %d, want 6 (%q)", w, bar)
	}
	if !strings.Contains(bar, "NORM") {
		t.Errorf("chip fully lost at tiny width: %q", bar)
	}
}

func TestAnsiBgSequence(t *testing.T) {
	cases := map[string]string{
		"234":     "\x1b[48;5;234m",
		"0":       "\x1b[48;5;0m",
		"#1a2b3c": "\x1b[48;2;26;43;60m",
		"999":     "",
		"banana":  "",
		"":        "",
	}
	for in, want := range cases {
		if got := ansiBgSequence(in); got != want {
			t.Errorf("ansiBgSequence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPaintBackgroundFillsAndReinjectsAfterResets(t *testing.T) {
	st := defaultStyles()
	bg := ansiBgSequence(st.theme.Bg)
	view := "plain\n\x1b[38;2;255;0;0mred\x1b[0m tail"
	got := paintBackground(view, 20, 3, st)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("painted view has %d lines, want 3 (missing rows should be added)", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 20 {
			t.Errorf("painted line %d width = %d, want 20 (%q)", i, w, line)
		}
		if !strings.HasPrefix(line, bg) {
			t.Errorf("painted line %d does not start with the bg sequence: %q", i, line)
		}
	}
	if !strings.Contains(lines[1], "\x1b[0m"+bg) {
		t.Errorf("bg not re-injected after an embedded reset: %q", lines[1])
	}
}

func TestPaintBackgroundNoopWithoutColor(t *testing.T) {
	st := newStyles(theme{})
	view := "unchanged"
	if got := paintBackground(view, 20, 2, st); got != view {
		t.Errorf("paintBackground without a bg color = %q, want unchanged input", got)
	}
}
