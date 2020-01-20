package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"sync"
)

var reFnameAndLinesInDiff = regexp.MustCompile(`diff --git a/(?P<fname>.+\.go) (?:[^@]+|\n)+@@ (?P<old>-\d+,\d+) (?P<new>\+\d+,\d+) @@`)
var reFnameUntrackedFiles = regexp.MustCompile(`\?\? (?P<fname>.+\.go)`)

var mu sync.Mutex
var untrackedFiles = map[string]string{}

// TODO comments
func GetDiff(workdir string) ([]string, error) {
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
	results := make([]string, len(matches))

	for i := range matches {
		results[i] = reFnameUntrackedFiles.ReplaceAllString(matches[i], "${fname}")
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
		results = append(results, reFnameAndLinesInDiff.ReplaceAllString(matches[i], "${fname} ${old} ${new}"))
	}

	return results, nil
}
