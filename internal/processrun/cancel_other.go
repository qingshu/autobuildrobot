//go:build !windows && !darwin && !linux

package processrun

import "os/exec"

func configureCancellation(cmd *exec.Cmd) {}
