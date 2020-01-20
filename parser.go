package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var reFnameAndLinesInDiff = regexp.MustCompile(`diff --git a/(?P<fname>.+\.go) (?:[^@]+|\n)+@@ (?P<old>-\d+,\d+) (?P<new>\+\d+,\d+) @@`)
var reFnameUntrackedFiles = regexp.MustCompile(`\?\? (?P<fname>.+\.go)`)

var mu sync.Mutex
var untrackedFiles = map[string]string{}

type Change struct {
	fpath string
	start int
	count int
}

func newChange(diff string) (Change, error) {
	var change Change
	var err error
	parts := strings.Split(diff, " ")
	nparts := len(parts)
	if nparts == 3 {
		change.fpath = parts[0]
		if len(parts[2]) == 0 || parts[2][0] != '+' {
			return change, fmt.Errorf("wrong diff format %s", diff)
		}
		lines := strings.Split(parts[2][1:], ",")
		change.start, err = strconv.Atoi(lines[0])
		if err != nil {
			return change, nil
		}
		change.count, err = strconv.Atoi(lines[1])
		if err != nil {
			return change, nil
		}
	} else if nparts == 1 {
		change.fpath = parts[0]
	} else {
		return change, fmt.Errorf("wrong diff format %s", diff)
	}
	return change, nil
}

// TODO comments
func GetDiff(workdir string) ([]Change, error) {
	// TODO store hashes of new files and return untracked new files to run
	var gitOut bytes.Buffer
	// get not yet commited go files
	gitCmd := exec.Command("git", "-C", workdir, "status", "--short")
	gitCmd.Stdout = &gitOut
	err := gitCmd.Run()
	if err != nil {
		return nil, err
	}
	matches := reFnameUntrackedFiles.FindAllString(gitOut.String(), -1)
	results := make([]Change, len(matches))

	for i := range matches {
		change, err := newChange(reFnameUntrackedFiles.ReplaceAllString(matches[i], "${fname}"))
		if err != nil {
			return nil, err
		}
		results[i] = change
	}
	gitOut.Reset()

	// get changes in go files
	// Disallow external diff drivers.
	gitCmd = exec.Command("git", "-C", workdir, "diff", "--no-ext-diff")
	gitCmd.Stdout = &gitOut
	err = gitCmd.Run()
	if err != nil {
		return nil, err
	}
	matches = reFnameAndLinesInDiff.FindAllString(gitOut.String(), -1)
	for i := range matches {
		change, err := newChange(reFnameAndLinesInDiff.ReplaceAllString(matches[i], "${fname} ${old} ${new}"))
		if err != nil {
			return nil, err
		}
		results = append(results, change)
	}

	return results, nil
}
