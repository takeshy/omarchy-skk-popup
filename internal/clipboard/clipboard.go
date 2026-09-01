// Package clipboard talks to the Wayland clipboard through wl-clipboard
// and synthesises the paste shortcut with wtype.
package clipboard

import (
	"errors"
	"os/exec"
	"strings"
	"sync"
)

// Wayland copies with wl-copy, reads with wl-paste, and pastes via wtype.
type Wayland struct {
	mu      sync.Mutex
	lastCmd *exec.Cmd
}

var ErrNoBackend = errors.New("wl-copy not found in PATH")

// Copy places text on the clipboard. A previous wl-copy child is killed
// first so repeated copies do not accumulate background processes;
// wl-copy forks immediately and keeps serving the selection from the new
// process.
func (w *Wayland) Copy(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := exec.LookPath("wl-copy"); err != nil {
		return ErrNoBackend
	}
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	if w.lastCmd != nil && w.lastCmd.Process != nil {
		_ = w.lastCmd.Process.Kill()
	}
	w.lastCmd = cmd
	return nil
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
