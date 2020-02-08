package main

import (
	"bytes"
	"errors"
	"log"
	"path/filepath"
	"regexp"
	"sort"

	"context"
	"fmt"
	"os/exec"
	"strings"
)

// regex to get untracked go file names from git status --short
var reFnameUntrackedFiles = regexp.MustCompile(`\?\? (?P<fname>[a-zA-Z0-9_\-/]+\.go)`)

// GitCMD git wrapper
type GitCMD struct {
	workDir string
}

// NewGitCMD returns git wrapper
func NewGitCMD(workDir string) *GitCMD {
	return &GitCMD{workDir}
}

// Diff returns file changes
// TODO pass CommandExecutor
func (g *GitCMD) Diff(ctx context.Context) ([]Change, error) {
	var gitOut bytes.Buffer
	var results []Change
	// get not yet committed go files in a workdir
	gitCmd := exec.CommandContext(ctx, "git", "-C", g.workDir, "status", "--short", g.workDir)
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
	// get git diff changes only in workDir (--relative)
	// -U0 zero lines around changes
	// Disallow external diff drivers.
	gitCmd = exec.CommandContext(ctx, "git", "-C", g.workDir, "diff", "-U0", "--no-ext-diff", "--relative")
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

// CommitChanges returns task to git commit file changes
// TODO maybe use branch as config gtr-no-commit, will suspend from committing
func CommitChanges(
	workDir string,
	newCmd CommandCreator,
) func(*log.Logger, context.Context) (string, error) {
	gitcmd := NewGitCMD(workDir)
	return func(log *log.Logger, ctx context.Context) (string, error) {
		in := ctx.Value(prevTaskOutputKey).(string)
		if !strings.HasPrefix(in, "Tests PASS:") {
			return "", errors.New("nothing to commit")
		}
		// get types changed changed, used as commit message
		changes, err := gitcmd.Diff(ctx)
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
			if _, ok := fileInfos[change.fpath]; !ok {
				info, err := getFileInfo(filepath.Join(workDir, change.fpath), nil)
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
