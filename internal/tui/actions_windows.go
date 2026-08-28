//go:build windows

package tui

import (
	"fmt"
	"os/exec"
	"strconv"
)

const actionsSupported = true

func killProcess(pid int) error {
	cmd := exec.Command(
		"taskkill",
		"/PID", strconv.Itoa(pid),
		"/T", // 同时结束子进程
		"/F", // 强制结束
	)

	return cmd.Run()
}
func termProcess(pid int) error    { return fmt.Errorf("not supported on Windows") }
func pauseProcess(pid int) error   { return fmt.Errorf("not supported on Windows") }
func resumeProcess(pid int) error  { return fmt.Errorf("not supported on Windows") }
func setNice(pid, value int) error { return fmt.Errorf("not supported on Windows") }
