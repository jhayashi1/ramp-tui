package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jhayashi1/ascii-tui/internal/config"
)

// Messages crossing between the keybinds screen and app.go:
// openKeybindsMsg switches to the screen, keysChangedMsg hands edited
// bindings up so the app can adopt and persist them, and keysSavedMsg
// reports the save result back down for the status bar.
type (
	openKeybindsMsg struct{}
	keysChangedMsg  struct{ keys config.Keys }
	keysSavedMsg    struct{ err error }
)

// reservedKeys are the keys the keybinds screen refuses to assign to a
// player action: they navigate the player (and this menu) themselves,
// so rebinding them could lock the user out.
var reservedKeys = map[string]string{
	"esc":    "back",
	"q":      "quit",
	"ctrl+c": "quit",
	"?":      "help",
}

// keybindAction names one rebindable player action and knows where its
// key list lives inside a config.Keys.
type keybindAction struct {
	label string
	get   func(*config.Keys) *[]string
}

// keybindActions fixes the menu order. Labels match the player help
// text so the two screens describe actions with the same words.
var keybindActions = []keybindAction{
	{"pause", func(k *config.Keys) *[]string { return &k.Pause }},
	{"scrub back", func(k *config.Keys) *[]string { return &k.SeekBack }},
	{"scrub forward", func(k *config.Keys) *[]string { return &k.SeekForward }},
	{"step back", func(k *config.Keys) *[]string { return &k.StepBack }},
	{"step forward", func(k *config.Keys) *[]string { return &k.StepForward }},
	{"speed up", func(k *config.Keys) *[]string { return &k.SpeedUp }},
	{"speed down", func(k *config.Keys) *[]string { return &k.SpeedDown }},
	{"next", func(k *config.Keys) *[]string { return &k.Next }},
	{"prev", func(k *config.Keys) *[]string { return &k.Prev }},
	{"filter bg", func(k *config.Keys) *[]string { return &k.Filter }},
}

// keybindsKeyMap drives the menu itself; these are fixed, not
// user-configurable, since they are the tool doing the configuring.
type keybindsKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Rebind  key.Binding
	AddKey  key.Binding
	Reset   key.Binding
	Default key.Binding
	Help    key.Binding
	Back    key.Binding
	Quit    key.Binding
}

func newKeybindsKeyMap() keybindsKeyMap {
	return keybindsKeyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Rebind:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "rebind")),
		AddKey:  key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add key")),
		Reset:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "reset")),
		Default: key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "reset all")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Back:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keybindsKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Rebind, k.AddKey, k.Reset, k.Default, k.Help, k.Back, k.Quit}
}

func (k keybindsKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Rebind, k.AddKey},
		{k.Reset, k.Default, k.Help, k.Back, k.Quit},
	}
}

// captureMode says what a captured keypress will do to the selected
// action's key list.
type captureMode int

const (
	captureNone    captureMode = iota
	captureReplace             // pressed key becomes the only binding
	captureAppend              // pressed key is added alongside existing ones
)

type keybindsModel struct {
	keys    config.Keys
	cursor  int
	capture captureMode
	status  string
	st      styles
	menu    keybindsKeyMap
	width   int
	height  int
}

func newKeybinds(keys config.Keys, st styles) keybindsModel {
	keys = cloneKeys(keys)
	return keybindsModel{keys: keys, st: st, menu: newKeybindsKeyMap()}
}

// cloneKeys deep-copies the key lists so menu edits never alias the
// app model's config until it adopts them via keysChangedMsg.
func cloneKeys(keys config.Keys) config.Keys {
	for _, action := range keybindActions {
		slot := action.get(&keys)
		*slot = append([]string(nil), *slot...)
	}
	keys.FillEmpty()
	return keys
}

func (k *keybindsModel) setSize(width, height int) {
	k.width, k.height = width, height
}

