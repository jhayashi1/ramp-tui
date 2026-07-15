// Package pathutil provides small path helpers shared by the CLI and TUI.
package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandTilde replaces a leading "~" with the current user's home
// directory. Forms like "~user/..." are returned unchanged, as is the
// input when the home directory cannot be determined.
func ExpandTilde(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}
