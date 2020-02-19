package main

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
)

var _ Strategy = (*dummyStrategy)(nil)

type dummyStrategy struct {
	runAll, coverageEnabled bool
	tests, subtests         []string
	err                     error
}

func (ds *dummyStrategy) CoverageEnabled() bool {
	return ds.coverageEnabled
}

func (ds *dummyStrategy) TestsToRun(ctx context.Context) (
	runAll bool, tests, subTests []string, err error,
) {
	return ds.runAll, ds.tests, ds.subtests, ds.err
}

func TestGoTestRunnerRun(t *testing.T) {
	cases := []struct {
		desc               string
		strategyErr        error
		cmdSuccess, runAll bool
		coverageEnabled    bool
		tests              []string
		subTests           []string
		output             string
		err                error
	}{
		{
			desc:       "No file changes",
			cmdSuccess: true,
			output:     "No test found to run",
			err:        nil,
		},
		{
			desc:            "2 top level tests pass",
			cmdSuccess:      true,
			runAll:          true,
			coverageEnabled: true,
			tests:           []string{"module/pkga.TestZ", "module.TestC"},
			output:          "Tests PASS: TestC$|TestZ$",
		},
		{
			desc:       "1 top level test and 2 subtests pass",
			cmdSuccess: true,
			tests:      []string{"module.TestZ"},
			subTests:   []string{"b 1", "z 2"},
			output:     "Tests PASS: TestZ$/(b_1|z_2)",
		},
		{
			desc:        "Strategy error",
			strategyErr: errors.New("injected error"),
			cmdSuccess:  true,
			tests:       []string{"module/pkga/pkgb.TestZ"},
			subTests:    []string{"b1"},
			err:         errors.New("strategy error injected error"),
		},
		{
			desc:       "Tests failed",
			cmdSuccess: false,
			tests:      []string{"module.TestZ"},
			subTests:   []string{"b1"},
			output:     "Tests FAIL: TestZ$/(b1)",
		},
		{
			desc:            "Subtests on runAll false",
			cmdSuccess:      true,
			runAll:          false,
			coverageEnabled: true,
			tests:           []string{"module.TestZ"},
			subTests:        []string{"group"},
			output:          "Tests PASS: TestZ$/(group)",
		},
	}
	logger := log.New(os.Stdout, "TestGoTestRunnerRun:", log.Ltime)
	var ds dummyStrategy
	for i, tc := range cases {
		ds.err = tc.strategyErr
		ds.tests = tc.tests
		ds.subtests = tc.subTests
		ds.runAll = tc.runAll
		ds.coverageEnabled = tc.coverageEnabled
		mockCmd := NewMockCommand(nil, tc.cmdSuccess)
		runner := NewGoTestRunner(&ds, mockCmd.New, "", logger)
		out, err := runner.Run(context.TODO())

		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}
		if tc.output != out {
			t.Errorf("case [%d] %s\nexpected \"%s\", got \"%s\"", i, tc.desc, tc.output, out)
		}

	}

}

func TestGoTestRunnerJoinTestAndSubtest(t *testing.T) {
	runner := &GoTestRunner{}
	cases := []struct {
		tests    []string
		subTests []string
		output   string
	}{
		{nil, nil, ""},
		{[]string{"TestZ", "TestC"}, nil, "TestC$|TestZ$"},
		{nil, []string{"b 1", "z 2"}, "/(b_1|z_2)"},
		{[]string{"TestZ", "TestC", "TestB"}, []string{"b 1", "z2"}, "TestB$|TestC$|TestZ$/(b_1|z2)"},
	}
	for i, tc := range cases {
		out := runner.joinTestAndSubtest(tc.tests, tc.subTests)
		if tc.output != out {
			t.Errorf("case [%d]expected %s, got %s", i, tc.output, out)
		}

	}

}
