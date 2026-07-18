package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"github.com/jhayashi1/ramp-tui/internal/pathutil"
)

const (
	// maxVisibleSuggestions is the fixed height of the completion list
	// shown under the path prompt.
	maxVisibleSuggestions = 6
	// Caps keeping the recursive gif search responsive in huge trees.
	maxWalkDepth   = 6
	maxWalkResults = 500
	maxWalkVisits  = 20000
)

type pathSuggestion struct {
	// name is the gif's path relative to the typed directory portion.
	name string
	// matches holds byte offsets into name that matched the fuzzy query.
	matches []int
}

// pathInput is a text input with fzf-style search over the filesystem:
// it recursively finds .gif files under the directory portion of the
// typed path and fuzzy-filters their relative paths by the text after
// the last separator. A leading "~" expands to the home directory.
type pathInput struct {
	input textinput.Model
	st    styles
	// dirPrefix is the typed text up to and including the last path
	// separator; completions are appended to it.
	dirPrefix   string
	suggestions []pathSuggestion
	sel         int
	offset      int
	// walk cache: gifs found under walkRoot, so only keystrokes that
	// change the directory portion trigger a re-scan.
	walkRoot   string
	walkHidden bool
	walkNames  []string
}

func newPathInput(st styles) pathInput {
	input := textinput.New()
	input.Placeholder = "path/to/animation.gif"
	return pathInput{input: input, st: st}
}

func (p *pathInput) focus() tea.Cmd {
	p.input.SetValue("")
	p.walkNames = nil
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
// (including the trailing separator) and the fuzzy query after it. A
// bare "~" is treated as the home directory itself.
func splitPathPrefix(raw string) (prefix, query string) {
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
	prefix, query := splitPathPrefix(p.input.Value())
	p.dirPrefix = prefix

	root := pathutil.ExpandTilde(prefix)
	if root == "" {
		root = "."
	}
	hidden := strings.HasPrefix(query, ".")
	if p.walkNames == nil || root != p.walkRoot || hidden != p.walkHidden {
		p.walkRoot, p.walkHidden = root, hidden
		p.walkNames = findGifs(root, hidden)
	}

	if query == "" {
		p.suggestions = make([]pathSuggestion, len(p.walkNames))
		for i, name := range p.walkNames {
			p.suggestions[i] = pathSuggestion{name: name}
		}
		return
	}

	matches := fuzzy.Find(query, p.walkNames)
	p.suggestions = make([]pathSuggestion, len(matches))
	for i, m := range matches {
		p.suggestions[i] = pathSuggestion{name: m.Str, matches: m.MatchedIndexes}
	}
}

// findGifs walks root up to maxWalkDepth levels deep and returns the
// relative paths of the .gif files it finds, shallowest first. Hidden
// entries are skipped unless includeHidden is set, and the walk is
// capped so typing in a huge tree stays responsive.
func findGifs(root string, includeHidden bool) []string {
	names := []string{}
	visited := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if visited++; visited > maxWalkVisits || len(names) >= maxWalkResults {
			return fs.SkipAll
		}
		if !includeHidden && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.Count(rel, string(os.PathSeparator)) >= maxWalkDepth-1 {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".gif") {
			names = append(names, rel)
		}
		return nil
	})
	sort.SliceStable(names, func(i, j int) bool {
		di := strings.Count(names[i], string(os.PathSeparator))
		dj := strings.Count(names[j], string(os.PathSeparator))
		if di != dj {
			return di < dj
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names
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

// complete fills the input with the full path of the selected gif.
func (p *pathInput) complete() {
	if p.sel >= len(p.suggestions) {
		return
	}
	p.input.SetValue(p.dirPrefix + p.suggestions[p.sel].name)
	p.input.CursorEnd()
	p.refresh()
}

// accept resolves enter: the selected gif — or, with no suggestions,
// the literal typed text — yields the tilde-expanded path to render.
func (p *pathInput) accept() (path string, ok bool) {
	if len(p.suggestions) > 0 {
		p.complete()
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
	b.WriteString(p.st.prompt.Render("render gif: " + p.input.View()))
	b.WriteByte('\n')
	end := min(p.offset+maxVisibleSuggestions, len(p.suggestions))
	for i := p.offset; i < end; i++ {
		b.WriteString(p.suggestionRow(i))
		b.WriteByte('\n')
	}
	if len(p.suggestions) == 0 && strings.TrimSpace(p.input.Value()) != "" {
		b.WriteString(p.st.dim.Render("    no matching gifs"))
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
		marker = "  " + p.st.accent.Render("▸ ")
	}
	name := truncateRunes(s.name, max(4, p.input.Width-4))
	return marker + p.renderMatched(name, s.matches)
}

// renderMatched styles name rune by rune so the fuzzy-matched positions
// stand out, avoiding nested-style resets from wrapping the whole row.
func (p pathInput) renderMatched(name string, matches []int) string {
	base := p.st.text
	if len(matches) == 0 {
		return base.Render(name)
	}
	match := p.st.accent.Bold(true)
	matched := make(map[int]bool, len(matches))
	for _, m := range matches {
		matched[m] = true
	}
	var b strings.Builder
	for i, r := range name {
		if matched[i] {
			b.WriteString(match.Render(string(r)))
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
