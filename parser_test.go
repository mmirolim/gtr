package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kr/pretty"
)

func TestChangesFromGitDiff(t *testing.T) {
	cases := []struct {
		data   string
		output []Change
		err    error
	}{
		{data: `diff --git a/parser.go b/parser.go
index 6452f09..de4ce2a 100644
--- a/parser.go
+++ b/parser.go
@@ -32,0 +33,2 @@ func changesFromGitDiff(diff string) ([]Change, error) {
        // comment
        // comment
+       fmt.Printf("All matches %+v\n", matches) // output for debug
+
}
         // comment
diff --git a/parser_test.go b/parser_test.go
index 7268a75..31a1203 100644
--- a/parser_test.go
+++ b/parser_test.go
@@ -341 +341 @@ func TestGetDiff(t *testing.T) {
//
-                       desc: "Update file math.go, change package level const and add comment",
+                       desc: "Multiple updates to file math.go",
@@ -343 +343,2 @@ func TestGetDiff(t *testing.T) {
-                               return ioutil.WriteFile(filePath("math.go"), mathgo_update_pkg_lvl_var_add_comment_change_func, 0600)
+                               return ioutil.WriteFile(filePath("math.go"),
+                                       mathgo_update_pkg_lvl_var_add_comment_change_func, 0600)
@@ -349 +350 @@ func TestGetDiff(t *testing.T) {
-                       output:      []Change{{"math.go", 1, 9}},
+                       output:      []Change{{"math.go", 0, 0}, {"math.go", 1, 9}},
diff --git a/process_go_file.go b/process_go_file.go
index 95a1cdd..d32a8ba 100644
--- a/process_go_file.go
+++ b/process_go_file.go
@@ -58 +57,0 @@ func parseTestFile(fname string) error {
-       fmt.Printf("parseTestFile %+v\n", fname) // output for debug
@@ -69,0 +69 @@ func parseTestFile(fname string) error {
+       // testFiles LOCK
@@ -72,0 +73 @@ func parseTestFile(fname string) error {
+       // testFiles UNLOCK
@@ -118,0 +120 @@ func indexFuncsInTestFile(fname string) {
+// TODO handle other types of entities like Types, Interfaces
@@ -166 +168 @@ func processFileChanges() (map[string]FileInfo, error) {
-       //fmt.Printf("Process changes\n%+v\n", changes) // output for debug
+       fmt.Printf("Process changes\n%+v\n", changes) // output for debug
diff --git a/process_go_file_test.go b/process_go_file_test.go
index d1f20de..c0bcbcd 100644
--- a/process_go_file_test.go
+++ b/process_go_file_test.go
@@ -5,0 +6,2 @@ import (
+
+       "github.com/kr/pretty"
@@ -69,0 +72,22 @@ func Test_fnNameFromCallExpr(t *testing.T) {
+       }
+}
`, output: []Change{
			{fpathOld: "parser.go", fpath: "parser.go", start: 33, count: 2},
			{fpathOld: "parser_test.go", fpath: "parser_test.go", start: 341, count: 0},
			{fpathOld: "parser_test.go", fpath: "parser_test.go", start: 343, count: 2},
			{fpathOld: "parser_test.go", fpath: "parser_test.go", start: 350, count: 0},
			{fpathOld: "process_go_file.go", fpath: "process_go_file.go", start: 57, count: 0},
			{fpathOld: "process_go_file.go", fpath: "process_go_file.go", start: 69, count: 0},
			{fpathOld: "process_go_file.go", fpath: "process_go_file.go", start: 73, count: 0},
			{fpathOld: "process_go_file.go", fpath: "process_go_file.go", start: 120, count: 0},
			{fpathOld: "process_go_file.go", fpath: "process_go_file.go", start: 168, count: 0},
			{fpathOld: "process_go_file_test.go", fpath: "process_go_file_test.go", start: 6, count: 2},
			{fpathOld: "process_go_file_test.go", fpath: "process_go_file_test.go", start: 72, count: 22}}},
		// deleted file
		{data: `diff --git a/main.go b/main.go
deleted file mode 100644
index 6e2c328..0000000
--- a/main.go
+++ /dev/null
@@ -1,12 +0,0 @@
-
-package main
-
-var a, b int = 10, 20
-
-func main() {
-	fmt.Printf("%+v\n", add(a, b))
-}
-
-func add(a, b int) {
-	return a + b
-}
`, output: nil},
	}
	var buffer bytes.Buffer
	for i, tc := range cases {
		buffer.Reset()
		buffer.WriteString(tc.data)

		changes, err := changesFromGitDiff(buffer)
		if isUnexpectedErr(t, i, "", tc.err, err) {
			continue
		}

		diffs := pretty.Diff(tc.output, changes)
		if len(diffs) > 0 {
			t.Errorf("%# v", pretty.Formatter(diffs))
		}

	}
}

