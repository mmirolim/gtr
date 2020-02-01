package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
)

var (
	delay           = flag.Int("delay", 1000, "delay in Milliseconds")
	testBinaryArgs  = flag.String("args", "", "arguments to pass to binary format -k1=v1 -k2=v2")
	excludePrefixes = flag.String("exclude-file-prefix", "flymake,#flymake", "prefixes to exclude sep by comma")
	excludeDirs     = flag.String("exclude-dir", "vendor,node_modules", "prefixes to exclude sep by comma")
	autoCommit      = flag.String("auto-commit", "", "auto commit on tests pass, default false")
)

// TODO add flag for turning of auto commit
// TODO output to stdout with gtr prefix
func main() {
	flag.Parse()
	workDir := "."
	logger := log.New(os.Stdout, "gtr: ", 0)
	diffStrategy := NewGitDiffStrategy(workDir, logger)
	notifier := NewDesktopNotificator(true, 2000)
	testRunner := NewGoTestRunner(
		diffStrategy,
		NewOsCommand,
		*testBinaryArgs,
		logger,
	)
	tasks := []Task{testRunner, notifier}
	if len(*autoCommit) > 0 {
		autoCommitTask := NewTask("AutoCommit",
			CommitChanges(workDir, NewOsCommand),
			logger)
		tasks = append(tasks, autoCommitTask)
		tasks = append(tasks, notifier)
	}
	watcher, err := NewWatcher(
		workDir,
		tasks,
		*delay,
		splitStr(*excludePrefixes, ","),
		splitStr(*excludeDirs, ","),
		logger,
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
