// +build windows

package platform

import (
	"os/exec"
	"github.com/gonutz/w32/v2"
)

func Shutdown() error {
	cmd := exec.Command("cmd", "/C", "shutdown", "/s", "/hybrid", "/t", "0")
	return cmd.Run()
}

func init() {
	console := w32.GetConsoleWindow()
	if console != 0 {
		_, consoleProcID := w32.GetWindowThreadProcessId(console)
		if w32.GetCurrentProcessId() == consoleProcID {
			w32.ShowWindowAsync(console, w32.SW_HIDE)
		}
	}
}
