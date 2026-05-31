package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Tool argument types

type affectedTestsArgs struct {
	Files    []string `json:"files"`
	GitRef   string   `json:"git_ref"`
	Analysis string   `json:"analysis"`
	WorkDir  string   `json:"work_dir"`
}

type runAffectedTestsArgs struct {
	Analysis string `json:"analysis"`
	WorkDir  string `json:"work_dir"`
	Args     string `json:"args"`
}

type fileImpactAnalysisArgs struct {
	WorkDir  string `json:"work_dir"`
	Analysis string `json:"analysis"`
}

// Tool result types

type affectedTestsResult struct {
	Tests    []string `json:"tests"`
	SubTests []string `json:"subtests"`
	RunAll   bool     `json:"run_all"`
}

type testRunResult struct {
	Tests    []string `json:"tests"`
	SubTests []string `json:"subtests"`
	Passed   bool     `json:"passed"`
	Output   string   `json:"output"`
}

type impactAnalysisResult struct {
	ChangedFiles    []string        `json:"changed_files"`
	AffectedTests   []string        `json:"affected_tests"`
	AffectedSubTests []string       `json:"affected_subtests"`
	ChangedBlocks   []changedBlock  `json:"changed_blocks"`
}

type changedBlock struct {
	File     string `json:"file"`
	Name     string `json:"name"`
	Type     string `json:"type"` // "func", "method", "type"
	StartLine int   `json:"start_line"`
	EndLine   int   `json:"end_line"`
}

func (s *MCPServer) toolAffectedTests(id json.RawMessage, argsJSON json.RawMessage) {
	var args affectedTestsArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		return
	}

	if args.WorkDir == "" {
		args.WorkDir = "."
	}
	if args.Analysis == "" {
		args.Analysis = "vta"
	}

	workDir, err := filepath.Abs(args.WorkDir)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid work_dir: %v", err))
		return
	}

	ctx := context.Background()

	// If git_ref is provided, create a temporary diff against that ref
	if args.GitRef != "" {
		// Stash current diff and use ref-based diff
		gitCmd := exec.CommandContext(ctx, "git", "-C", workDir, "diff", args.GitRef, "-U0", "--no-ext-diff", "--relative")
		var gitOut bytes.Buffer
		gitCmd.Stdout = &gitOut
		if err := gitCmd.Run(); err != nil {
			s.sendToolError(id, fmt.Sprintf("git diff against %s failed: %v", args.GitRef, err))
			return
		}
		// Parse changes from this diff
		changes, err := changesFromGitDiff(gitOut)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to parse diff: %v", err))
			return
		}
		if len(changes) == 0 {
			s.sendToolResult(id, affectedTestsResult{
				Tests:    []string{},
				SubTests: []string{},
			})
			return
		}
	}

	// Use existing strategy
	var strategy Strategy
	if args.Analysis == "coverage" {
		strategy = NewCoverStrategy(false, workDir, s.log)
	} else {
		strategy = NewSSAStrategy(args.Analysis, workDir, s.log)
	}

	runAll, tests, subTests, err := strategy.TestsToRun(ctx)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Analysis failed: %v", err))
		return
	}

	if tests == nil {
		tests = []string{}
	}
	if subTests == nil {
		subTests = []string{}
	}

	s.sendToolResult(id, affectedTestsResult{
		Tests:    tests,
		SubTests: subTests,
		RunAll:   runAll,
	})
}

func (s *MCPServer) toolRunAffectedTests(id json.RawMessage, argsJSON json.RawMessage) {
	var args runAffectedTestsArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		return
	}

	if args.WorkDir == "" {
		args.WorkDir = "."
	}
	if args.Analysis == "" {
		args.Analysis = "vta"
	}

	workDir, err := filepath.Abs(args.WorkDir)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid work_dir: %v", err))
		return
	}

	ctx := context.Background()

	var strategy Strategy
	if args.Analysis == "coverage" {
		strategy = NewCoverStrategy(false, workDir, s.log)
	} else {
		strategy = NewSSAStrategy(args.Analysis, workDir, s.log)
	}

	_, tests, subTests, err := strategy.TestsToRun(ctx)
	if err != nil {
		if err == ErrBuildFailed {
			s.sendToolResult(id, testRunResult{
				Passed: false,
				Output: "Build failed",
			})
			return
		}
		s.sendToolError(id, fmt.Sprintf("Analysis failed: %v", err))
		return
	}

	if len(tests) == 0 && len(subTests) == 0 {
		s.sendToolResult(id, testRunResult{
			Tests:  []string{},
			Passed: true,
			Output: "No affected tests found",
		})
		return
	}

	// Run the tests and capture output
	var testOutput bytes.Buffer
	testCmd := exec.CommandContext(ctx, "go", "test", "-v", "-vet", "off")
	if len(tests) > 0 {
		// Build -run pattern
		var testNames []string
		for _, t := range tests {
			idx := strings.LastIndexByte(t, '.')
			if idx >= 0 {
				testNames = append(testNames, t[idx+1:])
			}
		}
		testCmd.Args = append(testCmd.Args, "-run", strings.Join(testNames, "|"))
	}
	testCmd.Args = append(testCmd.Args, "./...")
	testCmd.Dir = workDir
	testCmd.Stdout = &testOutput
	testCmd.Stderr = &testOutput
	testCmd.Env = os.Environ()

	runErr := testCmd.Run()
	passed := runErr == nil

	if tests == nil {
		tests = []string{}
	}
	if subTests == nil {
		subTests = []string{}
	}

	s.sendToolResult(id, testRunResult{
		Tests:    tests,
		SubTests: subTests,
		Passed:   passed,
		Output:   testOutput.String(),
	})
}

