package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

var _ CommandExecutor = (*MockCommand)(nil)

// MockCommand mock executor for testing
// implements CommandExecutor interface
type MockCommand struct {
	bin            string
	args           []string
	env            []string
	stdOut, stdErr io.Writer
	success        bool
	error          error
	execLog        []string
}

// NewMockCommand returns preconfigured command
// with errors and success status
func NewMockCommand(err error, success bool) MockCommand {
	return MockCommand{
		error: err, success: success,
	}
}

// New --
func (c *MockCommand) New(ctx context.Context, bin string, args ...string) CommandExecutor {
	c.bin = bin
	c.args = args
	c.execLog = append(c.execLog, bin+" "+strings.Join(args, " "))
	return c
}

// GetArgs --
func (c *MockCommand) GetArgs() []string {
	return append([]string{c.bin}, c.args...)
}

// Run --
func (c *MockCommand) Run() error {
	return c.error
}

// SetStdout --
func (c *MockCommand) SetStdout(wr io.Writer) {
	c.stdOut = wr
}

// SetStderr --
func (c *MockCommand) SetStderr(wr io.Writer) {
	c.stdErr = wr
}

// SetEnv --
func (c *MockCommand) SetEnv(env []string) {
	c.env = env
}

// Success --
func (c *MockCommand) Success() bool {
	return c.success
}

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
