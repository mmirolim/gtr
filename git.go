package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"sort"

	"context"
	"fmt"
	"os/exec"
	"strings"
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

// TODO pass CommandExecutor
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

// CommitChanges returns committing task
func CommitChanges(workDir string, newCmd CommandCreator) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		in := ctx.Value(prevTaskOutputKey).(string)
		if !strings.HasPrefix(in, "Tests PASS:") {
			return "", errors.New("nothing to commit")
		}
		// get types changed changed, used as commit message
		changes, err := GetDiff(ctx, workDir)
		if err != nil {
			return fmt.Sprintf("%s %s", "Commit error %v", err), nil
		}

		// filter go files
		fileNames := map[string]bool{}
		n := 0
		for _, x := range changes {
			if strings.HasSuffix(x.fpath, ".go") {
				fileNames[x.fpath] = true
				changes[n] = x
				n++
			}
		}
		changes = changes[:n]
		if len(changes) == 0 {
			return "", errors.New("nothing to commit")
		}
		fileInfos := map[string]FileInfo{}
		for _, change := range changes {
			info, ok := fileInfos[change.fpath]
			if !ok {
				info, err = getFileInfo(filepath.Join(workDir, change.fpath), nil)
				if err != nil {
					return fmt.Sprintf("Commit add error %v\n", err), nil
				}
				fileInfos[change.fpath] = info
			}
		}

		changedBlocks, err := changesToFileBlocks(changes, fileInfos)
		if err != nil {
			return fmt.Sprintf("Commit add error %v\n", err), nil
		}
		effectedBlocks := map[string]bool{}
		for _, info := range changedBlocks {
			for _, block := range info.blocks {
				effectedBlocks[block.name] = true
			}
		}
		list := mapStrToSlice(fileNames)
		sort.Strings(list)

		// add changes and commit in one command
		cmd := newCmd(ctx, "git", append([]string{"-C", workDir, "add"}, list...)...)
		err = cmd.Run()
		if err != nil {
			return fmt.Sprintf("Commit add error %v\n", err), nil
		}
		list = mapStrToSlice(effectedBlocks)
		sort.Strings(list)

		out := "'auto_commit! " + strings.Join(list, " ") + "'"
		// commit changes
		cmd = newCmd(ctx, "git", append([]string{"-C", workDir, "commit", "-m"}, out)...)
		err = cmd.Run()
		if err != nil {
			return fmt.Sprintf("Commit commit error %v\n", err), nil
		}
		return out, nil
	}
}
