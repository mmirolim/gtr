package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
)

func main() {
	// subcommand dispatch
	subcmd := "watch" // default
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "watch", "mcp", "run", "analyze":
			subcmd = os.Args[1]
			// remove subcommand from args so flag parsing works
			os.Args = append(os.Args[:1], os.Args[2:]...)
		case "help", "-help", "--help":
			fmt.Println(subcommandUsage())
			os.Exit(0)
		}
	}

	switch subcmd {
	case "watch":
		runWatch()
	case "mcp":
		runMCP()
	case "run":
		runOneShot(false)
	case "analyze":
		runOneShot(true)
	}
}

func runWatch() {
	cfg, err := parseFlags(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
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

func runMCP() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	server := NewMCPServer(logger)
	if err := server.Run(); err != nil {
		logger.Error("MCP server error", "err", err)
		os.Exit(1)
	}
}

func runOneShot(analyzeOnly bool) {
	cfg, err := parseFlags(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	exitCode := executeOneShot(cfg, logger, analyzeOnly)
	os.Exit(exitCode)
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
	baseRef           string // for PR-scoped analysis
}

func subcommandUsage() string {
	return `GTR - Go Test Runner

Usage: gtr [command] [flags]

Commands:
  watch      Watch for file changes and run affected tests (default)
  mcp        Start MCP JSON-RPC 2.0 server over stdio
  run        One-shot: analyze changes and run affected tests
  analyze    One-shot: analyze changes and print affected tests as JSON

Flags:
` + flagUsage()
}

func flagUsage() string {
	return `  -C string
        directory to watch (default ".")
  -strategy string
        strategy analysis or coverage (default analysis)
  -analysis string
        source code analysis to use vta, static, rta, cha (default vta)
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
  -base-ref string
    	git ref to diff against for analysis (e.g. main, HEAD~1)
`
}

func newConfig() config {
	return config{
		workDir:           ".",
		delay:             1000,
		strategy:          "analysis",
		runInit:           true,
		analysis:          "vta",
		excludeFilePrefix: []string{"#"},
		excludeDirs:       []string{"vendor", "node_modules"},
		autoCommit:        false,
		argsToTestBinary:  "",
	}
}
