// +build !windows

package platform

import (
	"os/exec"
)

func Shutdown() error {
	cmd := exec.Command("shutdown", "-h", "now")
	return cmd.Run()
}
