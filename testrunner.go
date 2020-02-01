package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

var _ Task = (*GoTestRunner)(nil)

// Runs go tests
type GoTestRunner struct {
	strategy Strategy
	cmd      CommandCreator
	args     string
	log      *log.Logger
}

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

func (tr *GoTestRunner) ID() string {
	return "GoTestRunner"
}

func (tr *GoTestRunner) Run(ctx context.Context) (string, error) {
	tests, subTests, err := tr.strategy.TestsToRun(ctx)
	if err != nil {
		if err == ErrBuildFailed {
			return "Build Failed", nil
		}
		return "", fmt.Errorf("strategy error %v", err)
	}
	if len(tests) == 0 && len(subTests) == 0 {
		return "No test found to run", nil
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

	logStrList(tr.log, "Tests to run", tests, true)
	if len(subTests) > 0 {
		logStrList(tr.log, "Subtests to run", subTests, true)
	}
	tr.log.Println(">>", strings.Join(cmd.GetArgs(), " "))

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
	out := strings.Join(tests, "$|")
	if len(out) > 0 {
		out += "$"
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
