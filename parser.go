package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

// get file name and patch lines in @@ -1 +2 @@ and @@ -1,2 +1,10 @@ format
var reFnameAndLinesInDiff = regexp.MustCompile(`diff --git a/(?P<fname>.+\.go) (?:[^@]+|\n)+@@ (?P<old>-\d+(?:,\d+)?) (?P<new>\+\d+(?:,\d+)?) @@`)
var reFnameUntrackedFiles = regexp.MustCompile(`\?\? (?P<fname>.+\.go)`)

var mu sync.Mutex
var untrackedFilesMap = map[string][]byte{}

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
		// -1 +1 changes in same line
		if len(lines) > 1 {
			change.count, err = strconv.Atoi(lines[1])
			if err != nil {
				return change, nil
			}
		}
	} else if nparts == 1 {
		change.fpath = parts[0]
	} else {
		return change, fmt.Errorf("wrong diff format %s", diff)
	}
	return change, nil
}

func diff(fname string, prevdata, data []byte) ([]Change, error) {
	patcher := dmp.New()
	t1, t2, _ := patcher.DiffLinesToChars(string(prevdata), string(data))
	diffs := patcher.DiffMain(t1, t2, true)
	patches := patcher.PatchMake(diffs)
	var changes []Change
	for i := range patches {
		start := patches[i].Start2
		count := patches[i].Length2
		if count > 0 {
			start++
		}
		if start == 0 && count == 0 {
			continue
		}
		changes = append(changes, Change{fname, start, count})
	}

	return changes, nil
}

// TODO comments
// TODO maybe use existing git-diff parsers for unified format
func GetDiff(workdir string) ([]Change, error) {
	// TODO store hashes of new files and return untracked new files to run
	var gitOut bytes.Buffer
	var results []Change
	// get not yet commited go files
	gitCmd := exec.Command("git", "-C", workdir, "status", "--short")
	gitCmd.Stdout = &gitOut
	err := gitCmd.Run()
	if err != nil {
		return nil, err
	}
	matches := reFnameUntrackedFiles.FindAllString(gitOut.String(), -1)
	untrackedFiles := make([]string, len(matches))
	for i := range matches {
		untrackedFiles[i] = reFnameUntrackedFiles.ReplaceAllString(matches[i], "${fname}")
	}
	gitOut.Reset()
	// update untracked files changes
	mu.Lock()
	var data []byte

	for _, name := range untrackedFiles {
		// new file store
		data, err = ioutil.ReadFile(workdir + "/" + name)
		if err != nil {
			err = fmt.Errorf("reading file %s error %v", name, err)
			break
		}
		prevdata, ok := untrackedFilesMap[name]
		if ok {
			changes, err := diff(name, prevdata, data)
			if err != nil {
				err = fmt.Errorf("diff err %+v", err)
				break
			}
			results = append(results, changes...)

		} else {
			// new entry
			results = append(results, Change{name, 0, 0})
		}
	}
	mu.Unlock()
	if err != nil {
		return nil, err
	}
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
