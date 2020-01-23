package main

import (
	"bytes"
	"io"
	"os/exec"
	"regexp"
	"strconv"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

var reFnameUntrackedFiles = regexp.MustCompile(`\?\? (?P<fname>[a-zA-Z0-9_\-/]+\.go)`)

type Change struct {
	fpathOld string // a/
	fpath    string // b/ current
	start    int
	count    int
}

// TODO handle rename/copy/delete, /dev/null is used to signal created or deleted files.
func changesFromGitDiff(diff bytes.Buffer) ([]Change, error) {
	var changes []Change
	var serr error
	skipLine := func() {
		for {
			r, _, err := diff.ReadRune()
			if err != nil {
				serr = err
				return
			}
			if r == '\n' {
				break
			}
		}
	}
	var line []rune
	consumeLine := func() {
		line = line[:0]
		for {
			r, _, err := diff.ReadRune()
			if err != nil {
				serr = err
				return
			}
			if r == '\n' {
				break
			}
			line = append(line, r)
		}
	}
	readTokenInLineAt := func(i int) (string, int) {
		start := i
		for ; i < len(line); i++ {
			if line[i] == ' ' || line[i] == ',' {
				break
			}
		}
		return string(line[start:i]), i
	}
	readFileNames := func() (a string, b string) {
		var prev rune
		for i := 1; i < len(line); i++ {
			prev = line[i-1]
			if prev == 'a' && line[i] == '/' {
				a, i = readTokenInLineAt(i + 1)
			} else if prev == 'b' && line[i] == '/' {
				b, i = readTokenInLineAt(i + 1)
			}
		}
		return
	}
	readStartLineAndCount := func() (d1, d2 int) {
		var number string
		var err error
		for i := 0; i < len(line); i++ {
			if line[i] == '+' {
				// read start line
				number, i = readTokenInLineAt(i + 1)
				d1, err = strconv.Atoi(number)
				if err != nil {
					serr = err
					return
				}
				if line[i] == ',' {
					// read line count
					number, i = readTokenInLineAt(i + 1)
					d2, err = strconv.Atoi(number)
					if err != nil {
						serr = err
						return
					}
				}
			}
		}
		return
	}
	var r rune
	var f1, f2 string
	for {
		r, _, serr = diff.ReadRune()
		if serr != nil {
			if serr == io.EOF {
				break
			}
			return nil, serr
		}
		if r == '+' || r == '-' {
			skipLine()
			continue
		} else {
			diff.UnreadRune()
		}
		consumeLine()
		if line[0] == 'd' && line[1] == 'e' { //deleted
			// skip deleted files
			f1, f2 = "", ""
			continue
		}
		if line[0] == 'd' {
			f1, f2 = readFileNames()
		} else if line[0] == '@' && f2 != "" {
			d1, d2 := readStartLineAndCount()
			changes = append(changes, Change{f1, f2, d1, d2})
		}

	}
	if serr == io.EOF {
		serr = nil
	}

	return changes, serr
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
		// TODO use /dev/null?
		changes = append(changes, Change{fname, fname, start, count})
	}
	return changes, nil
}

// TODO comments
// TODO maybe use existing git-diff parsers for unified format
// TODO do not use global states
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
	for i := range matches {
		fname := reFnameUntrackedFiles.ReplaceAllString(matches[i], "${fname}")
		results = append(results, Change{fname, fname, 0, 0})
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
	changes, err := changesFromGitDiff(gitOut)
	if err != nil {
		return nil, err
	}
	results = append(results, changes...)
	return results, nil
}
