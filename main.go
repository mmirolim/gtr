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
	excludePrefixes = flag.String("exclude-file-prefix", "flymake,#flymake", "prefixes to exclude sep by comma")
	excludeDirs     = flag.String("exclude-dir", "vendor,node_modules", "prefixes to exclude sep by comma")
)

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
	diffStrategy := NewGitDiffStrategy(gitcmd)
	notifier := NewDesktopNotificator(true, 2000)
	testRunner := NewGoTestRunner(
		diffStrategy,
		NewOsCommand,
		*testBinaryArgs,
		true,
	)

	watcher := NewWatcher(
		[]Task{testRunner, notifier},
		*delay,
		splitStr(*excludePrefixes, ","),
		splitStr(*excludeDirs, ","),
	)
	watcher.Run()
}

func splitStr(str, sep string) []string {
	out := strings.Split(str, sep)
	for i := range out {
		out[i] = strings.Trim(out[i], " ")
	}
	return out
}
