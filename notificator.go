package main

import (
	"fmt"
	"os/exec"
	"strconv"
)

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

func (n *DesktopNotificator) Send(msg string) error {
	cmd := exec.Command("notify-send", "-t", n.expireTime)
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
