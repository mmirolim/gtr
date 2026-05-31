package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
)

// executeOneShot runs analysis and optionally tests, returning exit code
func executeOneShot(cfg config, logger *slog.Logger, analyzeOnly bool) int {
	ctx := context.Background()

	var strategy Strategy
	if cfg.strategy == "coverage" {
		strategy = NewCoverStrategy(cfg.runInit, cfg.workDir, logger)
	} else {
		strategy = NewSSAStrategy(cfg.analysis, cfg.workDir, logger)
	}

	runAll, tests, subTests, err := strategy.TestsToRun(ctx)
	if err != nil {
		if err == ErrBuildFailed {
			logger.Error("build failed")
			return 1
		}
		logger.Error("analysis failed", "err", err)
		return 1
	}

	if analyzeOnly {
		// Output as JSON
		result := struct {
			Tests    []string `json:"tests"`
			SubTests []string `json:"subtests"`
			RunAll   bool     `json:"run_all"`
		}{
			Tests:    tests,
			SubTests: subTests,
			RunAll:   runAll,
		}
		if result.Tests == nil {
			result.Tests = []string{}
		}
		if result.SubTests == nil {
			result.SubTests = []string{}
		}
		sort.Strings(result.Tests)
		sort.Strings(result.SubTests)

		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			logger.Error("failed to encode JSON", "err", err)
			return 1
		}
		return 0
	}

	// Run mode: analyze + run tests + exit
	if len(tests) == 0 && len(subTests) == 0 {
		fmt.Println("No affected tests found")
		return 0
	}

	testRunner := NewGoTestRunner(
		strategy,
		NewOsCommand,
		cfg.argsToTestBinary,
		logger,
	)
	msg, err := testRunner.Run(ctx, nil)
	if err != nil {
		logger.Error("test runner failed", "err", err)
		return 1
	}

	fmt.Println(msg)
	if len(msg) > 0 && msg[:10] == "Tests FAIL" {
		return 1
	}
	return 0
}