var gofile = []byte(`
package main

import "github.com/pkg"

type T1 struct{
     a int
}

func (t *T1) M1(a, b int) int {
	z := a + b
	a += b
	return z + a + b
}

func (t T1) M2(a, b int) int {
	return b - a
}

func F2() int {
	return pkg.F()
}

func Perimeter(d, h int) int {
	return 2*d + 2*h
}

func Area(d, h int) int {
	return d * h
}
`)

func TestGetFileBlocks(t *testing.T) {
	cases := []struct {
		fileName string
		fileData []byte
		output   FileInfo
		err      error
	}{
		{
			fileName: "gofile.go", fileData: gofile, output: FileInfo{
				fname:   "gofile.go",
				pkgName: "main", endLine: 30,
				blocks: []FileBlock{
					{typ: BlockType, name: "T1", start: 6, end: 8},
					{typ: BlockMethod, name: "T1.M1", start: 10, end: 14},
					{typ: BlockMethod, name: "T1.M2", start: 16, end: 18},
					{typ: BlockFunc, name: "F2", start: 20, end: 22},
					{typ: BlockFunc, name: "Perimeter", start: 24, end: 26},
					{typ: BlockFunc, name: "Area", start: 28, end: 30},
				},
			},
		},
	}
	for i, tc := range cases {
		fileInfo, err := getFileInfo("gofile.go", gofile)
		if isUnexpectedErr(t, i, "", tc.err, err) {
			continue
		}

		diffs := pretty.Diff(tc.output, fileInfo)
		if len(diffs) > 0 {
			fmt.Printf("%# v\n", pretty.Formatter(fileInfo)) // output for debug
			t.Errorf("%# v", pretty.Formatter(diffs))
		}
	}
}

func TestParseFlag(t *testing.T) {
	cases := []struct {
		desc   string
		osArgs []string
		out    config
		err    error
	}{
		{
			desc:   "no flags are defined",
			osArgs: []string{"./binary"},
			out:    newConfig(),
			err:    nil,
		},
		{
			desc: "All flags are correctly defined",
			osArgs: []string{
				"./binary", "-C", "/home/user/go", "-strategy", "coverage",
				"-analysis", "cha", "-delay", "10", "-exclude-file-prefix", "h,v,#",
				"-exclude-dirs", "vendor,node_modules", "-auto-commit", "t", "-run-init", "false", "-args",
				"-tf1", "10", "-tf2", "20,30"},
			out: config{
				workDir:           "/home/user/go",
				strategy:          "coverage",
				analysis:          "cha",
				runInit:           false,
				delay:             10,
				excludeFilePrefix: []string{"h", "v", "#"},
				excludeDirs:       []string{"vendor", "node_modules"},
				autoCommit:        true,
				argsToTestBinary:  "-tf1 10 -tf2 20,30",
			},
			err: nil,
		},
		{
			desc: "Delay flag invalid",
			osArgs: []string{"./binary", "-delay", "10.1", "-args",
				"-tf1", "10", "-tf2", "20,30"},
			out: config{},
			err: errors.New("-delay invalid value 10.1"),
		},
		{
			desc:   "Flag value missing",
			osArgs: []string{"./binary", "-auto-commit", "t", "-exclude-dirs"},
			out:    config{},
			err:    errors.New("-exclude-dirs value missing"),
		},
		{
			desc:   "on help return usage",
			osArgs: []string{"./binary", "help"},
			out:    config{},
			err:    errors.New(flagUsage()),
		},
		{
			desc:   "test flag setting",
			osArgs: []string{"./binary", "-strategy=\"coverage\"", "-analysis=cha"},
			out: config{
				workDir:           ".",
				delay:             1000,
				strategy:          "coverage",
				runInit:           true,
				analysis:          "cha",
				excludeFilePrefix: []string{"#"},
				excludeDirs:       []string{"vendor", "node_modules"},
				autoCommit:        false,
				argsToTestBinary:  "",
			},
			err: nil,
		},
	}
	for i, tc := range cases {
		cfg, err := parseFlags(tc.osArgs)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}

		diffs := pretty.Diff(tc.out, cfg)
		if len(diffs) > 0 {
			t.Errorf("case [%d] %s\nunexpected result %# v", i, tc.desc, pretty.Formatter(diffs))
		}
	}

}

