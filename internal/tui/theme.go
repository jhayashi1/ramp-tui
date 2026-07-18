package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jhayashi1/ramp-tui/internal/config"
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

// namedTheme is a preset the gallery can cycle through with "t"; name is
// stored in the config so cycling resumes where it left off across runs.
type namedTheme struct {
	name  string
	theme theme
}

// themePresets are the built-in palettes, in cycle order. All share the
// same dark slate background and neutral text/error colors, differing in
// their accent so switching reads as a deliberate mood change. The first
// entry is the default and must match config.Defaults().Theme.
var themePresets = []namedTheme{
	{"pink", theme{Accent: "212", AccentAlt: "179", Border: "240", Text: "252", Dim: "243", Error: "203", Bg: "234", SelBg: "237", ChipText: "234"}},
	{"matrix", theme{Accent: "46", AccentAlt: "120", Border: "238", Text: "252", Dim: "243", Error: "203", Bg: "234", SelBg: "236", ChipText: "234"}},
	{"amber", theme{Accent: "214", AccentAlt: "179", Border: "240", Text: "223", Dim: "243", Error: "203", Bg: "234", SelBg: "237", ChipText: "234"}},
	{"ocean", theme{Accent: "39", AccentAlt: "45", Border: "240", Text: "252", Dim: "243", Error: "203", Bg: "234", SelBg: "237", ChipText: "234"}},
}

// defaultTheme keeps the app's pink accent on a dark slate background.
func defaultTheme() theme { return themePresets[0].theme }

// themeIndexByName returns the preset index for a stored theme name, or
// -1 when the name is empty or names no built-in preset (a custom
// config). The first "t" press then advances to preset 0.
func themeIndexByName(name string) int {
	for i, p := range themePresets {
		if p.name == name {
			return i
		}
	}
	return -1
}

// themeFromConfig maps the flat config colors onto the tui theme; the two
// structs are parallel but name their selection-background field
// differently.
func themeFromConfig(t config.Theme) theme {
	return theme{
		Accent:    t.Accent,
		AccentAlt: t.AccentAlt,
		Border:    t.Border,
		Text:      t.Text,
		Dim:       t.Dim,
		Error:     t.Error,
		Bg:        t.Bg,
		SelBg:     t.SelectionBg,
		ChipText:  t.ChipText,
	}
}

// configTheme is themeFromConfig's inverse: it flattens a preset back
// into the serializable config shape, tagging it with the preset name so
// the switcher resumes cycling from here next run.
func (n namedTheme) configTheme() config.Theme {
	return config.Theme{
		Name:        n.name,
		Accent:      n.theme.Accent,
		AccentAlt:   n.theme.AccentAlt,
		Border:      n.theme.Border,
		Text:        n.theme.Text,
		Dim:         n.theme.Dim,
		Error:       n.theme.Error,
		Bg:          n.theme.Bg,
		SelectionBg: n.theme.SelBg,
		ChipText:    n.theme.ChipText,
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

// brandIcon and brandName form the app wordmark shown right-aligned in
// the header line; the icon is the luminance ramp the engine draws with.
const (
	brandIcon = "░▒▓"
	brandName = "ramp"
)

// headerLine builds the preview column's top row: an accent "▸" and the
// bold entry name on the left, the app wordmark right-aligned. An
// empty name leaves just the wordmark; the wordmark is dropped first
// when width runs out. Segments are truncated as plain text before
// styling so the ANSI width math in fitLine stays exact.
func headerLine(name string, width int, st styles) string {
	if width <= 0 {
		return ""
	}
	left, leftW := "", 0
	if name != "" {
		name = truncateLabel(name, max(0, width-2))
		left = st.accent.Render("▸") + " " + st.headerName.Render(name)
		leftW = 2 + lipgloss.Width(name)
	}
	gap := width - leftW - lipgloss.Width(brandIcon) - 1 - lipgloss.Width(brandName)
	if gap < 2 {
		return left
	}
	return left + strings.Repeat(" ", gap) +
		st.accent.Render(brandIcon) + " " + st.headerName.Render(brandName)
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
