package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/jhayashi1/ascii-tui/internal/pathutil"
)

// maxVisibleSuggestions is the fixed height of the completion list shown
// under the path prompt.
const maxVisibleSuggestions = 6

var (
	suggestionFileStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	suggestionDirStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	suggestionMatchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	suggestionMarkerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	suggestionEmptyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type pathSuggestion struct {
	name string
	dir  bool
	// matches holds byte offsets into name that matched the fuzzy query.
	matches []int
}

// pathInput is a text input with fzf-style completion over the
// filesystem: it lists directories and .gif files in the directory
// portion of the typed path, fuzzy-filtered by the partial name after
// the last separator. A leading "~" expands to the home directory.
type pathInput struct {
	input textinput.Model
	// dirPrefix is the typed text up to and including the last path
	// separator; completions are appended to it.
	dirPrefix   string
	suggestions []pathSuggestion
	sel         int
	offset      int
}

func newPathInput() pathInput {
	input := textinput.New()
	input.Placeholder = "path/to/animation.gif"
	return pathInput{input: input}
}

func (p *pathInput) focus() tea.Cmd {
	p.input.SetValue("")
	p.refresh()
	return p.input.Focus()
}

func (p *pathInput) blur() { p.input.Blur() }

func (p *pathInput) setWidth(w int) { p.input.Width = w }

func (p pathInput) update(msg tea.Msg) (pathInput, tea.Cmd) {
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.refresh()
	}
	return p, cmd
}

// splitPathPrefix splits the typed value into the directory portion
// (including the trailing separator) and the partial name being
// completed. A bare "~" is treated as the home directory itself.
func splitPathPrefix(raw string) (prefix, partial string) {
	if i := strings.LastIndexAny(raw, `/\`); i >= 0 {
		return raw[:i+1], raw[i+1:]
	}
	if raw == "~" {
		return "~" + string(os.PathSeparator), ""
	}
	return "", raw
}

// refresh recomputes the completion candidates for the current input.
func (p *pathInput) refresh() {
	p.sel, p.offset = 0, 0
	prefix, partial := splitPathPrefix(p.input.Value())
	p.dirPrefix = prefix

	scanDir := pathutil.ExpandTilde(prefix)
	if scanDir == "" {
		scanDir = "."
	}
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		p.suggestions = nil
		return
	}

	var names []string
	isDir := make(map[string]bool, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(partial, ".") {
			continue
		}
		dir := e.IsDir()
		if !dir && e.Type()&os.ModeSymlink != 0 {
			if fi, err := os.Stat(filepath.Join(scanDir, name)); err == nil {
				dir = fi.IsDir()
			}
		}
		if !dir && !strings.EqualFold(filepath.Ext(name), ".gif") {
			continue
		}
		names = append(names, name)
		isDir[name] = dir
	}

	if partial == "" {
		sort.SliceStable(names, func(i, j int) bool {
			if isDir[names[i]] != isDir[names[j]] {
				return isDir[names[i]]
			}
			return strings.ToLower(names[i]) < strings.ToLower(names[j])
		})
		p.suggestions = make([]pathSuggestion, len(names))
		for i, name := range names {
			p.suggestions[i] = pathSuggestion{name: name, dir: isDir[name]}
		}
		return
	}

	matches := fuzzy.Find(partial, names)
	p.suggestions = make([]pathSuggestion, len(matches))
	for i, m := range matches {
		p.suggestions[i] = pathSuggestion{name: m.Str, dir: isDir[m.Str], matches: m.MatchedIndexes}
	}
}

func (p *pathInput) moveSelection(delta int) {
	n := len(p.suggestions)
	if n == 0 {
		return
	}
	p.sel = (p.sel + delta + n) % n
	if p.sel < p.offset {
		p.offset = p.sel
	}
	if p.sel >= p.offset+maxVisibleSuggestions {
		p.offset = p.sel - maxVisibleSuggestions + 1
	}
}

// complete fills the input with the selected suggestion. Completing a
// directory descends into it and re-scans; completing a gif leaves its
// full path in the input.
func (p *pathInput) complete() {
	if p.sel >= len(p.suggestions) {
		return
	}
	s := p.suggestions[p.sel]
	value := p.dirPrefix + s.name
	if s.dir {
		value += string(os.PathSeparator)
	}
	p.input.SetValue(value)
	p.input.CursorEnd()
	p.refresh()
}

// accept resolves enter: a selected directory is descended into (and
// ok is false), while a selected gif — or, with no suggestions, the
// literal typed text — yields the tilde-expanded path to render.
func (p *pathInput) accept() (path string, ok bool) {
	if len(p.suggestions) > 0 {
		dir := p.suggestions[p.sel].dir
		p.complete()
		if dir {
			return "", false
		}
		return pathutil.ExpandTilde(p.input.Value()), true
	}
	value := strings.TrimSpace(p.input.Value())
	if value == "" {
		return "", false
	}
	return pathutil.ExpandTilde(value), true
}

// view renders the input line plus a fixed maxVisibleSuggestions rows,
// padding with blanks so the overall height never changes.
func (p pathInput) view() string {
	var b strings.Builder
	b.WriteString(promptStyle.Render("render gif: " + p.input.View()))
	b.WriteByte('\n')
	end := min(p.offset+maxVisibleSuggestions, len(p.suggestions))
	for i := p.offset; i < end; i++ {
		b.WriteString(p.suggestionRow(i))
		b.WriteByte('\n')
	}
	if len(p.suggestions) == 0 && strings.TrimSpace(p.input.Value()) != "" {
		b.WriteString(suggestionEmptyStyle.Render("    no matching gifs or directories"))
		b.WriteByte('\n')
		end++
	}
	for i := end - p.offset; i < maxVisibleSuggestions; i++ {
		b.WriteByte('\n')
	}
	return b.String()
}

func (p pathInput) suggestionRow(i int) string {
	s := p.suggestions[i]
	marker := "    "
	if i == p.sel {
		marker = "  " + suggestionMarkerStyle.Render("▸ ")
	}
	base := suggestionFileStyle
	name := s.name
	if s.dir {
		base = suggestionDirStyle
		name += string(os.PathSeparator)
	}
	name = truncateRunes(name, max(4, p.input.Width-4))
	return marker + renderMatched(name, s.matches, base)
}

// renderMatched styles name rune by rune so the fuzzy-matched positions
// stand out, avoiding nested-style resets from wrapping the whole row.
func renderMatched(name string, matches []int, base lipgloss.Style) string {
	if len(matches) == 0 {
		return base.Render(name)
	}
	matched := make(map[int]bool, len(matches))
	for _, m := range matches {
		matched[m] = true
	}
	var b strings.Builder
	for i, r := range name {
		if matched[i] {
			b.WriteString(suggestionMatchStyle.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

func truncateRunes(s string, limit int) string {
	count := 0
	for i := range s {
		if count == limit {
			return s[:i] + "…"
		}
		count++
	}
	return s
}
