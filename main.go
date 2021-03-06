package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
)

func main() {
	cfg, err := parseFlags(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	logger := log.New(os.Stdout, "gtr: ", 0)
	var strategy Strategy
	if cfg.strategy == "coverage" {
		strategy = NewCoverStrategy(cfg.runInit, cfg.workDir, logger)
	} else {
		strategy = NewSSAStrategy(cfg.analysis, cfg.workDir, logger)
	}

	notifier := NewDesktopNotificator(true, 2000)
	testRunner := NewGoTestRunner(
		strategy,
		NewOsCommand,
		cfg.argsToTestBinary,
		logger,
	)
	tasks := []Task{testRunner, notifier}
	if cfg.autoCommit {
		autoCommitTask := NewTask("AutoCommit",
			CommitChanges(cfg.workDir, NewOsCommand),
			logger)
		tasks = append(tasks, autoCommitTask)
		tasks = append(tasks, notifier)
	}
	watcher, err := NewWatcher(
		cfg.workDir,
		tasks,
		cfg.delay,
		cfg.excludeFilePrefix,
		cfg.excludeDirs,
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

type config struct {
	workDir           string
	delay             int
	strategy          string
	analysis          string
	runInit           bool // run init in strategies
	excludeFilePrefix []string
	excludeDirs       []string
	autoCommit        bool
	argsToTestBinary  string
}

func flagUsage() string {
	return `
Usage of gtr:
  -C string
        directory to watch (default ".")
  -strategy string
        strategy analysis or coverage (default analysis)
  -analysis string
        source code analysis to use pointer, static, rta, cha (default pointer)
  -run-init bool
        runs init steps like on first run get coverage for all tests on coverage strategy (default true)
  -args string
    	args to the test binary
  -auto-commit bool
    	auto commit on tests pass (default false)
  -delay int
    	delay in Milliseconds (default 1000)
  -exclude-dirs string
    	prefixes to exclude sep by comma (default "vendor,node_modules")
  -exclude-file-prefix string
    	prefixes to exclude sep by comma (default "#")
`
}

func newConfig() config {
	return config{
		workDir:           ".",
		delay:             1000,
		strategy:          "analysis",
		runInit:           true,
		analysis:          "pointer",
		excludeFilePrefix: []string{"#"},
		excludeDirs:       []string{"vendor", "node_modules"},
		autoCommit:        false,
		argsToTestBinary:  "",
	}
}
