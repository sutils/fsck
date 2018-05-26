package main

import "os/exec"

func setCmdAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true
}