func TestGetModuleName(t *testing.T) {
	gomod := []byte(`module rock.com/solid

go 1.13

require (
	golang.org/x/tools v0.0.0-20190729092621-ff9f1409240a
)`)
	// setup
	testDir := filepath.Join(os.TempDir(), "test-get-module-name")
	workDir := filepath.Join(testDir, "src", "rockcom", "solid")
	err := os.MkdirAll(workDir, 0700)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// change dir
	err = os.Chdir(workDir)
	if err != nil {
		t.Fatal(err)
	}
	// teardown
	defer func() {
		// go back
		os.Chdir(dir)
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(testDir)
		}
	}()

	cases := []struct {
		desc            string
		gofile          []byte
		module          string
		err             error
		setup, teardown func() error
	}{
		{
			desc:   "Not a go module",
			module: "rockcom/solid",
			setup: func() error {
				return os.Setenv("GOPATH", testDir)
			},
		},
		{
			desc:   "Go module",
			gofile: gomod,
			module: "rock.com/solid",
			setup: func() error {
				return ioutil.WriteFile(filepath.Join(workDir, "go.mod"), gomod, 0600)
			},
			teardown: func() error {
				return os.Remove(filepath.Join(workDir, "go.mod"))
			},
		},
	}

	for i, tc := range cases {
		// setup
		execTestHelper(t, i, tc.desc, tc.setup)

		module, err := getModuleName(workDir)

		// teardown
		execTestHelper(t, i, tc.desc, tc.teardown)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}

		if !reflect.DeepEqual(tc.module, module) {
			t.Errorf("case [%d] %s\nexpected %s, got %s", i, tc.desc, tc.module, module)
		}
	}
}

func TestParseCoverProfile(t *testing.T) {
	testRunnerFile := []byte(`mode: set
mmirolim/gtr/strategy.go:329.38,331.15 1 0
mmirolim/gtr/strategy.go:333.28,335.7 1 0
mmirolim/gtr/testrunner.go:39.17,46.2 1 1
mmirolim/gtr/testrunner.go:49.37,51.2 1 0
mmirolim/gtr/testrunner.go:100.77,102.18 2 1
mmirolim/gtr/testrunner.go:105.2,105.24 1 1
mmirolim/gtr/testrunner.go:108.2,108.12 1 1
mmirolim/gtr/testrunner.go:102.18,104.3 1 1
mmirolim/gtr/testrunner.go:105.24,107.3 1 1
mmirolim/gtr/testrunner.go:111.77,113.12 2 1
mmirolim/gtr/testrunner.go:121.2,123.21 3 1
mmirolim/gtr/testrunner.go:126.2,126.30 1 1
mmirolim/gtr/testrunner.go:113.12,117.3 3 1
mmirolim/gtr/testrunner.go:117.8,119.3 1 0
mmirolim/gtr/testrunner.go:123.21,125.3 1 1
mmirolim/gtr/watcher.go:51.21,64.2 2 0
mmirolim/gtr/watcher.go:67.31,71.16 3 1
`)
	cases := []struct {
		desc    string
		data    []byte
		infoMap map[string]*FileCoverInfo
		err     error
	}{
		{
			desc: "No data",
			data: nil,
			err:  io.EOF,
		},
		{
			desc: "Cover profile data for multiple files",
			data: testRunnerFile,
			infoMap: map[string]*FileCoverInfo{
				"mmirolim/gtr/watcher.go": &FileCoverInfo{
					"mmirolim/gtr/watcher.go", [][2]int{{67, 71}},
				},
				"mmirolim/gtr/strategy.go": &FileCoverInfo{
					File: "mmirolim/gtr/strategy.go",
				},
				"mmirolim/gtr/testrunner.go": &FileCoverInfo{
					"mmirolim/gtr/testrunner.go",
					[][2]int{{39, 46}, {100, 104}, {105, 107}, {108, 108},
						{111, 117}, {121, 125}, {126, 126}},
				},
			},
		},
	}

	for i, tc := range cases {
		infos, err := ParseCoverProfile(tc.data)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}
		if err != nil {
			continue
		}

		diffs := pretty.Diff(tc.infoMap, infos)
		if len(diffs) > 0 {
			t.Errorf("case [%d] %s\nexpected %+v, got %+v", i, tc.desc, tc.infoMap, infos)
			fmt.Printf("Diffs %# v\n", pretty.Formatter(diffs)) // output for debug

		}
	}
}
