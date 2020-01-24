package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	delay           = flag.Int("delay", 1000, "delay in Milliseconds")
	showDebug       = flag.Bool("debug", true, "show debug information")
	testBinaryArgs  = flag.String("args", "", "arguments to pass to binary format -k1=v1 -k2=v2")
	excludePrefixes = flag.String("exclude", "flymake,#flymake", "prefixes to exclude sep by comma")
)

// FIX godef not working with gomodules
func main() {
	flag.Parse()
	cmd := exec.Command("git", "status")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("git status error %+v\n", err) // output for debug
		os.Exit(1)
	}
	debug = *showDebug
	workDir := "."
	gitcmd := NewGitCMD(workDir)
	store := NewIndex()
	diffStrategy, err := NewGitDiffStrategy(workDir, gitcmd, store)
	if err != nil {
		fmt.Printf("NewGitDiffStrategy error %+v\n", err) // output for debug
		os.Exit(1)
	}
	notifier := NewDesktopNotificator(true, 2000)
	testRunner := NewGoTestRunner(diffStrategy, *testBinaryArgs)
	watcher := NewWatcher(
		[]Task{testRunner},
		notifier,
		*delay, strings.Split(*excludePrefixes, ","),
	)
	watcher.Run()
}
