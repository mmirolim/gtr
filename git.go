package main

import (
	"bytes"
	"context"
	"os/exec"
)

type GitCMD struct {
	workDir string
}

func NewGitCMD(workDir string) *GitCMD {
	return &GitCMD{workDir}
}

func (g *GitCMD) Diff(ctx context.Context) ([]Change, error) {
	return GetDiff(ctx, g.workDir)
}

func GetDiff(ctx context.Context, workdir string) ([]Change, error) {
	// TODO store hashes of new files and return untracked new files to run
	var gitOut bytes.Buffer
	var results []Change
	// get not yet commited go files
	gitCmd := exec.CommandContext(ctx, "git", "-C", workdir, "status", "--short")
	gitCmd.Stdout = &gitOut
	err := gitCmd.Run()
	if err != nil {
		return nil, err
	}
	matches := reFnameUntrackedFiles.FindAllString(gitOut.String(), -1)
	for i := range matches {
		fname := reFnameUntrackedFiles.ReplaceAllString(matches[i], "${fname}")
		results = append(results, Change{fname, fname, 0, 0})
	}
	gitOut.Reset()
	// get changes in go files
	// Disallow external diff drivers.
	gitCmd = exec.CommandContext(ctx, "git", "-C", workdir, "diff", "--no-ext-diff")
	gitCmd.Stdout = &gitOut
	err = gitCmd.Run()
	if err != nil {
		return nil, err
	}
	changes, err := changesFromGitDiff(gitOut)
	if err != nil {
		return nil, err
	}
	results = append(results, changes...)
	return results, nil
}

// TODO add context
func GitCmdFactory(workDir string) func(args ...string) error {
	return func(args ...string) error {
		gitCmd := exec.Command("git", "-C", workDir)
		gitCmd.Args = append(gitCmd.Args, args...)
		return gitCmd.Run()
	}
}
