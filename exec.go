package main

import (
	"context"
	"io"
	"os/exec"
)

type CommandExecutor interface {
	GetArgs() []string
	Run() error
	Success() bool
	SetStdout(wr io.Writer)
	SetStderr(wr io.Writer)
	SetEnv(env []string)
}

type CommandCreator func(context.Context, string, ...string) CommandExecutor

var _ CommandExecutor = (*OsCommand)(nil)

func NewOsCommand(ctx context.Context, bin string, args ...string) CommandExecutor {
	cmd := exec.CommandContext(ctx, bin, args...)
	return &OsCommand{cmd}
}

type OsCommand struct {
	*exec.Cmd
}

func (c *OsCommand) GetArgs() []string {
	return c.Cmd.Args
}

func (c *OsCommand) Success() bool {
	// cmd maybe killed by canceled ctx
	if c.Cmd.ProcessState != nil {
		return c.Cmd.ProcessState.Success()
	}
	return false
}

func (c *OsCommand) SetStdout(wr io.Writer) {
	c.Cmd.Stdout = wr
}

func (c *OsCommand) SetStderr(wr io.Writer) {
	c.Cmd.Stderr = wr
}

func (c *OsCommand) SetEnv(env []string) {
	c.Cmd.Env = env
}

var _ CommandExecutor = (*MockCommand)(nil)

type MockCommand struct {
	bin            string
	args           []string
	env            []string
	stdOut, stdErr io.Writer
	success        bool
	error          error
}

func NewMockCommand(err error, success bool) MockCommand {
	return MockCommand{
		error: err, success: success,
	}
}

func (c *MockCommand) New(ctx context.Context, bin string, args ...string) CommandExecutor {
	c.args = args
	return c
}

func (c *MockCommand) GetArgs() []string {
	return c.args
}

func (c *MockCommand) Run() error {
	return c.error
}

func (c *MockCommand) SetStdout(wr io.Writer) {
	c.stdOut = wr
}

func (c *MockCommand) SetStderr(wr io.Writer) {
	c.stdErr = wr
}

func (c *MockCommand) SetEnv(env []string) {
	c.env = env
}

func (c *MockCommand) Success() bool {
	return c.success
}
