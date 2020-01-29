package main

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

var _ Task = (*DesktopNotificator)(nil)

type DesktopNotificator struct {
	transient  bool
	expireTime string
}

func NewDesktopNotificator(transient bool, expireInMillisecond int) *DesktopNotificator {
	notifier := &DesktopNotificator{}
	notifier.transient = transient
	notifier.expireTime = strconv.Itoa(expireInMillisecond)
	return notifier
}

func (n *DesktopNotificator) ID() string {
	return "DesktopNotificator"
}

func (n *DesktopNotificator) Run(ctx context.Context) (string, error) {
	in := ctx.Value(prevTaskOutputKey).(string)
	return in, n.Send(ctx, in)
}

func (n *DesktopNotificator) Send(ctx context.Context, msg string) error {
	cmd := exec.CommandContext(ctx, "notify-send", "-t", n.expireTime)
	if n.transient {
		cmd.Args = append(cmd.Args, []string{"--hint", "int:transient:1"}...)
	}
	cmd.Args = append(cmd.Args, msg)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Desktop notification error %v", err)
	}
	return nil
}
