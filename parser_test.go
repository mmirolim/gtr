package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

// test data
var maingo = []byte(`
var a, b int = 10, 20

func main() {
	fmt.Printf("%+v\n", add(a, b))
}

func add(a, b int) {
	return a + b
}
`)

var mathgo = []byte(`
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

var mathgo_update_pkg_lvl_var_add_comment = []byte(`
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
	if min(a, b) == a {
		return b
	}
	return a
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
		//fmt.Printf("run %+v\n", gitCmd.Args) // output for debug
		err := gitCmd.Run()
		//fmt.Printf("git output\n%+v\n", gitOut.String()) // output for debug
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
		output          []string
		expectedErr     string
	}{
		{
			desc: "Add new file, math.go",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("math.go"), mathgo, 0600)
			},
			tearDown: func(desc string) error {
				err := gitCmdRun("add", "math.go")
				if err != nil {
					return err
				}
				return gitCmdRun("commit", "-m", desc)
			},
			expectedErr: "",
			output:      []string{},
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
			output:      []string{"main.go -1,10 +0,0"},
		},
		{
			desc: "Update file math.go with new func max",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("math.go"), mathgo_add_func, 0600)
			},
			tearDown: func(desc string) error {
				return gitCmdRun("commit", "-am", desc)
			},
			expectedErr: "",
			output:      []string{"math.go -10,3 +10,10"},
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
			output:      []string{"math.go -1,5 +1,6"},
		},
		{
			desc: "Update file math.go, change package level const and add comment",
			setup: func(desc string) error {
				return ioutil.WriteFile(fileFullName("math.go"), mathgo_update_pkg_lvl_var_add_comment, 0600)
			},
			tearDown: func(desc string) error {
				return gitCmdRun("commit", "-am", desc)
			},
			expectedErr: "",
			output:      []string{"math.go -1,6 +1,7"},
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
			t.Errorf("case [%d] %s\nexpected error %v, got %v", i, tc.desc, tc.expectedErr, errOut)
			continue
		}

		if !reflect.DeepEqual(tc.output, output) {
			t.Errorf("case [%d] %s\nexpected %s got %s", i, tc.desc, tc.output, output)
		}
	}
}
