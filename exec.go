package main

import (
	"context"
	"io"
	"os/exec"
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
