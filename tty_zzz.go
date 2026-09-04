//go:build !aix && !android && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows && !zos

package pty

// IsTerminal returns false on platforms that do not provide Unix terminals or
// Windows consoles.
func IsTerminal(fd uintptr) bool {
	return false
}

// EnableVirtualTerminal is a no-op on platforms without a console API to
// configure. See tty_windows.go for the Windows implementation.
func EnableVirtualTerminal(stdin, stdout, stderr bool) error {
	return nil
}
