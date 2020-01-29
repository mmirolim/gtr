package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var (
	delay           = flag.Int("delay", 1000, "delay in Milliseconds")
	testBinaryArgs  = flag.String("args", "", "arguments to pass to binary format -k1=v1 -k2=v2")
	excludePrefixes = flag.String("exclude-file-prefix", "flymake,#flymake", "prefixes to exclude sep by comma")
	excludeDirs     = flag.String("exclude-dir", "vendor,node_modules", "prefixes to exclude sep by comma")
)

// TODO output to stdout with gtr prefix
func main() {
	flag.Parse()
	cmd := exec.Command("git", "status")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("git status error %+v\n", err) // output for debug
		os.Exit(1)
	}
	workDir := "."
	diffStrategy := NewGitDiffStrategy(workDir)
	notifier := NewDesktopNotificator(true, 2000)
	testRunner := NewGoTestRunner(
		diffStrategy,
		NewOsCommand,
		*testBinaryArgs,
		true,
	)

	watcher, err := NewWatcher(
		workDir,
		[]Task{testRunner, notifier},
		*delay,
		splitStr(*excludePrefixes, ","),
		splitStr(*excludeDirs, ","),
	)
	if err != nil {
		fmt.Printf("NewWatcher error %+v\n", err) // output for debug
		os.Exit(1)
	}
	// limit cpu usage
	runtime.GOMAXPROCS(runtime.NumCPU() / 2)
	err = watcher.Run()
	if err != nil {
		fmt.Printf("Watcher.Run error %+v\n", err) // output for debug
		os.Exit(1)
	}
}

func splitStr(str, sep string) []string {
	out := strings.Split(str, sep)
	for i := range out {
		out[i] = strings.Trim(out[i], " ")
	}
	return out
}
