package main

import (
	"context"
	"io"
	"os/exec"
	"strings"
)

// CommandExecutor interface for os command execution
type CommandExecutor interface {
	GetArgs() []string
	Run() error
	Success() bool
	SetStdout(wr io.Writer)
	SetStderr(wr io.Writer)
	SetEnv(env []string)
}

// CommandCreator constructer interface
type CommandCreator func(context.Context, string, ...string) CommandExecutor

var _ CommandExecutor = (*OsCommand)(nil)

// NewOsCommand returns real command executor
func NewOsCommand(ctx context.Context, bin string, args ...string) CommandExecutor {
	cmd := exec.CommandContext(ctx, bin, args...)
	return &OsCommand{cmd}
}

// OsCommand wrapper for exec.Cmd
type OsCommand struct {
	*exec.Cmd
}

// GetArgs returns all command arguments
func (c *OsCommand) GetArgs() []string {
	return c.Cmd.Args
}

// Success returns true if execution returned 0 exit code
func (c *OsCommand) Success() bool {
	// cmd maybe killed by canceled ctx
	if c.Cmd.ProcessState != nil {
		return c.Cmd.ProcessState.Success()
	}
	return false
}

// SetStdout settter
func (c *OsCommand) SetStdout(wr io.Writer) {
	c.Cmd.Stdout = wr
}

// SetStderr settter
func (c *OsCommand) SetStderr(wr io.Writer) {
	c.Cmd.Stderr = wr
}

// SetEnv setter
func (c *OsCommand) SetEnv(env []string) {
	c.Cmd.Env = env
}

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
