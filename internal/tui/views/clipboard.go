package views

import (
	"os/exec"
	"runtime"
	"strings"
)

// CopyToClipboard copies text to the system clipboard.
// Uses pbcopy on macOS, xclip on Linux. Fails silently if unavailable.
func CopyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return nil // unsupported platform — no-op
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
