package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// theme is a flat set of named colors. The foreground colors accept any
// value lipgloss.Color does (ANSI index, hex, name); Bg additionally
// feeds a raw background escape, which supports ANSI indexes and hex.
type theme struct {
	Accent    string
	AccentAlt string
	Border    string
	Text      string
	Dim       string
	Error     string
	Bg        string
	SelBg     string
	ChipText  string
}

// defaultTheme keeps the app's pink accent on a dark slate background.
func defaultTheme() theme {
	return theme{
		Accent:    "212",
		AccentAlt: "179",
		Border:    "240",
		Text:      "252",
		Dim:       "243",
		Error:     "203",
		Bg:        "234",
		SelBg:     "237",
		ChipText:  "234",
	}
}

// styles is the bundle of lipgloss styles derived once from a theme and
// plumbed into every sub-model, replacing package-level style vars.
type styles struct {
	theme       theme
	help        lipgloss.Style
	status      lipgloss.Style
	selected    lipgloss.Style
	text        lipgloss.Style
	dim         lipgloss.Style
	accent      lipgloss.Style
	warning     lipgloss.Style
	prompt      lipgloss.Style
	columnTitle lipgloss.Style
	sectionHead lipgloss.Style
	rule        lipgloss.Style
	headerName  lipgloss.Style
	metaKey     lipgloss.Style
	metaValue   lipgloss.Style
	chip        lipgloss.Style
	chipAlert   lipgloss.Style
	selBar      lipgloss.Style
	selBarText  lipgloss.Style
}

func newStyles(t theme) styles {
	return styles{
		theme:       t,
		help:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim)),
		status:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)),
		selected:    lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Bold(true),
		text:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text)),
		dim:         lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim)),
		accent:      lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)),
		warning:     lipgloss.NewStyle().Foreground(lipgloss.Color(t.Error)).Bold(true),
		prompt:      lipgloss.NewStyle().Padding(0, 2),
		columnTitle: lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim)).Bold(true),
		sectionHead: lipgloss.NewStyle().Foreground(lipgloss.Color(t.AccentAlt)).Bold(true),
		rule:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.Border)),
		headerName:  lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text)).Bold(true),
		metaKey:     lipgloss.NewStyle().Foreground(lipgloss.Color(t.Dim)),
		metaValue:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.Text)),
		chip:        lipgloss.NewStyle().Foreground(lipgloss.Color(t.ChipText)).Background(lipgloss.Color(t.Text)).Bold(true),
		chipAlert:   lipgloss.NewStyle().Foreground(lipgloss.Color(t.ChipText)).Background(lipgloss.Color(t.Error)).Bold(true),
		selBar:      lipgloss.NewStyle().Background(lipgloss.Color(t.SelBg)),
		selBarText:  lipgloss.NewStyle().Foreground(lipgloss.Color(t.Accent)).Background(lipgloss.Color(t.SelBg)).Bold(true),
	}
}

// defaultStyles is a convenience for construction sites (and tests) that
// have no theme yet.
func defaultStyles() styles { return newStyles(defaultTheme()) }

// renderColumn lays out one borderless column: a dim uppercase title
// row, a blank spacer, then the content rows, each padded/clipped so the
// result is exactly width x height. An empty title means the caller
// supplies its own first rows (the preview column's header line).
func renderColumn(title, content string, width, height int, st styles) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	var rows []string
	if title != "" {
		rows = append(rows, st.columnTitle.Render(truncateLabel(strings.ToUpper(title), width)), "")
	}
	rows = append(rows, strings.Split(content, "\n")...)

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		line := ""
		if i < len(rows) {
			line = rows[i]
		}
		b.WriteString(fitLine(line, width))
	}
	return b.String()
}

// headerLine builds the preview column's top row: an accent glyph, the
// bold entry name, and dim metadata, truncated to fit width. Segments
// are truncated as plain text before styling so the ANSI width math in
// fitLine stays exact.
func headerLine(glyph, name, meta string, width int, st styles) string {
	if width <= 0 {
		return ""
	}
	gw := lipgloss.Width(glyph)
	name = truncateLabel(name, max(0, width-gw-1))
	rest := width - gw - 1 - lipgloss.Width(name)
	metaPart := ""
	if meta != "" && rest > 4 {
		metaPart = "  " + truncateLabel(meta, rest-2)
	}
	return st.accent.Render(glyph) + " " + st.headerName.Render(name) + st.dim.Render(metaPart)
}