func (s *MCPServer) toolFileImpactAnalysis(id json.RawMessage, argsJSON json.RawMessage) {
	var args fileImpactAnalysisArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid arguments: %v", err))
		return
	}

	if args.WorkDir == "" {
		args.WorkDir = "."
	}
	if args.Analysis == "" {
		args.Analysis = "vta"
	}

	workDir, err := filepath.Abs(args.WorkDir)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Invalid work_dir: %v", err))
		return
	}

	ctx := context.Background()
	gitCmd := NewGitCMD(workDir)

	changes, err := gitCmd.Diff(ctx)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("git diff failed: %v", err))
		return
	}

	// Filter to Go files
	var goChanges []Change
	var changedFiles []string
	seen := map[string]bool{}
	for _, c := range changes {
		if strings.HasSuffix(c.fpath, ".go") {
			goChanges = append(goChanges, c)
			if !seen[c.fpath] {
				changedFiles = append(changedFiles, c.fpath)
				seen[c.fpath] = true
			}
		}
	}

	if len(goChanges) == 0 {
		s.sendToolResult(id, impactAnalysisResult{
			ChangedFiles:  []string{},
			AffectedTests: []string{},
			ChangedBlocks: []changedBlock{},
		})
		return
	}

	// Parse file infos
	fileInfos := map[string]FileInfo{}
	for _, change := range goChanges {
		if _, ok := fileInfos[change.fpath]; ok {
			continue
		}
		info, err := getFileInfo(filepath.Join(workDir, change.fpath), nil)
		if err != nil {
			s.sendToolError(id, fmt.Sprintf("Failed to parse %s: %v", change.fpath, err))
			return
		}
		fileInfos[change.fpath] = info
	}

	changedBlocksMap, err := changesToFileBlocks(goChanges, fileInfos)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to analyze changes: %v", err))
		return
	}

	// Collect changed blocks
	var blocks []changedBlock
	for fname, info := range changedBlocksMap {
		for _, b := range info.blocks {
			blockType := "func"
			if b.typ&BlockMethod > 0 {
				blockType = "method"
			} else if b.typ&BlockType > 0 {
				blockType = "type"
			}
			blocks = append(blocks, changedBlock{
				File:      fname,
				Name:      b.name,
				Type:      blockType,
				StartLine: b.start,
				EndLine:   b.end,
			})
		}
	}

	// Get affected tests
	strategy := NewSSAStrategy(args.Analysis, workDir, s.log)
	_, tests, subTests, strategyErr := strategy.TestsToRun(ctx)
	if strategyErr != nil && strategyErr != ErrBuildFailed {
		s.sendToolError(id, fmt.Sprintf("Analysis failed: %v", strategyErr))
		return
	}

	if tests == nil {
		tests = []string{}
	}
	if subTests == nil {
		subTests = []string{}
	}

	s.sendToolResult(id, impactAnalysisResult{
		ChangedFiles:     changedFiles,
		AffectedTests:    tests,
		AffectedSubTests: subTests,
		ChangedBlocks:    blocks,
	})
}

// MCP tool response helpers

func (s *MCPServer) sendToolResult(id json.RawMessage, result any) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		s.sendToolError(id, fmt.Sprintf("Failed to marshal result: %v", err))
		return
	}

	s.sendResult(id, map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": string(resultJSON),
			},
		},
	})
}

func (s *MCPServer) sendToolError(id json.RawMessage, message string) {
	s.sendResult(id, map[string]any{
		"content": []map[string]any{
			{
				"type": "text",
				"text": message,
			},
		},
		"isError": true,
	})
}