func (k keybindsModel) update(msg tea.Msg) (keybindsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case keysSavedMsg:
		if msg.err != nil {
			k.status = fmt.Sprintf("save failed: %v", msg.err)
		}
		return k, nil

	case tea.KeyMsg:
		if k.capture != captureNone {
			return k.updateCapture(msg)
		}
		switch {
		case key.Matches(msg, k.menu.Quit):
			return k, tea.Quit
		case key.Matches(msg, k.menu.Back):
			return k, func() tea.Msg { return backToGalleryMsg{} }
		case key.Matches(msg, k.menu.Help):
			return k, func() tea.Msg { return toggleHelpMsg{} }
		case key.Matches(msg, k.menu.Up):
			k.cursor = (k.cursor - 1 + len(keybindActions)) % len(keybindActions)
			return k, nil
		case key.Matches(msg, k.menu.Down):
			k.cursor = (k.cursor + 1) % len(keybindActions)
			return k, nil
		case key.Matches(msg, k.menu.Rebind):
			k.capture = captureReplace
			k.status = ""
			return k, nil
		case key.Matches(msg, k.menu.AddKey):
			k.capture = captureAppend
			k.status = ""
			return k, nil
		case key.Matches(msg, k.menu.Reset):
			*keybindActions[k.cursor].get(&k.keys) = nil
			k.keys.FillEmpty()
			k.status = ""
			return k, k.changedCmd()
		case key.Matches(msg, k.menu.Default):
			k.keys = config.DefaultKeys()
			k.status = ""
			return k, k.changedCmd()
		}
	}
	return k, nil
}

// updateCapture consumes the keypress that follows "rebind"/"add key":
// esc cancels, a reserved or conflicting key is rejected with a status
// message, and anything else is committed and handed up for saving.
func (k keybindsModel) updateCapture(msg tea.KeyMsg) (keybindsModel, tea.Cmd) {
	mode := k.capture
	k.capture = captureNone
	pressed := msg.String()
	if pressed == "esc" {
		k.status = ""
		return k, nil
	}
	token := keyToken(pressed)
	if action, ok := reservedKeys[pressed]; ok {
		k.status = fmt.Sprintf("%q is reserved for %s", displayKey(token), action)
		return k, nil
	}
	if owner, ok := k.ownerOf(token); ok {
		if owner == k.cursor && mode == captureAppend {
			k.status = fmt.Sprintf("%q is already bound to %s", displayKey(token), keybindActions[owner].label)
			return k, nil
		}
		if owner != k.cursor {
			k.status = fmt.Sprintf("%q is taken by %s", displayKey(token), keybindActions[owner].label)
			return k, nil
		}
	}

	slot := keybindActions[k.cursor].get(&k.keys)
	if mode == captureAppend {
		*slot = append(*slot, token)
	} else {
		*slot = []string{token}
	}
	k.status = ""
	return k, k.changedCmd()
}

// ownerOf finds which action, if any, currently holds token.
func (k *keybindsModel) ownerOf(token string) (int, bool) {
	for i, action := range keybindActions {
		for _, t := range *action.get(&k.keys) {
			if t == token {
				return i, true
			}
		}
	}
	return 0, false
}

// changedCmd hands the edited bindings to app.go, which adopts them
// into its config and persists to disk.
func (k keybindsModel) changedCmd() tea.Cmd {
	keys := cloneKeys(k.keys)
	return func() tea.Msg { return keysChangedMsg{keys: keys} }
}

// keybindsPanelWidth fits "scrub forward" plus a generous key column.
const keybindsPanelWidth = 42

func (k keybindsModel) view() string {
	panelW := min(keybindsPanelWidth, max(1, k.width))
	rows := make([]string, 0, len(keybindActions)+2)
	rows = append(rows, sectionRule("player keybinds", panelW, k.st), "")
	labelW := 15
	for i, action := range keybindActions {
		label := fitLine(action.label, labelW)
		keysText := keyLabel(*action.get(&k.keys))
		line := label + truncateLabel(keysText, max(0, panelW-labelW-2))
		if i == k.cursor {
			rows = append(rows, k.st.selBarText.Render(fitLine("▸ "+line, panelW)))
		} else {
			rows = append(rows, "  "+k.st.text.Render(label)+k.st.accent.Render(truncateLabel(keysText, max(0, panelW-labelW-2))))
		}
	}
	panel := strings.Join(rows, "\n")
	return lipgloss.Place(max(1, k.width), max(1, k.height-1), lipgloss.Center, lipgloss.Center, panel) +
		"\n" + k.statusBar()
}

func (k keybindsModel) statusBar() string {
	chipStyle, chipLabel := k.st.chip, "KEYBINDS"
	middleStyle := k.st.help
	var middle string
	if k.capture != captureNone {
		chipStyle, chipLabel = k.st.chipAlert, "REBIND"
		verb := "rebind"
		if k.capture == captureAppend {
			verb = "add to"
		}
		middleStyle = k.st.warning
		middle = fmt.Sprintf("press a key to %s %q · esc cancel", verb, keybindActions[k.cursor].label)
	} else {
		middle = shortHelpLine(k.menu.ShortHelp())
	}
	if k.status != "" {
		middle, middleStyle = k.status, k.st.status
	}
	return renderStatusBar(chipStyle, chipLabel, middle, middleStyle, brandName, k.width, k.st)
}
