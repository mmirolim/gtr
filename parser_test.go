package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
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
				{"geo.go", 0, 0},
				{"math.go", 0, 0},
				{"math_test.go", 0, 0}},
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
			output:      []Change{{"main.go", 0, 0}},
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
			output:      []Change{{"geo.go", 1, 10}},
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
			output:      []Change{{"math.go", 12, 10}, {"math_test.go", 7, 10}},
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
			output:      []Change{{"math.go", 2, 7}},
		},
		{
			desc: "Update file math.go, change package level const and add comment",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("math.go"), mathgo_update_pkg_lvl_var_add_comment_change_func, 0600)
			},
			tearDown: func(desc string) error {
				return gitCmdRun("commit", "-am", desc)
			},
			expectedErr: "",
			output:      []Change{{"math.go", 1, 9}},
		},
		{
			desc: "Change func name in untracked file geo.go",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("geo.go"), geo_area_func_rename, 0600)
			},
			tearDown: func(desc string) error {
				return nil
			},
			expectedErr: "",
			output:      []Change{{"geo.go", 4, 7}},
		},
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

		if !reflect.DeepEqual(tc.output, output) {
			t.Errorf("case [%d] %s\nexpected %#v\ngot %#v", i, tc.desc, tc.output, output)
		}
	}
}

var testFile = []byte(`
package main

import (
    pk "github.com/name/pkga"
)

func TestMin(t *testing.T) {
	res := pk.TA.Min(10, 20)
	if res != 10 {
		t.Errorf("expected 10, got %v", res)
	}
}

func TestMax(t *testing.T) {
	res := max(10, 20)
	if res != 20 {
		t.Errorf("expected 20, got %v", res)
	}
	var m map[int]func()

	m[0]()
}
`)

func TestGetTestedFuncs(t *testing.T) {
	expected := map[string][]Entity{
		"TestMin": {
			{typ: "func", name: "pk.TA.Min"},
			{typ: "func", name: "t.Errorf"},
		},
		"TestMax": {
			{typ: "func", name: "max"},
			{typ: "func", name: "t.Errorf"},
		},
	}

	dic, err := getTestedFuncs(testFile)
	if err != nil {
		t.Errorf("unexpected error %v", err)
		return
	}
	diffs := pretty.Diff(expected, dic)
	if len(diffs) > 0 {
		t.Errorf("%# v", pretty.Formatter(diffs))
	}
}

func TestFnFullName(t *testing.T) {
	cases := []struct {
		callExpr *ast.CallExpr
		output   string
		err      string
	}{
		{callExpr: &ast.CallExpr{
			Fun: &ast.Ident{
				NamePos: 213,
				Name:    "max",
				Obj:     nil,
			}},
			output: "max",
		},
		{callExpr: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					NamePos: 133,
					Name:    "t",
					Obj:     nil,
				},
				Sel: &ast.Ident{
					NamePos: 135,
					Name:    "Errorf",
					Obj:     nil,
				},
			}},
			output: "t.Errorf",
		},
		{callExpr: &ast.CallExpr{
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
			output: "pk.Type.Min",
		},
		{callExpr: &ast.CallExpr{
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
		},
			err: "unexpected value *ast.IndexExpr",
		},
	}

	var errOut string
	for i, tc := range cases {
		errOut = ""
		name, err := fnNameFromCallExpr(tc.callExpr)
		if err != nil {
			errOut = err.Error()
		}
		if errOut != tc.err {
			t.Errorf("case [%d]\nexpected error %#v\ngot %#v", i, tc.err, errOut)
			continue
		}
		if name != tc.output {
			t.Errorf("case [%d]\nexpected %#v\ngot %#v", i, tc.output, name)
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
	fileInfo, err := getFileInfo("gofile.go", gofile)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	fmt.Printf("%# v\n", pretty.Formatter(fileInfo)) // output for debug
	t.Errorf("%v", "TODO")
}
