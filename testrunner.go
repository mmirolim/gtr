package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

var _ Task = (*GoTestRunner)(nil)

// Runs go tests
type GoTestRunner struct {
	strategy Strategy
	args     string
}

func NewGoTestRunner(strategy Strategy, args string) *GoTestRunner {
	return &GoTestRunner{strategy, args}
}

func (tr *GoTestRunner) ID() string {
	return "GoTestRunner"
}

// TODO run tests in parallel per package?
func (tr *GoTestRunner) Run(ctx context.Context) (msg string, err error) {
	tests, subTests, err := tr.strategy.TestsToRun()
	if err != nil {
		return "", fmt.Errorf("strategy error %v", err)
	}
	if len(tests) == 0 {
		return "no test found to run", nil
	}

	printStrList("Tests to run", tests, true)
	printStrList("Subtests to run", subTests, true)

	testNames := tr.joinTestAndSubtest(tests, subTests)
	// run tests
	// do not wait process to finish
	// in case of console blocking programs
	// -vet=off to improve speed
	// TODO if all test in same package, run only it
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-vet", "off", "-run",
		testNames, "./...", "-args", tr.args,
	)
	fmt.Println(">>", strings.Join(cmd.Args, " "))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	err = cmd.Start()
	if err != nil {
		return
	}
	err = cmd.Wait()
	if cmd.ProcessState.Success() {
		msg = "Tests PASS: " + testNames
	} else {
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
