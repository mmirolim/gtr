package main

import (
	"bytes"
	"os/exec"
	"regexp"
)

var reFnameAndLinesInDiff = regexp.MustCompile(`diff --git a/(?P<fname>.+\.go) (?:[^@]+|\n)+@@ (?P<old>-\d+,\d+) (?P<new>\+\d+,\d+) @@`)

var untrackedFiles = map[string]string{}

// TODO comments
func GetDiff(workdir string) ([]string, error) {
	// TODO store hashes of new files and return untracked new files to run
	var gitOut bytes.Buffer
	// Disallow external diff drivers.
	gitCmd := exec.Command("git", "-C", workdir, "diff", "--no-ext-diff")
	gitCmd.Stdout = &gitOut
	err := gitCmd.Run()
	if err != nil {
		return nil, err
	}
	matches := reFnameAndLinesInDiff.FindAllString(gitOut.String(), -1)
	results := make([]string, len(matches))
	for i := range matches {
		results[i] = reFnameAndLinesInDiff.ReplaceAllString(matches[i], "${fname} ${old} ${new}")
	}

	return results, nil
}
