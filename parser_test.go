package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
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

func TestFnNameFromCallExpr(t *testing.T) {
	cases := []struct {
		callExpr *ast.CallExpr
		output   string
		err      error
	}{
		{
			&ast.CallExpr{
				Fun: &ast.Ident{
					Name: "F1",
				},
			}, "F1", nil},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "t",
					},
					Sel: &ast.Ident{
						Name: "Error",
					},
				},
			}, "t.Error", nil},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "pkg1",
					},
					Sel: &ast.Ident{
						Name: "F2",
					},
				},
			}, "pkg1.F2", nil,
		},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "pkgc",
					},
					Sel: &ast.Ident{
						Name: "Diff",
					},
				},
			}, "pkgc.Diff", nil,
		},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.SelectorExpr{
						X: &ast.Ident{
							NamePos: 95,
							Name:    "pk",
						},
						Sel: &ast.Ident{
							NamePos: 98,
							Name:    "Type",
						},
					},
					Sel: &ast.Ident{
						NamePos: 103,
						Name:    "Min",
					},
				}},
			"pk.Type.Min", nil,
		},
		{
			&ast.CallExpr{
				Fun: &ast.IndexExpr{
					X: &ast.Ident{
						NamePos: 307,
						Name:    "m",
						Obj: &ast.Object{
							Kind: 4,
							Name: "m",
							Data: int(0),
							Type: nil,
						},
					},
				},
			}, "", errors.New("unexpected value *ast.IndexExpr"),
		},
	}
	for i, tc := range cases {
		fnName, err := fnNameFromCallExpr(tc.callExpr)
		if isUnexpectedErr(t, i, "", tc.err, err) {
			continue
		}
		if fnName != tc.output {
			t.Errorf("case [%d]\nexpected %#v\ngot %#v", i, tc.output, fnName)
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
			osArgs: []string{"./binary", "-delay", "10", "-exclude-file-prefix", "h,v,#",
				"-exclude-dirs", "vendor,node_modules", "-auto-commit", "t", "-args",
				"-tf1", "10", "-tf2", "20,30"},
			out: config{
				delay:             10,
				excludeFilePrefix: []string{"h", "v", "#"},
				excludeDirs:       []string{"vendor", "node_modules"},
				autoCommit:        "t",
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
