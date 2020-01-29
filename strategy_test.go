package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kr/pretty"
)

func TestChangesToFileBlocks(t *testing.T) {
	f1_Blocks := []FileBlock{
		{typ: "func", name: "main", start: 6, end: 8},
		{typ: "func", name: "add", start: 10, end: 12},
	}

	f2_Blocks := []FileBlock{
		{typ: "func", name: "sub", start: 6, end: 8},
		{typ: "func", name: "min", start: 10, end: 16},
		{typ: "func", name: "min", start: 18, end: 23},
		{typ: "method", name: "Method", start: 25, end: 30},
	}

	fileInfos := map[string]FileInfo{
		"f1": FileInfo{
			fname: "f1", pkgName: "main", blocks: f1_Blocks,
		},
		"f2": FileInfo{
			fname: "f2", pkgName: "main", blocks: f2_Blocks,
		},
	}
	cases := []struct {
		desc      string
		changes   []Change
		fileInfos map[string]FileInfo
		output    map[string]FileInfo
		err       error
	}{
		{desc: "no file changes"},
		{
			desc: "file f1, 1 block changed",
			changes: []Change{
				{"f1", "f1", 10, 2},
			},
			fileInfos: fileInfos,
			output: map[string]FileInfo{
				"f1": {"f1", "main", f1_Blocks[1:]},
			},
		},
		{
			desc: "3 changes in 2 files",
			changes: []Change{
				{"", "f1", 3, 7},
				{"f2", "f2", 1, 1},
				{"", "f2", 20, 25},
			},
			fileInfos: fileInfos,
			output: map[string]FileInfo{
				"f1": {"f1", "main", []FileBlock{f1_Blocks[0]}},
				"f2": {"f2", "main", []FileBlock{f2_Blocks[2], f2_Blocks[3]}},
			},
		},
		{
			desc: "new untracked file added",
			changes: []Change{
				{"f1", "f1", 0, 0},
			},
			fileInfos: fileInfos,
			output: map[string]FileInfo{
				"f1": {"f1", "main", f1_Blocks},
			},
		},
	}

	for i, tc := range cases {
		out, err := changesToFileBlocks(tc.changes, tc.fileInfos)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}
		diff := pretty.Diff(tc.output, out)
		if len(diff) > 0 {
			t.Errorf("case [%d]\ndiff %+v", i, diff)
		}
	}

}

