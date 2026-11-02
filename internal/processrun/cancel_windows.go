//go:build windows

package processrun

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

func configureCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Cancel = func() error {
		// Kill descendants as well as Git Bash; wait for taskkill to complete.
		kill := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if out, err := kill.CombinedOutput(); err != nil {
			return fmt.Errorf("terminate process tree: %w: %s", err, out)
		}
		return nil
	}
}
