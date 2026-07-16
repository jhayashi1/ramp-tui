package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/lipgloss"
)

// toggleHelpMsg flips the top-level model's help overlay. It is
// produced as a command (rather than handled inline) so any screen's
// keymap can trigger it the same way playEntryMsg or backToGalleryMsg
// cross from one sub-model into app.go.
type toggleHelpMsg struct{}

// renderHelpOverlay replaces the whole screen with a centered borderless
// block listing the current screen's full key bindings. There is no
// attempt to composite over the screen underneath: the rest of the
// terminal is simply left blank while the overlay is open, the same as
// the render and player screens' own full-screen views.
func renderHelpOverlay(width, height int, keys help.KeyMap, st styles) string {
	content := st.columnTitle.Render("HELP") + "\n\n" + formatFullHelp(keys, st)
	return lipgloss.Place(max(1, width), max(1, height), lipgloss.Center, lipgloss.Center, content)
}

// formatFullHelp renders a help.KeyMap's FullHelp groups as one
// "key  description" row per binding, blank-line separated by group.
func formatFullHelp(keys help.KeyMap, st styles) string {
	var groups []string
	for _, group := range keys.FullHelp() {
		var rows []string
		for _, b := range group {
			h := b.Help()
			if h.Key == "" {
				continue
			}
			rows = append(rows, st.accent.Bold(true).Render(h.Key)+"  "+st.dim.Render(h.Desc))
		}
		if len(rows) > 0 {
			groups = append(groups, strings.Join(rows, "\n"))
		}
	}
	return strings.Join(groups, "\n\n")
}