func TestGitDiffStrategyTestRun(t *testing.T) {
	var gomod = []byte(`module git-diff-strategy-test-run

go 1.13
`)
	var fileA = []byte(`package main

import "fmt"

var a, b int = 10, 20

func main() {
	fmt.Printf("%+v\n", add(a, b))
}

func add(a, b int) int {
	return a + b
}
`)

	var fileB = []byte(`package main

const PI = 3.14

func sub(a, b int) int {
	return a - b
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func max(a, b int) int {
	if b > a {
		return b
	}
	return a
}
`)

	var testFile = []byte(`package main

import "testing"

func TestAdd(t *testing.T) {
	if add(1, 2) != 3 {
		t.Error("unexpected result")
	}
}

func TestMinMax(t *testing.T) {
	t.Run("min", func(t *testing.T) {
		if min(1, 2) != 1 {
			t.Error("unexpected result")
		}
	})
	t.Run("max", func(t *testing.T) {
		if max(1, 2) != 2 {
			t.Error("unexpected result")
		}
	})
}

func TestSub(t *testing.T) {
	if sub(add(1, 2), 1) != 2 {
		t.Error("unexpected result")
	}
}
`)
	fileAUpdateAdd := []byte(`package main

import "fmt"

var a, b int = 10, 20

func main() {
	fmt.Printf("%+v\n", add(a, b))
}

func add(a, b int) int {
	// add comment
	return a + b
}
`)
	var fileBUpdateMax = []byte(`package main

const PI = 3.14

func sub(a, b int) int {
	return a - b
}

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
`)
	var pkgAFile = []byte(`package pkga

import (
	rename "git-diff-strategy-test-run/pkgb"
)

func F() string {
	return "pkga.F()->" + rename.F()
}
`)
	var pkgBFile = []byte(`package pkgb

func F() string {
	return "pkgb.F()"
}
`)
	var pkgATestFile = []byte(`package pkga
import "testing"

func TestPkgAFunc(t *testing.T) {
	if F() !=  "pkga.F()->pkgb.F"{
		t.Error("unexpected result")
	}
}
`)
	var pkgBFileUpdate = []byte(`package pkgb

func F() string {
	return "pkgb.F():updated"
}
`)

	pkgAFilePath := filepath.Join("pkga", "f.go")
	pkgATestFilePath := filepath.Join("pkga", "f_test.go")
	pkgBFilePath := filepath.Join("pkgb", "f.go")

	// setup
	testDir := filepath.Join(os.TempDir(), "test_diff_strategy_tests_to_run")
	gitCmdRun := GitCmdFactory(testDir)

	files := map[string][]byte{
		"go.mod":    gomod,
		"file_a.go": fileA, "file_b.go": fileB, "file_test.go": testFile,
		pkgAFilePath: pkgAFile, pkgBFilePath: pkgBFile,
		pkgATestFilePath: pkgATestFile,
	}
	setup := func() *GitDiffStrategy {
		setupTestGitDir(t,
			testDir, files,
			[]string{
				"go.mod", "file_a.go", "file_b.go", "file_test.go",
				pkgAFilePath, pkgBFilePath, pkgATestFilePath,
			},
		)
		return NewGitDiffStrategy(testDir)
	}

	// teardown
	defer func() {
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(testDir)
		}
	}()
	cases := []struct {
		desc                  string
		setup, tearDown       func() error
		outTests, outSubTests []string
		err                   error
	}{
		{desc: "No changes in files"},
		{
			desc: "Update file_a.go file",
			setup: func() error {
				return ioutil.WriteFile(
					filepath.Join(testDir, "file_a.go"), fileAUpdateAdd, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "commit file_a.go changes")
			},
			outTests: []string{"TestAdd$", "TestSub$"}, outSubTests: []string{}, // should be nil?
		},
		{
			desc: "Update file_b.go file max func",
			setup: func() error {
				return ioutil.WriteFile(
					filepath.Join(testDir, "file_b.go"), fileBUpdateMax, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "commit file_b.go changes")
			},
			outTests: []string{"TestMinMax$"}, outSubTests: []string{"max"},
		},
		{
			desc: "Check named imports",
			setup: func() error {
				return ioutil.WriteFile(
					filepath.Join(testDir, pkgBFilePath), pkgBFileUpdate, 0600)
			},
			outTests: []string{"TestPkgAFunc$"}, outSubTests: []string{},
		},
	}
	gitDiffStrategy := setup()
	for i, tc := range cases {
		// setup()
		execTestHelper(t, i, tc.desc, tc.setup)

		testsList, subTestsList, err := gitDiffStrategy.TestsToRun(context.Background())

		// teardown()
		execTestHelper(t, i, tc.desc, tc.tearDown)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}

		sort.Strings(tc.outTests)
		sort.Strings(testsList)
		if !reflect.DeepEqual(tc.outTests, testsList) {
			t.Errorf("case [%d]\nexpected Tests %+v\ngot %+v", i, tc.outTests, testsList)
		}

		sort.Strings(tc.outSubTests)
		sort.Strings(subTestsList)
		if !reflect.DeepEqual(tc.outSubTests, subTestsList) {
			t.Errorf("case [%d]\nexpected Subtests %+v\ngot %+v", i, tc.outSubTests, subTestsList)
		}
	}

}

func setupTestGitDir(t *testing.T, testDir string, files map[string][]byte, filesToCommit []string) {
	t.Helper()
	// check that we are working in TempDir
	if !strings.HasPrefix(testDir, os.TempDir()) {
		t.Fatalf("expected test dir be in %s, got %s", os.TempDir(), testDir)
	}
	_ = os.RemoveAll(testDir)
	err := os.Mkdir(testDir, 0700)
	if err != nil {
		t.Fatalf("setup Mkdir error %s", err)
	}

	for fname, fdata := range files {
		path, _ := filepath.Split(fname)
		if path != "" {
			err = os.MkdirAll(filepath.Join(testDir, path), 0700)
			if err != nil {
				t.Fatalf("setup MkdirAll error %s", err)
			}
		}
		err = ioutil.WriteFile(filepath.Join(testDir, fname), fdata, 0600)
		if err != nil {
			t.Fatalf("setup write error %v", err)
		}
	}
	gitCmd := GitCmdFactory(testDir)
	// init git
	err = gitCmd("init")
	if err != nil {
		t.Fatalf("setup git init error %s", err)
	}
	if len(filesToCommit) == 0 {
		return
	}
	for i := range filesToCommit {
		err = gitCmd("add", filesToCommit[i])
		if err != nil {
			t.Fatalf("setup git add error %v", err)
		}
	}
	err = gitCmd("commit", "-m", "init commit")
	if err != nil {
		t.Fatalf("setup git commit error %v", err)
	}

}

// wrapper to setup/teardown helper in test cases
func execTestHelper(t *testing.T, testID int, desc string, helper func() error) {
	t.Helper()
	if helper == nil {
		return
	}
	err := helper()
	if err != nil {
		t.Fatalf("case [%d] %s\nhelper func failed, unexpected error %v", testID, desc, err)
	}
}

func isUnexpectedErr(t *testing.T, caseID int, desc string, expectedErr, goterr error) bool {
	t.Helper()
	var eStr, gotStr string
	if expectedErr != nil {
		eStr = expectedErr.Error()
	}
	if goterr != nil {
		gotStr = goterr.Error()
	}

	if eStr != gotStr {
		t.Errorf("case [%d] %s\nexpected error \"%s\"\ngot \"%s\"", caseID, desc, eStr, gotStr)
		return true
	}
	return false

}
