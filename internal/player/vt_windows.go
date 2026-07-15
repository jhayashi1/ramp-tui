//go:build windows

package player

import (
	"os"

	"golang.org/x/sys/windows"
)

// EnableVirtualTerminal turns on ANSI escape processing for legacy
// Windows consoles. Modern Windows Terminal has it on already.
func EnableVirtualTerminal(f *os.File) {
	var mode uint32
	handle := windows.Handle(f.Fd())
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
