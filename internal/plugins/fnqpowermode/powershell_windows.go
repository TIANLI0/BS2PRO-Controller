//go:build windows

package fnqpowermode

import (
	"context"
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func newPowerShellCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("powershell.exe", args...)
	cmd.SysProcAttr = hiddenPowerShellSysProcAttr()
	return cmd
}

func newPowerShellCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "powershell.exe", args...)
	cmd.SysProcAttr = hiddenPowerShellSysProcAttr()
	return cmd
}

func hiddenPowerShellSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
