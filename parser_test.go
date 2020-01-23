package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"

	"github.com/kr/pretty"
)

// test data
var maingo = []byte(`
package main

var a, b int = 10, 20

func main() {
	fmt.Printf("%+v\n", add(a, b))
}

func add(a, b int) {
	return a + b
}
`)

var mathgo = []byte(`
package main

const PI = 3.14
func sub(a, b) int {
	return a - b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
`)

var mathgo_add_func = []byte(`
package main

const PI = 3.14
func sub(a, b) int {
	return a - b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if min(a, b) == a {
		return b
	}
	return a
}
`)

var mathgo_update_min_func = []byte(`
package main

const PI = 3.14

func sub(a, b) int {
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
	if min(a, b) == a {
		return b
	}
	return a
}
`)

var mathgo_update_pkg_lvl_var_add_comment_change_func = []byte(`
package main

const PI = 3.14159

// sub func subtracts b from a
func sub(a, b) int {
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

var math_test_go = []byte(`
package main

func TestMin(t *testing.T) {
	res := min(10, 20)
	if res != 10 {
		t.Errorf("expected 10, got %v", res)
	}
}
`)

var math_test_go_test_max = []byte(`
package main

func TestMin(t *testing.T) {
	res := min(10, 20)
	if res != 10 {
		t.Errorf("expected 10, got %v", res)
	}
}

func TestMax(t *testing.T) {
	res := max(10, 20)
	if res != 20 {
		t.Errorf("expected 20, got %v", res)
	}
}
`)

var geogo = []byte(`
package main

func Perimeter(d, h int) int {
	return 2*d + 2*h
}
`)

var geo_add_area = []byte(`
package main

func Perimeter(d, h int) int {
	return 2*d + 2*h
}

func Area(d, h int) int {
	return d * h
}
`)

var geo_area_func_rename = []byte(`
package main

func Perimeter(d, h int) int {
	return 2*d + 2*h
}

func AreaRect(d, h int) int {
	return d * h
}
`)

func TestGetDiff(t *testing.T) {
	const testGetDiff = "test_get_diff"
	tempDir := os.TempDir()
	newTestDir := tempDir + "/" + testGetDiff
	fileFullName := func(fname string) string {
		return newTestDir + "/" + fname
	}
	var gitOut bytes.Buffer

	gitCmdRun := func(args ...string) error {
		gitOut.Reset()
		gitCmd := exec.Command("git", "-C", newTestDir)
		gitCmd.Stdout = &gitOut
		gitCmd.Args = append(gitCmd.Args, args...)
		err := gitCmd.Run()
		if err != nil {
			return err
		}
		return nil
	}
	setup := func() {
		_ = os.RemoveAll(newTestDir)
		err := os.Mkdir(newTestDir, 0700)
		if err != nil {
			t.Fatalf("setup Mkdir error %s", err)
		}
		// init git
		err = gitCmdRun("init")
		if err != nil {
			t.Fatalf("setup git init error %s", err)
		}
		err = ioutil.WriteFile(newTestDir+"/main.go", maingo, 0600)
		if err != nil {
			t.Fatalf("setup main.go write error %v", err)
		}
		err = gitCmdRun("add", "main.go")
		if err != nil {
			t.Fatalf("setup git add main.go %v", err)
		}
		err = gitCmdRun("commit", "-m", "add main.go")
		if err != nil {
			t.Fatalf("setup git commit main.go %v", err)
		}
	}

	tearDown := func() {
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(newTestDir)
		}
	}

	// prepare
	setup()
	defer tearDown()

	// cases
	cases := []struct {
		desc            string
		setup, tearDown func(desc string) error
		output          []Change
		expectedErr     string
	}{
		// TODO describe expected behavior
		{
			desc: "Add new file, math.go, geo.go, math_test.go",
			setup: func(desc string) error {
				err := ioutil.WriteFile(fileFullName("math.go"), mathgo, 0600)
				if err != nil {
					return err
				}
				err = ioutil.WriteFile(fileFullName("geo.go"), geogo, 0600)
				if err != nil {
					return err
				}
				return ioutil.WriteFile(fileFullName("math_test.go"), math_test_go, 0600)
			},
			tearDown: func(desc string) error {
				err := gitCmdRun("add", "math.go")
				if err != nil {
					return err
				}
				err = gitCmdRun("add", "math_test.go")
				if err != nil {
					return err
				}
				return gitCmdRun("commit", "-m", desc)
			},
			expectedErr: "",
			output: []Change{
				{"geo.go", "geo.go", 0, 0},
				{"math.go", "math.go", 0, 0},
				{"math_test.go", "math_test.go", 0, 0}},
		},
		{
			desc: "Delete old file main.go",
			setup: func(desc string) error {
				return os.Remove(fileFullName("main.go"))
			},
			tearDown: func(desc string) error {
				return gitCmdRun("commit", "-am", desc)
			},
			expectedErr: "",
			output:      []Change{{"geo.go", "geo.go", 0, 0}},
		},
		{
			desc: "Change untracked file geo.go, add func Area",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("geo.go"), geo_add_area, 0600)
			},
			tearDown: func(desc string) error {
				return nil
			},
			expectedErr: "",
			output:      []Change{{"geo.go", "geo.go", 0, 0}},
		},
		{
			desc: "Commit untracked file geo.go",
			setup: func(desc string) error {
				err := gitCmdRun("add", "geo.go")
				if err != nil {
					return err
				}
				return gitCmdRun("commit", "-m", desc)
			},
			tearDown: func(desc string) error {
				return nil
			},
			expectedErr: "",
			output:      nil,
		},
		{
			desc: "Update file math.go with new func max with test",
			setup: func(desc string) error {
				err := ioutil.WriteFile(fileFullName("math.go"), mathgo_add_func, 0600)
				if err != nil {
					return err
				}
				return ioutil.WriteFile(fileFullName("math_test.go"), math_test_go_test_max, 0600)
			},
			tearDown: func(desc string) error {
				return gitCmdRun("commit", "-am", desc)
			},
			expectedErr: "",
			output:      []Change{{"math.go", "math.go", 12, 10}, {"math_test.go", "math_test.go", 7, 10}},
		},
		{
			desc: "Update file math.go, update func min",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("math.go"), mathgo_update_min_func, 0600)
			},
			tearDown: func(desc string) error {
				return gitCmdRun("commit", "-am", desc)
			},
			expectedErr: "",
			output:      []Change{{"math.go", "math.go", 2, 7}, {"math.go", "math.go", 10, 12}},
		},
		{
			desc: "Multiple updates to file math.go",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("math.go"),
					mathgo_update_pkg_lvl_var_add_comment_change_func, 0600)
			},
			tearDown: func(desc string) error {
				return gitCmdRun("commit", "-am", desc)
			},
			expectedErr: "",
			output:      []Change{{"math.go", "math.go", 1, 9}, {"math.go", "math.go", 16, 8}},
		},
		{
			desc: "Change func name in file geo.go",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("geo.go"), geo_area_func_rename, 0600)
			},
			tearDown: func(desc string) error {
				return nil
			},
			expectedErr: "",
			output:      []Change{{"geo.go", "geo.go", 5, 6}},
		},
		// TODO add case with renaming file
	}

	var errOut string
	for i, tc := range cases {
		errOut = ""
		if err := tc.setup(tc.desc); err != nil {
			t.Errorf("case [%d] %s\nsetup failed, unexpected error %v", i, tc.desc, err)
			t.FailNow()
		}
		// should get line numbers by file and namespace
		output, err := GetDiff(newTestDir)
		if err != nil {
			errOut = err.Error()
		}
		err = tc.tearDown(tc.desc)
		if err != nil {
			t.Errorf("case [%d] %s\ntearDown failed unexpected error %v", i, tc.desc, err)
			t.FailNow()
		}
		if errOut != tc.expectedErr {
			t.Errorf("case [%d] %s\nexpected error %v\ngot %v", i, tc.desc, tc.expectedErr, errOut)
			continue
		}
		diffs := pretty.Diff(tc.output, output)

		if len(diffs) > 0 {
			t.Errorf("case [%d] %s\nexpected %# v\ngot %# v", i, tc.desc, tc.output, output)
		}
	}
}

func Test_changesFromGitDiff(t *testing.T) {
	cases := []struct {
		data   string
		output []Change
		err    string
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
-                               return ioutil.WriteFile(fileFullName("math.go"), mathgo_update_pkg_lvl_var_add_comment_change_func, 0600)
+                               return ioutil.WriteFile(fileFullName("math.go"),
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
			{fpathOld: "process_go_file_test.go", fpath: "process_go_file_test.go", start: 72, count: 22}}, err: ""},
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
`, output: nil /* no changes */, err: ""},
	}
	var buffer bytes.Buffer
	var errOut string
	for i, tc := range cases {
		buffer.Reset()
		errOut = ""
		_, err := buffer.WriteString(tc.data)
		if err != nil {
			t.Errorf("unexpected buffer.WriteString error %v", err)
			continue
		}
		changes, err := changesFromGitDiff(buffer)
		if err != nil {
			errOut = err.Error()
		}
		if errOut != tc.err {
			t.Errorf("case [%d]\nexpected error %#v\ngot %#v", i, tc.err, errOut)
			continue
		}

		diffs := pretty.Diff(tc.output, changes)
		if len(diffs) > 0 {
			t.Errorf("%# v", pretty.Formatter(diffs))
		}

	}
}
