// Package clipboard talks to the Wayland clipboard through wl-clipboard
// and synthesises the paste shortcut with wtype.
package clipboard

import (
	"errors"
	"os/exec"
	"strings"
)

// Wayland copies with wl-copy, reads with wl-paste, and pastes via wtype.
type Wayland struct{}

var ErrNoBackend = errors.New("wl-copy not found in PATH")

// Copy places text on the clipboard. wl-copy daemonises itself to keep
// serving the selection, so Run returns once it has consumed our text and
// forked: the clipboard is populated even if the caller exits immediately
// afterwards (a plain Start would race that shutdown and lose the write).
func (w *Wayland) Copy(text string) error {
	if _, err := exec.LookPath("wl-copy"); err != nil {
		return ErrNoBackend
	}
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// Read returns the clipboard's plain text ("" when it holds none).
func (w *Wayland) Read() (string, error) {
	out, err := exec.Command("wl-paste", "--no-newline", "--type", "text").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Paste sends the configured paste shortcut to the focused window.
func (w *Wayland) Paste(shortcut string) error {
	switch shortcut {
	case "ctrl+shift+v":
		// Most terminal emulators reserve Ctrl+V for readline's "insert
		// next character literally" and paste on Ctrl+Shift+V instead.
		return exec.Command("wtype", "-M", "ctrl", "-M", "shift", "-k", "v", "-m", "shift", "-m", "ctrl").Run()
	default:
		return exec.Command("wtype", "-M", "ctrl", "-k", "v", "-m", "ctrl").Run()
	}
}
