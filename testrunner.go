package main

import (
	"fmt"
	"sort"
	"strings"
)

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
func (tr *GoTestRunner) Run(fname string, stop <-chan bool) (msg string, err error) {
	if !strings.HasSuffix(fname, ".go") {
		return "", ErrUnsupportedType
	}
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
	cmd := newCmd("go", []string{
		"test", "-v", "-vet", "off", "-run",
		testNames, "./...", "-args", tr.args,
	})
	fmt.Println(">>", strings.Join(cmd.Args, " "))
	err = cmd.Start()
	if err != nil {
		return
	}
	// TODO notify on test fail
	go func() {
		<-stop
		if cmd.ProcessState.Exited() {
			// already exited
			return
		}
		// kill process if already running
		// try to kill process
		err := cmd.Process.Kill()
		if err != nil {
			fmt.Println("test process kill returned error" + err.Error())
		}
	}()

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