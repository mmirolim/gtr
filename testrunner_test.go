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
	pkgPaths, tests, subtests []string
	err                       error
}

func (ds *dummyStrategy) TestsToRun(ctx context.Context) (
	pkgPaths, tests, subTests []string, err error,
) {
	return ds.pkgPaths, ds.tests, ds.subtests, ds.err
}

func TestGoTestRunnerRun(t *testing.T) {
	cases := []struct {
		desc        string
		strategyErr error
		cmdSuccess  bool
		pkgPaths    []string
		tests       []string
		subTests    []string
		output      string
		err         error
	}{
		{
			desc:       "No file changes",
			cmdSuccess: true,
			output:     "No test found to run",
			err:        nil,
		},
		{
			desc:       "2 top level tests pass",
			cmdSuccess: true,
			tests:      []string{"TestZ", "TestC"},
			output:     "Tests PASS: TestZ$|TestC$",
		},
		{
			desc:       "1 top level test and 2 subtests pass",
			cmdSuccess: true,
			tests:      []string{"TestZ"},
			subTests:   []string{"b 1", "z 2"},
			output:     "Tests PASS: TestZ$/(b_1|z_2)",
		},
		{
			desc:        "Strategy error",
			strategyErr: errors.New("injected error"),
			cmdSuccess:  true,
			tests:       []string{"TestZ"},
			subTests:    []string{"b1"},
			err:         errors.New("strategy error injected error"),
		},
		{
			desc:       "Tests failed",
			cmdSuccess: false,
			tests:      []string{"TestZ"},
			subTests:   []string{"b1"},
			output:     "Tests FAIL: TestZ$/(b1)",
		},
		{
			desc:       "Subtests pass",
			cmdSuccess: true,
			subTests:   []string{"group"},
			output:     "Tests PASS: /(group)",
		},
		{
			desc:       "Test Pass package module-name/pkgA",
			cmdSuccess: true,
			pkgPaths:   []string{"module-name/pkgA"},
			tests:      []string{"TestZ"},
			subTests:   []string{"b1"},
			output:     "Tests PASS: TestZ$/(b1)",
		},
	}
	logger := log.New(os.Stdout, "TestGoTestRunnerRun:", log.Ltime)
	var ds dummyStrategy
	for i, tc := range cases {
		ds.err = tc.strategyErr
		ds.pkgPaths = tc.pkgPaths
		ds.tests = tc.tests
		ds.subtests = tc.subTests
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
		{[]string{"TestZ", "TestC"}, nil, "TestZ$|TestC$"},
		{nil, []string{"b 1", "z 2"}, "/(b_1|z_2)"},
		{[]string{"TestZ", "TestC", "TestB"}, []string{"b 1", "z2"}, "TestZ$|TestC$|TestB$/(b_1|z2)"},
	}
	for i, tc := range cases {
		out := runner.joinTestAndSubtest(tc.tests, tc.subTests)
		if tc.output != out {
			t.Errorf("case [%d]expected %s, got %s", i, tc.output, out)
		}

	}

}
