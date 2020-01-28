package main

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestOsCommand(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewOsCommand(context.TODO(), "echo", "hi")
	cmd.SetStdout(&buf)
	cmd.SetStderr(&buf)
	err := cmd.Run()
	if err != nil {
		t.Errorf("unexpected err %v", err)
		return
	}
	if !cmd.Success() {
		t.Error("expected cmd success, got false")
		return
	}
	out := buf.String()
	if out[:2] != "hi" {
		t.Errorf("expected \"hi\", got \"%s\"", out[:2])
		return
	}
	// kill immediately
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	cmd = NewOsCommand(ctx, "watch", "echo", "watch")
	err = cmd.Run()
	if err == nil {
		t.Error("expected not nil error")
		return
	}
	if cmd.Success() {
		t.Error("expected cmd success false, got true")
	}
}
