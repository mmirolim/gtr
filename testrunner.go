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
	TestsToRun(context.Context) (pkgPaths, tests, subTests []string, err error)
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
	pkgPaths, tests, subTests, err := tr.strategy.TestsToRun(ctx)
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
	if len(pkgPaths) == 0 {
		listArg = []string{"."}
	} else {
		listArg = pkgPaths
	}

	testNames := tr.joinTestAndSubtest(tests, subTests)
	// run tests
	// do not wait process to finish
	// in case of console blocking programs
	// -vet=off to improve speed
	// TODO if all test in same package, run only it
	testParams := []string{"test", "-v", "-vet", "off", "-failfast",
		"-cpu", strconv.Itoa(runtime.GOMAXPROCS(0)), "-run", testNames}

	testParams = append(testParams, listArg...)
	if len(tr.args) > 0 {
		testParams := append(testParams, "-args")
		testParams = append(testParams, tr.args)
	}
	cmd := tr.cmd(ctx, "go", testParams...)
	logStrList(tr.log, "Tests to run", tests, true)
	if len(subTests) > 0 {
		logStrList(tr.log, "Subtests to run", subTests, true)
	}
	tr.log.Println(">>", strings.Join(cmd.GetArgs(), " "))

	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	cmd.SetEnv(os.Environ())

	cmd.Run()
	msg := ""
	if cmd.Success() {
		msg = "Tests PASS: " + testNames
		tr.log.Println("\033[32mTests PASS\033[39m")
	} else {
		msg = "Tests FAIL: " + testNames
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
