//go:build !windows

package player

import "os"

// EnableVirtualTerminal is a no-op on platforms where ANSI escape
// processing is always available.
func EnableVirtualTerminal(*os.File) {}
