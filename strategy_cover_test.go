package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestCoverStrategyTestsToRun(t *testing.T) {
	var (
		gomod = []byte(`module cover-strategy-test-run

		go 1.13
		`)
		mainFile = []byte(`package main

		import (
			"fmt"
                        "cover-strategy-test-run/pkga"
		)

		var a, b int = 10, 20

		func main() {
			fmt.Printf("%+v\n", add(a, b))
		}

		func sub(a, b int) int {
			return pkga.Sub(a, b)
		}
		`)
		fileA = []byte(`package main

		func add(a, b int) int {
			return a + b
		}

		func mul(a, b int) int {
			return a * b
		}
		`)
		mainTestFile = []byte(`package main

		import (
			"testing"
		)

		func TestAdd(t *testing.T) {
			if add(3, 4) != 7 {
				t.Error("add unexpected result")
			}
		}

		func TestMul(t *testing.T) {
			if mul(10, 5) != 50 {
				t.Error("mul unexpected result")
			}
		}
		`)

		pkgAFileA = []byte(`package pkga

		func Div(a, b int) int {
			return a / b
		}

		func Sub(a, b int) int {
			return a - b
		}
		`)
		pkgATestFile = []byte(`package pkga

		import (
			"testing"
		)

		func TestDiv(t *testing.T) {
			if Div(3, 2) != 1 {
				t.Error("Div unexpected result")
			}
		}

		func TestSub(t *testing.T) {
			if Sub(10, 5) != 5 {
				t.Error("Sub unexpected result")
			}
		}
		`)
		fileAAddDouble = []byte(`package main

		func add(a, b int) int {
			return a + b
		}

		func mul(a, b int) int {
			return a * b
		}

		func double(a int) int {
			return 2 * a
		}
		`)
		mainTestFileAddDouble = []byte(`package main

		import (
			"testing"
		)

		func TestAdd(t *testing.T) {
			if add(3, 4) != 7 {
				t.Error("add unexpected result")
			}
		}

		func TestMul(t *testing.T) {
			if mul(10, 5) != 50 {
				t.Error("mul unexpected result")
			}
		}

		func TestDouble(t *testing.T) {
			if double(1) != 2 {
				t.Error("double unexpected result")
			}
		}
		`)
		fileAChangeMul = []byte(`package main

		func add(a, b int) int {
			return a + b
		}

		func mul(a, b int) int {
			return a * b * 1
		}

		func double(a int) int {
			return 2 * a
		}
		`)
		pkgAfileAChangeSub = []byte(`package pkga

		func Div(a, b int) int {
			return a / b
		}

		func Sub(a, b int) int {
			return a - b + 0
		}`)
		testAddProf = []byte(`mode: set
cover-strategy-test-run/pkga/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/pkga/file_a.go:7.26,9.4 1 0
cover-strategy-test-run/file_a.go:3.26,5.4 1 1
cover-strategy-test-run/file_a.go:7.26,9.4 1 0
cover-strategy-test-run/file_a.go:11.26,13.4 1 0
cover-strategy-test-run/main.go:10.15,12.4 1 0
cover-strategy-test-run/main.go:14.26,16.4 1 0
`)
		testDivProf = []byte(`mode: set
cover-strategy-test-run/pkga/file_a.go:3.26,5.4 1 1
cover-strategy-test-run/pkga/file_a.go:7.26,9.4 1 0
cover-strategy-test-run/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/file_a.go:7.26,9.4 1 0
cover-strategy-test-run/file_a.go:11.26,13.4 1 0
cover-strategy-test-run/main.go:10.15,12.4 1 0
cover-strategy-test-run/main.go:14.26,16.4 1 0
`)

		testDoubleProf = []byte(`mode: set
cover-strategy-test-run/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/file_a.go:7.26,9.4 1 0
cover-strategy-test-run/file_a.go:11.26,13.4 1 1
cover-strategy-test-run/main.go:10.15,12.4 1 0
cover-strategy-test-run/main.go:14.26,16.4 1 0
cover-strategy-test-run/pkga/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/pkga/file_a.go:7.26,9.4 1 0
`)

		testMulProf = []byte(`mode: set
cover-strategy-test-run/main.go:10.15,12.4 1 0
cover-strategy-test-run/main.go:14.26,16.4 1 0
cover-strategy-test-run/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/file_a.go:7.26,9.4 1 1
cover-strategy-test-run/file_a.go:11.26,13.4 1 0
cover-strategy-test-run/pkga/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/pkga/file_a.go:7.26,9.4 1 0
`)
		testSubProf = []byte(`mode: set
cover-strategy-test-run/pkga/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/pkga/file_a.go:7.26,9.4 1 1
cover-strategy-test-run/file_a.go:3.26,5.4 1 0
cover-strategy-test-run/file_a.go:7.26,9.4 1 0
cover-strategy-test-run/file_a.go:11.26,13.4 1 0
cover-strategy-test-run/main.go:10.15,12.4 1 0
cover-strategy-test-run/main.go:14.26,16.4 1 0
`)
	)
	// setup
	testDir := filepath.Join(os.TempDir(), "test_cover_strategy_tests_to_run")
	gitCmdRun := NewGitCmd(testDir)

	pkgAFileAPath := filepath.Join("pkga", "file_a.go")
	pkgATestFileAPath := filepath.Join("pkga", "file_a_test.go")

	files := map[string][]byte{
		"go.mod":  gomod,
		"main.go": mainFile, "file_a.go": fileA, "main_test.go": mainTestFile,
		pkgAFileAPath: pkgAFileA, pkgATestFileAPath: pkgATestFile,
	}
	logger := log.New(os.Stdout, "gtr-cover-strategy-test:", log.Ltime)
	setup := func() *CoverStrategy {
		setupTestGitDir(t,
			testDir, files,
			[]string{
				"go.mod", "main.go", "main_test.go", "file_a.go",
				pkgAFileAPath, pkgATestFileAPath,
			},
		)
		return NewCoverStrategy(testDir, logger)
	}
	// teardown
	defer func() {
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(testDir)
		}
	}()
	cases := []struct {
		desc            string
		setup, tearDown func() error
		outTests        []string
		err             error
	}{
		{
			desc: "No changes in files, no cover profiles",
			outTests: []string{"cover-strategy-test-run.TestAdd",
				"cover-strategy-test-run.TestMul",
				"cover-strategy-test-run/pkga.TestDiv",
				"cover-strategy-test-run/pkga.TestSub"},
		},
		{
			desc: "Add func double in file_a.go",
			setup: func() error {
				return ioutil.WriteFile(
					filepath.Join(testDir, "file_a.go"), fileAAddDouble, 0600)
			},
			tearDown: func() error {
				files := [][]byte{
					[]byte("cover-strategy-test-run.TestAdd"),
					testAddProf,
					[]byte("cover-strategy-test-run.TestMul"),
					testMulProf,
					[]byte("cover-strategy-test-run.TestDouble"),
					testDoubleProf,
					[]byte("cover-strategy-test-run_pkga.TestSub"),
					testSubProf,
					[]byte("cover-strategy-test-run_pkga.TestDiv"),
					testDivProf,
				}
				for i := 0; i < len(files); i += 2 {
					err := ioutil.WriteFile(
						filepath.Join(testDir, ".gtr", string(files[i])),
						files[i+1], 0600)
					if err != nil {
						return err
					}
				}
				return gitCmdRun("commit", "-am", "changes")
			},
			outTests: nil,
		},
		{
			desc: "Add test to Double",
			setup: func() error {
				return ioutil.WriteFile(
					filepath.Join(testDir, "main_test.go"),
					mainTestFileAddDouble, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "commit changes") // Test
			},
			outTests: []string{"cover-strategy-test-run.TestDouble"},
		},
		{
			desc: "Update mul func in file_a.go and sub in pkga/file_a.go",
			setup: func() error {
				err := ioutil.WriteFile(
					filepath.Join(testDir, "file_a.go"),
					fileAChangeMul, 0600)
				if err != nil {
					return err
				}
				return ioutil.WriteFile(
					filepath.Join(testDir, pkgAFileAPath),
					pkgAfileAChangeSub, 0600)
			},
			tearDown: func() error {
				return gitCmdRun("commit", "-am", "commit changes")
			},
			outTests: []string{"cover-strategy-test-run.TestMul",
				"cover-strategy-test-run/pkga.TestSub"},
		},
		// TODO add test with helper func in different packages
		// TODO add test with different testing frameworks
	}
	coverStrategy := setup()
	for i, tc := range cases {
		// setup()
		execTestHelper(t, i, tc.desc, tc.setup)
		_, testsList, _, err := coverStrategy.TestsToRun(context.Background())
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

	}

}
