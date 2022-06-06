// +build darwin

package base

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func Daemon() {
	argvs := os.Args[1:]
	cmd := exec.Command(os.Args[0], argvs...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	err := cmd.Start()
	if err == nil {
		fmt.Printf("Daemon process %d is created successfully.\n", cmd.Process.Pid)
		_ = cmd.Process.Release()
		os.Exit(0)
	}
}
