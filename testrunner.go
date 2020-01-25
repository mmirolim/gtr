package main

import (
	"fmt"
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
	tests, err := tr.strategy.TestsToRun()
	if err != nil {
		return "", fmt.Errorf("strategy error %v", err)
	}
	if len(tests) == 0 {
		return "no test found to run", nil
	}

	printTestNames(tests)

	testNames := strings.Join(tests, "|")
	// run tests
	// do not wait process to finish
	// in case of console blocking programs
	// -vet=off to improve speed
	cmd := newCmd("go", []string{
		"test", "-v", "-vet", "off", "-run",
		testNames, "./...", "-args", tr.args,
	})
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

func printTestNames(tests []string) {
	fmt.Println("=============\nTests to run") // output for debug
	for i := range tests {
		fmt.Printf("-> %+v\n", tests[i]) // output for debug
	}
	fmt.Println("=============")
}
