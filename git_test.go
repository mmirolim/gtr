package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
	testDir := filepath.Join(os.TempDir(), "test_get_diff")
	filePath := func(fname string) string {
		return filepath.Join(testDir, fname)
	}
	files := map[string][]byte{
		"main.go": maingo,
	}
	gitCmdRun := GitCmdFactory(testDir)
	setup := func() {
		setupTestGitDir(t,
			testDir, files,
			[]string{"main.go"},
		)
	}

	tearDown := func() {
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(testDir)
		}
	}

	// prepare
	setup()
	defer tearDown()

	// cases
	cases := []struct {
		desc            string
		setup, tearDown func() error
		output          []Change
		expectedErr     error
	}{
		{
			desc: "Add new file, math.go, geo.go, math_test.go",
			setup: func() error {
				err := ioutil.WriteFile(filePath("math.go"), mathgo, 0600)
				if err != nil {
					return err
				}
				err = ioutil.WriteFile(filePath("geo.go"), geogo, 0600)
				if err != nil {
					return err
				}
				return ioutil.WriteFile(filePath("math_test.go"), math_test_go, 0600)
			},
			tearDown: func() error {
				err := gitCmdRun("add", "math.go")
				if err != nil {
					return err
				}
				err = gitCmdRun("add", "math_test.go")
				if err != nil {
					return err
				}
				return gitCmdRun("commit", "-m", "add files")
			},
			output: []Change{
				{"geo.go", "geo.go", 0, 0},
				{"math.go", "math.go", 0, 0},
				{"math_test.go", "math_test.go", 0, 0}},
		},
		{
			desc: "Delete old file main.go",
			setup: func() error {
				return os.Remove(filePath("main.go"))
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "changes")
			},
			output: []Change{{"geo.go", "geo.go", 0, 0}},
		},
		{
			desc: "Change untracked file geo.go, add func Area",
			setup: func() error {
				return ioutil.WriteFile(filePath("geo.go"), geo_add_area, 0600)
			},
			output: []Change{{"geo.go", "geo.go", 0, 0}},
		},
		{
			desc: "Commit untracked file geo.go",
			setup: func() error {
				err := gitCmdRun("add", "geo.go")
				if err != nil {
					return err
				}
				return gitCmdRun("commit", "-m", "add geo.go")
			},
			tearDown: nil,
			output:   nil,
		},
		{
			desc: "Update file math.go with new func max with test",
			setup: func() error {
				err := ioutil.WriteFile(filePath("math.go"), mathgo_add_func, 0600)
				if err != nil {
					return err
				}
				return ioutil.WriteFile(filePath("math_test.go"), math_test_go_test_max, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "changes")
			},
			output: []Change{{"math.go", "math.go", 15, 7},
				{"math_test.go", "math_test.go", 10, 7}},
		},
		{
			desc: "Update file math.go, update func min",
			setup: func() error {
				return ioutil.WriteFile(filePath("math.go"), mathgo_update_min_func, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "changes")
			},
			output: []Change{{"math.go", "math.go", 5, 0},
				{"math.go", "math.go", 13, 2},
				{"math.go", "math.go", 15, 0},
				{"math.go", "math.go", 18, 0},
			},
		},
		{
			desc: "Multiple updates to file math.go",
			setup: func() error {
				return ioutil.WriteFile(filePath("math.go"),
					mathgo_update_pkg_lvl_var_add_comment_change_func, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "changes")
			},
			output: []Change{{"math.go", "math.go", 4, 0},
				{"math.go", "math.go", 6, 0},
				{"math.go", "math.go", 18, 0},
				{"math.go", "math.go", 20, 0},
			},
		},
		{
			desc: "Change func name in file geo.go",
			setup: func() error {
				return ioutil.WriteFile(filePath("geo.go"), geo_area_func_rename, 0600)
			},
			output: []Change{{"geo.go", "geo.go", 8, 0}},
		},
	}

	for i, tc := range cases {
		// setup()
		execTestHelper(t, i, tc.desc, tc.setup)

		// should get line numbers by file and namespace
		output, err := GetDiff(context.Background(), testDir)

		// teardown()
		execTestHelper(t, i, tc.desc, tc.tearDown)
		if isUnexpectedErr(t, i, tc.desc, tc.expectedErr, err) {
			continue
		}

		diffs := pretty.Diff(tc.output, output)
		if len(diffs) > 0 {
			t.Errorf("case [%d] %s\nexpected %# v\ngot %# v", i, tc.desc, tc.output, output)
		}
	}
}

func TestCommitChangesTask(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "test_commit_changes_task")
	filePath := func(fname string) string {
		return filepath.Join(testDir, fname)
	}
	files := map[string][]byte{
		"main.go": maingo,
	}
	gitCmdRun := GitCmdFactory(testDir)
	setup := func() {
		setupTestGitDir(t,
			testDir, files,
			[]string{"main.go"},
		)
	}

	tearDown := func() {
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(testDir)
		}
	}

	// prepare
	setup()
	defer tearDown()

	// cases
	cases := []struct {
		desc            string
		ctx             context.Context
		in              string
		cmdErr          error
		cmdSuccess      bool
		setup, tearDown func() error
		commitCmdLine   string
		output          string
		expectedErr     error
	}{
		{
			desc:   "Add new file, math.go, geo.go, math_test.go",
			ctx:    context.Background(),
			in:     "Tests PASS: TestA$",
			cmdErr: nil, cmdSuccess: true,
			setup: func() error {
				_ = ioutil.WriteFile(filePath("math.go"), mathgo, 0600)
				_ = ioutil.WriteFile(filePath("geo.go"), geogo, 0600)
				return ioutil.WriteFile(filePath("math_test.go"), math_test_go, 0600)
			},
			tearDown: func() error {
				_ = gitCmdRun("add", "math.go", "math_test.go")
				return gitCmdRun("commit", "-m", "add files")
			},
			commitCmdLine: "git -C /tmp/test_commit_changes_task commit -m 'auto_commit! Perimeter TestMin min sub'",
			output:        "'auto_commit! Perimeter TestMin min sub'",
			expectedErr:   nil,
		},
		// TODO add more test case
	}

	for i, tc := range cases {
		// setup()
		execTestHelper(t, i, tc.desc, tc.setup)
		tc.ctx = context.WithValue(tc.ctx, prevTaskOutputKey, tc.in)
		cmd := NewMockCommand(tc.cmdErr, tc.cmdSuccess)
		output, err := CommitChanges(testDir, cmd.New)(tc.ctx)

		// teardown()
		execTestHelper(t, i, tc.desc, tc.tearDown)
		if isUnexpectedErr(t, i, tc.desc, tc.expectedErr, err) {
			continue
		}
		cmdLineStr := strings.Join(cmd.GetArgs(), " ")
		if tc.commitCmdLine != cmdLineStr {
			t.Errorf("case [%d] %s\nexpected %# v\ngot %# v", i, tc.desc, tc.commitCmdLine, cmdLineStr)
		}
		if tc.output != output {
			t.Errorf("case [%d] %s\nexpected %# v\ngot %# v", i, tc.desc, tc.output, output)
		}
	}
}
