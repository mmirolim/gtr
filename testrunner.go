package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

var _ Task = (*GoTestRunner)(nil)

// Strategy interface defines provider of tests for the testrunner
type Strategy interface {
	CoverageEnabled() bool
	TestsToRun(context.Context) (runAll bool, tests, subTests []string, err error)
}

// GoTestRunner runs go tests
type GoTestRunner struct {
	strategy Strategy
	cmd      CommandCreator
	args     string
	log      *log.Logger
}

// NewGoTestRunner creates test runner
// strategy to use
// cmd creator
// args to runner
// logger for runner
func NewGoTestRunner(
	strategy Strategy,
	cmd CommandCreator,
	args string,
	logger *log.Logger,
) *GoTestRunner {
	return &GoTestRunner{
		strategy: strategy,
		cmd:      cmd,
		args:     args,
		log:      logger,
	}
}

// ID returns Task ID
func (tr *GoTestRunner) ID() string {
	return "GoTestRunner"
}

// Run method implements Task interface
// runs go tests
func (tr *GoTestRunner) Run(ctx context.Context) (string, error) {
	runAll, tests, subTests, err := tr.strategy.TestsToRun(ctx)
	if err != nil {
		if err == ErrBuildFailed {
			return "Build Failed", nil
		}
		return "", fmt.Errorf("strategy error %v", err)
	}
	if len(tests) == 0 && len(subTests) == 0 {
		return "No test found to run", nil
	}
	var listArg []string
	pkgPaths := map[string][]string{}
	for _, tname := range tests {
		id := strings.LastIndexByte(tname, '.')
		pkgPath := tname[:id]
		if pkgPath == "" {
			pkgPath = "."
		}
		pkgPaths[pkgPath] = append(pkgPaths[pkgPath], tname[id+1:])
	}

	// run tests
	// do not wait process to finish
	// in case of console blocking programs
	// -vet=off to improve speed
	// TODO handle run all
	msg := ""
	testParams := []string{"test", "-v", "-vet", "off", "-failfast",
		"-cpu", strconv.Itoa(runtime.GOMAXPROCS(0))}

	logStrList(tr.log, "Tests to run", tests, true)
	if len(subTests) > 0 {
		logStrList(tr.log, "Subtests to run", subTests, true)
	}
	var pkgList, testNames []string
	for k, pkgtests := range pkgPaths {
		pkgList = append(pkgList, k)
		testNames = append(testNames, pkgtests...)
	}
	testsFormated := tr.joinTestAndSubtest(testNames, subTests)
	var cmd CommandExecutor
	// TODO refactor
	if runAll {
		if tr.strategy.CoverageEnabled() {
			testParams = append(testParams, "-coverprofile")
			testParams = append(testParams, "coverage_profile")
		}
		testParams = append(testParams, "-run")
		testParams = append(testParams, testsFormated)
		testParams = append(testParams, listArg...)
		if len(tr.args) > 0 {
			testParams = append(testParams, "-args")
			testParams = append(testParams, tr.args)
		}
		cmd = tr.cmd(ctx, "go", testParams...)
		tr.log.Println(">>", strings.Join(cmd.GetArgs(), " "))

		cmd.SetStdout(os.Stdout)
		cmd.SetStderr(os.Stderr)
		cmd.SetEnv(os.Environ())
		cmd.Run()
	} else {
	OUTER: // run cmd for each test and skip subtests to have separation between tests
		for pkg, pkgtests := range pkgPaths {
			for _, tname := range pkgtests {
				testParams := []string{"test", "-v", "-vet", "off", "-failfast",
					"-cpu", strconv.Itoa(runtime.GOMAXPROCS(0))}

				if tr.strategy.CoverageEnabled() {
					testParams = append(testParams, "-coverprofile")
					testParams = append(testParams, fmt.Sprintf(".gtr/%s.%s",
						strings.ReplaceAll(pkg, "/", "_"),
						tname))
				}

				testParams = append(testParams, "-run")
				testParams = append(testParams, tname) // test
				testParams = append(testParams, pkg)   // package
				if len(tr.args) > 0 {
					testParams = append(testParams, "-args")
					testParams = append(testParams, tr.args) // test binary args
				}
				cmd = tr.cmd(ctx, "go", testParams...)
				tr.log.Println(">>", strings.Join(cmd.GetArgs(), " "))

				cmd.SetStdout(os.Stdout)
				cmd.SetStderr(os.Stderr)
				cmd.SetEnv(os.Environ())
				cmd.Run()
				if !cmd.Success() {
					// stop executing tests
					break OUTER
				}
			}
		}
	}

	if cmd.Success() {
		msg = "Tests PASS: " + testsFormated
		tr.log.Println("\033[32mTests PASS\033[39m")
	} else {
		msg = "Tests FAIL: " + testsFormated
		tr.log.Println("\033[31mTests FAIL\033[39m")
	}
	return msg, nil
}

// joinTestAndSubtest joins and format tests according to go test -run arg format
func (tr *GoTestRunner) joinTestAndSubtest(tests, subTests []string) string {
	out := strings.Join(tests, "$|")
	if len(out) > 0 {
		out += "$"
	}
	for i := range subTests {
		subTests[i] = strings.ReplaceAll(subTests[i], " ", "_")
	}
	if len(subTests) != 0 {
		out += "/(" + strings.Join(subTests, "|") + ")"
	}
	return out
}

func logStrList(log *log.Logger, title string, tests []string, toSort bool) {
	var out []string
	if toSort {
		out = make([]string, len(tests))
		copy(out[0:], tests)
		sort.Strings(out)
	} else {
		out = tests
	}

	log.Println("=============") // output for debug
	log.Println(title)
	for i := range out {
		log.Printf("-> %+v\n", out[i]) // output for debug
	}
	log.Println("=============")
}
