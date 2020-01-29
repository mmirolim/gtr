package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
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
+       fmt.Printf("All matches %+v\n", matches) // output for debug
+
diff --git a/parser_test.go b/parser_test.go
index 7268a75..31a1203 100644
--- a/parser_test.go
+++ b/parser_test.go
@@ -341 +341 @@ func TestGetDiff(t *testing.T) {
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
				pkgName: "main",
				blocks: []FileBlock{
					{typ: "type", name: "T1", start: 6, end: 8},
					{typ: "method", name: "T1.M1", start: 10, end: 14},
					{typ: "method", name: "T1.M2", start: 16, end: 18},
					{typ: "func", name: "F2", start: 20, end: 22},
					{typ: "func", name: "Perimeter", start: 24, end: 26},
					{typ: "func", name: "Area", start: 28, end: 30},
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
