package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
)

var _ Task = (*DesktopNotificator)(nil)

// DesktopNotificator implements Task
type DesktopNotificator struct {
	transient  bool
	expireTime string
}

// NewDesktopNotificator constructs task
func NewDesktopNotificator(transient bool, expireInMillisecond int) *DesktopNotificator {
	notifier := &DesktopNotificator{}
	notifier.transient = transient
	notifier.expireTime = strconv.Itoa(expireInMillisecond)
	return notifier
}

// ID of the task
func (n *DesktopNotificator) ID() string {
	return "DesktopNotificator"
}

// Run implements Task interface
func (n *DesktopNotificator) Run(ctx context.Context) (string, error) {
	in := ctx.Value(prevTaskOutputKey).(string)
	return in, n.Send(ctx, in)
}

// Send desktop notification
func (n *DesktopNotificator) Send(ctx context.Context, msg string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript", "-e",
			fmt.Sprintf("'display notification \"%s\" with title \"%s\"'", msg, msg))

	case "linux":
		cmd = exec.CommandContext(ctx, "notify-send", "-t", n.expireTime)
		if n.transient {
			cmd.Args = append(cmd.Args, []string{"--hint", "int:transient:1"}...)
		}
		cmd.Args = append(cmd.Args, msg)
	default:
		return fmt.Errorf("unsupported os %s", runtime.GOOS)
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Desktop notification error %v", err)
	}
	return nil
}