// sectionRule renders "LABEL ────" to at most width columns: an
// uppercase gold label with a dim rule filling the remainder.
func sectionRule(label string, width int, st styles) string {
	if width <= 0 {
		return ""
	}
	label = truncateLabel(strings.ToUpper(label), width)
	fill := width - lipgloss.Width(label) - 1
	if fill < 1 {
		return st.sectionHead.Render(label)
	}
	return st.sectionHead.Render(label) + " " + st.rule.Render(strings.Repeat("─", fill))
}

// kvKeyWidth is the key column of kvRow; it fits the longest detail
// key, "modified".
const kvKeyWidth = 8

// kvRow renders a "key   value" detail row with the key padded to
// kvKeyWidth columns and the value truncated so the row fits width.
func kvRow(k, v string, width int, st styles) string {
	k = truncateLabel(k, min(kvKeyWidth, width))
	pad := max(0, kvKeyWidth-lipgloss.Width(k))
	v = truncateLabel(v, max(0, width-kvKeyWidth-1))
	return st.metaKey.Render(k) + strings.Repeat(" ", pad+1) + st.metaValue.Render(v)
}

// renderStatusBar renders the single-line footer: an inverted mode chip
// on the left, middle text (key hints or a status message), and a
// right-aligned dim status, exactly width columns. When space runs out
// the middle is truncated first, then the right status; the chip always
// survives.
func renderStatusBar(chipStyle lipgloss.Style, chipLabel, middle string, middleStyle lipgloss.Style, status string, width int, st styles) string {
	if width <= 0 {
		return ""
	}
	chipText := truncateLabel(" "+chipLabel+" ", width)
	chip := chipStyle.Render(chipText)
	avail := width - lipgloss.Width(chipText) - 1
	if avail <= 0 {
		return fitLine(chip, width)
	}

	status = truncateLabel(status, avail)
	statusW := lipgloss.Width(status)
	middleMax := avail - statusW
	if statusW > 0 {
		middleMax -= 2
	}
	middle = truncateLabel(middle, max(0, middleMax))
	gap := avail - lipgloss.Width(middle) - statusW

	return chip + " " + middleStyle.Render(middle) +
		strings.Repeat(" ", max(0, gap)) + st.dim.Render(status)
}

// ansiBgSequence converts a color value (256 index or #rrggbb hex) to a
// raw SGR background sequence; any other value (including empty)
// disables background painting.
func ansiBgSequence(color string) string {
	if n, err := strconv.Atoi(color); err == nil && n >= 0 && n <= 255 {
		return fmt.Sprintf("\x1b[48;5;%dm", n)
	}
	if len(color) == 7 && color[0] == '#' {
		if v, err := strconv.ParseUint(color[1:], 16, 32); err == nil {
			return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", (v>>16)&0xff, (v>>8)&0xff, v&0xff)
		}
	}
	return ""
}

// paintBackground floods a composed screen with the theme background:
// every line is padded to width, prefixed with the bg sequence, and the
// bg is re-injected after every SGR reset so content carrying its own
// ANSI (frame art, bubbles components) doesn't punch holes in the fill.
// Rows are added up to height so the fill spans the whole screen.
func paintBackground(view string, width, height int, st styles) string {
	bg := ansiBgSequence(st.theme.Bg)
	if bg == "" || width <= 0 {
		return view
	}
	lines := strings.Split(view, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		line = fitLine(line, width)
		line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+bg)
		line = strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+bg)
		lines[i] = bg + line + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

// fitLine pads or truncates a (possibly ANSI-styled) line to exactly
// width display columns.
func fitLine(line string, width int) string {
	w := lipgloss.Width(line)
	if w == width {
		return line
	}
	if w < width {
		return line + strings.Repeat(" ", width-w)
	}
	return truncateLabel(line, width)
}

// truncateLabel trims a string to at most width display columns. It is
// ANSI-aware via lipgloss.Width but assumes styling does not span the
// cut point; every chrome primitive truncates plain text before
// styling, so this holds.
func truncateLabel(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	for i := range runes {
		if lipgloss.Width(string(runes[:i+1])) > width {
			return string(runes[:i])
		}
	}
	return s
}
