package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kr/pretty"
)

func TestChangesToFileBlocks(t *testing.T) {
	f1Blocks := []FileBlock{
		{typ: BlockFunc, name: "main", start: 6, end: 8},
		{typ: BlockFunc, name: "add", start: 10, end: 12},
	}

	f2Blocks := []FileBlock{
		{typ: BlockFunc, name: "sub", start: 6, end: 8},
		{typ: BlockFunc, name: "min", start: 10, end: 16},
		{typ: BlockFunc, name: "min", start: 18, end: 23},
		{typ: BlockMethod, name: "Method", start: 25, end: 30},
	}

	fileInfos := map[string]FileInfo{
		"f1": {
			fname: "f1", pkgName: "main", endLine: 15, blocks: f1Blocks,
		},
		"f2": {
			fname: "f2", pkgName: "main", endLine: 32, blocks: f2Blocks,
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
				{"f1", "f1", 7, 6},
			},
			fileInfos: fileInfos,
			output: map[string]FileInfo{
				"f1": {"f1", "main", 15, f1Blocks},
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
				"f1": {"f1", "main", 15, []FileBlock{f1Blocks[0]}},
				"f2": {"f2", "main", 32, []FileBlock{f2Blocks[2], f2Blocks[3]}},
			},
		},
		{
			desc: "new untracked file added",
			changes: []Change{
				{"f1", "f1", 0, 0},
			},
			fileInfos: fileInfos,
			output: map[string]FileInfo{
				"f1": {"f1", "main", 15, f1Blocks},
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

var (
	gomod = []byte(`module git-diff-strategy-test-run

go 1.13
`)
	fileA = []byte(`package main
import "fmt"

var a, b int = 10, 20

func main() {
	fmt.Printf("%+v\n", add(a, b))
}

func add(a, b int) int {
	return a + b
}
`)

	fileB = []byte(`package main

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

	testFile = []byte(`package main

import "testing"

func TestAdd(t *testing.T) {
	if add(1, 2) != 3 {
		t.Error("unexpected result")
	}
}

func TestMinMaxAdd(t *testing.T) {
	cf := func(t *testing.T) {
		t.Error("error")
	}
	cf(t)
	t.Run("min", func(t *testing.T) {
		if min(1, 2) != 1 {
			t.Error("unexpected result")
		}
		cf2 := func(t *testing.T){
			if add(1, 2) != 3 {
				t.Error("unexpected result")
			}
		}
		cf2(t)
	})
	t.Run("max", helperMax)

	for _, tc := range []string{"test1", "test2"} {
		tc := tc // capture range variable
		t.Run(tc, func(t *testing.T) {
			if add(1, 2) != 3 {
				t.Error(tc)
			}
		})
	}
	t.Run("group", func(t *testing.T) {
		t.Run("group test 1", helperMax)
	})
}

func TestSub(t *testing.T) {
	if sub(add(1, 2), 1) != 2 {
		t.Error("unexpected result")
	}
}

func helperMax(t *testing.T) {
	if max(1, 2) != 2 {
		t.Error("unexpected result")
	}
}
`)
	fileAUpdateAdd = []byte(`package main

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
	fileBUpdateMax = []byte(`package main

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
	pkgAFile = []byte(`package pkga

import (
	rename "git-diff-strategy-test-run/pkgb"
)

type A struct {
	a int
}

func NewA() A {
	return A{}
}

func (a *A) MethodOnPointer(c int) int {
	return c + a.a
}

func (a A) MethodOnValue(c int) int {
	return c + a.a
}

func F() string {
	return "pkga.F()->" + rename.F()
}
`)
	pkgBFile = []byte(`package pkgb

type A struct {
	b int
}

func NewA() A {
	return A{}
}

func (a *A) MethodOnPointer() int {
	return a.b
}

func (a A) MethodOnValue() int {
	return a.b
}

func F() string {
	return "pkgb.F()"
}
`)
	pkgATestFile = []byte(`package pkga

import (
        "testing"
	rename "git-diff-strategy-test-run/pkgb"
)

func TestPkgAFunc(t *testing.T) {
	if F() !=  "pkga.F()->pkgb.F"{
		t.Error("unexpected result")
	}
}

func TestPkgAMethodOnPointer(t *testing.T) {
	a := NewA()
	if a.MethodOnPointer(10) !=  10 {
		t.Error("unexpected result")
	}
}

func TestPkgBMethodOnValue(t *testing.T) {
	a := rename.NewA()
	if a.MethodOnValue() !=  1 {
		t.Error("unexpected result")
	}
}
`)
	pkgBFileUpdateF = []byte(`package pkgb

	type A struct {
		b int
	}

	func NewA() A {
		return A{}
	}

	func (a *A) MethodOnPointer() int {
		return a.b
	}

	func (a A) MethodOnValue() int {
		return a.b
	}

	func F() string {
		return "pkgb.F():updated"
	}
	`)

	pkgBFileUpdateMethods = []byte(`package pkgb

	type A struct {
		b int
	}

	func NewA() A {
		return A{}
	}

	func (a *A) MethodOnPointer() int {
		return a.b + 1
	}

	func (a A) MethodOnValue() int {
		return a.b + 1
	}

	func F() string {
		return "pkgb.F():updated"
	}
	`)
)

func TestSSAStrategyTestsToRun(t *testing.T) {
	// setup
	testDir := filepath.Join(os.TempDir(), "test_ssa_strategy_tests_to_run")
	gitCmdRun := NewGitCmd(testDir)

	pkgAFilePath := filepath.Join("pkga", "f.go")
	pkgATestFilePath := filepath.Join("pkga", "f_test.go")
	pkgBFilePath := filepath.Join("pkgb", "f.go")

	files := map[string][]byte{
		"go.mod":    gomod,
		"file_a.go": fileA, "file_b.go": fileB, "file_test.go": testFile,
		pkgAFilePath: pkgAFile, pkgBFilePath: pkgBFile,
		pkgATestFilePath: pkgATestFile,
	}
	logger := log.New(os.Stdout, "gtr-test:", log.Ltime)
	setup := func() *SSAStrategy {
		setupTestGitDir(t,
			testDir, files,
			[]string{
				"go.mod", "file_a.go", "file_b.go", "file_test.go",
				pkgAFilePath, pkgBFilePath, pkgATestFilePath,
			},
		)
		// TODO test for different analysis types
		return NewSSAStrategy("pointer", testDir, logger)
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
			outTests: []string{"git-diff-strategy-test-run.TestAdd",
				"git-diff-strategy-test-run.TestMinMaxAdd",
				"git-diff-strategy-test-run.TestSub"},
			outSubTests: []string{"min"},
		},
		{
			desc: "Update file_b.go file max func",
			setup: func() error {
				return ioutil.WriteFile(
					filepath.Join(testDir, "file_b.go"), fileBUpdateMax, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "commit file_b.go changes") // Test
			},
			outTests:    []string{"git-diff-strategy-test-run.TestMinMaxAdd"},
			outSubTests: []string{"group test 1", "max"},
		},
		{
			desc: "Check named imports",
			setup: func() error {

				return ioutil.WriteFile(
					filepath.Join(testDir, pkgBFilePath), pkgBFileUpdateF, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "commit changes")
			},
			outTests: []string{"git-diff-strategy-test-run/pkga.TestPkgAFunc",
				"git-diff-strategy-test-run/pkga.TestPkgBMethodOnValue"},
			outSubTests: nil,
		},
		{
			desc: "Update pkgb.A type methods",
			setup: func() error {
				return ioutil.WriteFile(
					filepath.Join(testDir, pkgBFilePath),
					pkgBFileUpdateMethods, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "commit changes")
			},
			outTests:    []string{"git-diff-strategy-test-run/pkga.TestPkgBMethodOnValue"},
			outSubTests: nil,
		},
		// TODO add test with helper func in different packages
		// TODO add test with different testing frameworks
	}
	ssaStrategy := setup()
	for i, tc := range cases {
		// setup()
		execTestHelper(t, i, tc.desc, tc.setup)
		_, testsList, subTestsList, err := ssaStrategy.TestsToRun(context.Background())
		// teardown()
		execTestHelper(t, i, tc.desc, tc.tearDown)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}
		// TODO test packages are handled
		sort.Strings(tc.outTests)
		sort.Strings(testsList)
		if !reflect.DeepEqual(tc.outTests, testsList) {
			t.Errorf("case [%d] %s\nexpected Tests %+v\ngot %+v", i, tc.desc, tc.outTests, testsList)
		}

		sort.Strings(tc.outSubTests)
		sort.Strings(subTestsList)
		if !reflect.DeepEqual(tc.outSubTests, subTestsList) {
			t.Errorf("case [%d] %s\nexpected Subtests %+v\ngot %+v", i, tc.desc, tc.outSubTests, subTestsList)
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
	gitCmd := NewGitCmd(testDir)
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
