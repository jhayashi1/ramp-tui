package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"

	"github.com/jhayashi1/ascii-tui/internal/config"
)

var (
	_ help.KeyMap = galleryKeyMap{}
	_ help.KeyMap = playerKeyMap{}
	_ help.KeyMap = keybindsKeyMap{}
)

// galleryKeyMap is the single source of truth for gallery bindings: it
// drives both key.Matches dispatch and the footer/help-overlay text.
type galleryKeyMap struct {
	Play     key.Binding
	Add      key.Binding
	Rename   key.Binding
	Delete   key.Binding
	Filter   key.Binding
	Keybinds key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func newGalleryKeyMap() galleryKeyMap {
	return galleryKeyMap{
		Play:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play")),
		Add:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add gif")),
		Rename:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Keybinds: key.NewBinding(key.WithKeys("k"), key.WithHelp("k", "keybinds")),
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k galleryKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Play, k.Add, k.Rename, k.Delete, k.Keybinds, k.Help, k.Quit}
}

func (k galleryKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Play, k.Add, k.Rename, k.Delete},
		{k.Filter, k.Keybinds, k.Help, k.Quit},
	}
}

// playerKeyMap is the single source of truth for player bindings.
type playerKeyMap struct {
	Pause       key.Binding
	SeekBack    key.Binding
	SeekForward key.Binding
	StepBack    key.Binding
	StepForward key.Binding
	SpeedUp     key.Binding
	SpeedDown   key.Binding
	Next        key.Binding
	Prev        key.Binding
	Filter      key.Binding
	Help        key.Binding
	Back        key.Binding
	Quit        key.Binding
}

// newPlayerKeyMap builds the playback bindings from the user's
// configured keys; Help, Back, and Quit stay fixed so a bad config can
// never lock the user inside the player. A zero-value config.Keys gets
// the defaults via fillEmpty-style fallback in configBinding.
func newPlayerKeyMap(keys config.Keys) playerKeyMap {
	def := config.DefaultKeys()
	return playerKeyMap{
		Pause:       configBinding(keys.Pause, def.Pause, "pause"),
		SeekBack:    configBinding(keys.SeekBack, def.SeekBack, "scrub"),
		SeekForward: configBinding(keys.SeekForward, def.SeekForward, "scrub"),
		StepBack:    configBinding(keys.StepBack, def.StepBack, "step back"),
		StepForward: configBinding(keys.StepForward, def.StepForward, "step fwd"),
		SpeedUp:     configBinding(keys.SpeedUp, def.SpeedUp, "speed up"),
		SpeedDown:   configBinding(keys.SpeedDown, def.SpeedDown, "speed down"),
		Next:        configBinding(keys.Next, def.Next, "next"),
		Prev:        configBinding(keys.Prev, def.Prev, "prev"),
		Filter:      configBinding(keys.Filter, def.Filter, "filter bg"),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// configBinding turns a configured key list (falling back to def when
// empty) into a binding whose help label shows the humanized keys.
func configBinding(tokens, def []string, desc string) key.Binding {
	if len(tokens) == 0 {
		tokens = def
	}
	teaKeys := make([]string, len(tokens))
	for i, tok := range tokens {
		teaKeys[i] = tokenToKey(tok)
	}
	return key.NewBinding(key.WithKeys(teaKeys...), key.WithHelp(keyLabel(tokens), desc))
}

// tokenToKey maps a config token to the string Bubble Tea reports for
// that key. The only divergence is the space bar, stored as "space" so
// config files stay readable.
func tokenToKey(token string) string {
	if token == "space" {
		return " "
	}
	return token
}

// keyToken is tokenToKey's inverse, turning a tea.KeyMsg.String() value
// into its config spelling.
func keyToken(key string) string {
	if key == " " {
		return "space"
	}
	return key
}

// displayKey renders a config token for help text and the keybinds
// menu, using arrows for the cursor keys the same way the old
// hard-coded help strings did.
func displayKey(token string) string {
	switch token {
	case "left":
		return "←"
	case "right":
		return "→"
	case "up":
		return "↑"
	case "down":
		return "↓"
	default:
		return token
	}
}

// keyLabel joins a key list into a compact "←/h"-style help label.
func keyLabel(tokens []string) string {
	parts := make([]string, len(tokens))
	for i, tok := range tokens {
		parts[i] = displayKey(tok)
	}
	return strings.Join(parts, "/")
}

func (k playerKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Pause, k.SeekBack, k.SeekForward, k.Next, k.Prev, k.Filter, k.Help, k.Back, k.Quit}
}

func (k playerKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Pause, k.SeekBack, k.SeekForward, k.StepBack, k.StepForward},
		{k.SpeedUp, k.SpeedDown, k.Next, k.Prev},
		{k.Filter, k.Help, k.Back, k.Quit},
	}
}

// shortHelpLine joins a keymap's short-help bindings into a tuxedo-style
// "key action · key action" hint string for the status bar.
func shortHelpLine(bindings []key.Binding) string {
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		if h.Key == "" {
			continue
		}
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return strings.Join(parts, " · ")
}
