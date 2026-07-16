package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

var (
	_ help.KeyMap = galleryKeyMap{}
	_ help.KeyMap = playerKeyMap{}
)

// galleryKeyMap is the single source of truth for gallery bindings: it
// drives both key.Matches dispatch and the footer/help-overlay text.
type galleryKeyMap struct {
	Play   key.Binding
	Add    key.Binding
	Rename key.Binding
	Delete key.Binding
	Filter key.Binding
	Help   key.Binding
	Quit   key.Binding
}

func newGalleryKeyMap() galleryKeyMap {
	return galleryKeyMap{
		Play:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play")),
		Add:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add gif")),
		Rename: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rename")),
		Delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k galleryKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Play, k.Add, k.Rename, k.Delete, k.Help, k.Quit}
}

func (k galleryKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Play, k.Add, k.Rename, k.Delete},
		{k.Filter, k.Help, k.Quit},
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

func newPlayerKeyMap() playerKeyMap {
	return playerKeyMap{
		Pause:       key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause")),
		SeekBack:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "scrub")),
		SeekForward: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "scrub")),
		StepBack:    key.NewBinding(key.WithKeys(","), key.WithHelp(",", "step back")),
		StepForward: key.NewBinding(key.WithKeys("."), key.WithHelp(".", "step fwd")),
		SpeedUp:     key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "speed up")),
		SpeedDown:   key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "speed down")),
		Next:        key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next")),
		Prev:        key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "prev")),
		Filter:      key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter bg")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Back:        key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
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
