package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
)

var _ Task = (*GoTestRunner)(nil)

// Runs go tests
type GoTestRunner struct {
	strategy  Strategy
	cmd       CommandCreator
	args      string
	showTests bool
}

func NewGoTestRunner(
	strategy Strategy,
	cmd CommandCreator,
	args string,
	showTests bool,
) *GoTestRunner {
	return &GoTestRunner{
		strategy:  strategy,
		cmd:       cmd,
		args:      args,
		showTests: showTests,
	}
}

func (tr *GoTestRunner) ID() string {
	return "GoTestRunner"
}

// TODO run tests in parallel per package?
func (tr *GoTestRunner) Run(ctx context.Context) (string, error) {
	tests, subTests, err := tr.strategy.TestsToRun()
	if err != nil {
		return "", fmt.Errorf("strategy error %v", err)
	}
	if len(tests) == 0 {
		return "no test found to run", nil
	}

	testNames := tr.joinTestAndSubtest(tests, subTests)
	// run tests
	// do not wait process to finish
	// in case of console blocking programs
	// -vet=off to improve speed
	// TODO if all test in same package, run only it
	cmd := tr.cmd(ctx, "go", "test", "-v", "-vet", "off", "-run",
		testNames, "./...", "-args", tr.args,
	)

	if tr.showTests {
		printStrList("Tests to run", tests, true)
		printStrList("Subtests to run", subTests, true)
		fmt.Println(">>", strings.Join(cmd.GetArgs(), " "))
	}

	cmd.SetStdout(os.Stdout)
	cmd.SetStderr(os.Stderr)
	cmd.SetEnv(os.Environ())

	err = cmd.Run()
	msg := "Tests PASS: " + testNames
	if !cmd.Success() {
		msg = "Tests FAIL: " + testNames
	}
	return msg, nil
}

func (tr *GoTestRunner) joinTestAndSubtest(tests, subTests []string) string {
	out := strings.Join(tests, "|")
	if len(subTests) != 0 {
		out += "/(" + strings.Join(subTests, "|") + ")"
	}
	return out
}

func printStrList(title string, tests []string, toSort bool) {
	var out []string
	if toSort {
		out = make([]string, len(tests))
		copy(out[0:], tests)
		sort.Strings(out)
	} else {
		out = tests
	}
	fmt.Printf("\n=============\n%s\n", title) // output for debug
	for i := range out {
		fmt.Printf("-> %+v\n", out[i]) // output for debug
	}
	fmt.Println("=============")
}
