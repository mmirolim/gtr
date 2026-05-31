package main

import (
	"context"
	"testing"
	"time"
)

func TestDesktopNotificator(t *testing.T) {
	notifier := NewDesktopNotificator(true, 2000)
	if notifier.ID() != "DesktopNotificator" {
		t.Errorf("expected ID DesktopNotificator, got %v", notifier.ID())
		return
	}
	if notifier.expireTime != "2000" {
		t.Errorf("expected expireTime 2000, got %v", notifier.expireTime)
		return
	}

	if notifier.transient != true {
		t.Error("expected transient true, got false")
	}

	// Task should be cancelable
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	tc := &TaskContext{PrevTaskOutput: "prev task msg"}
	_, err := notifier.Run(ctx, tc)
	if err == nil {
		t.Error("expected not nil error")
		return
	}
}
