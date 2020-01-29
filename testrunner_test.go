package main

import (
	"context"
	"errors"
	"testing"
)

var _ Strategy = (*dummyStrategy)(nil)

type dummyStrategy struct {
	tests, subtests []string
	err             error
}

func (ds *dummyStrategy) TestsToRun(ctx context.Context) (
	tests []string, subTests []string, err error,
) {
	return ds.tests, ds.subtests, ds.err
}

func TestGoTestRunnerRun(t *testing.T) {
	cases := []struct {
		desc        string
		strategyErr error
		cmdSuccess  bool
		tests       []string
		subTests    []string
		output      string
		err         error
	}{
		{desc: "No file changes", cmdSuccess: true, output: "no test found to run"},
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
			subTests:   []string{"b1", "z2"},
			output:     "Tests PASS: TestZ$/(b1|z2)",
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
	}

	var ds dummyStrategy
	for i, tc := range cases {
		ds.err = tc.strategyErr
		ds.tests = tc.tests
		ds.subtests = tc.subTests
		mockCmd := NewMockCommand(nil, tc.cmdSuccess)

		runner := NewGoTestRunner(&ds, mockCmd.New, "", false)
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
		{nil, []string{"b1", "z2"}, "/(b1|z2)"},
		{[]string{"TestZ", "TestC", "TestB"}, []string{"b1", "z2"}, "TestZ$|TestC$|TestB$/(b1|z2)"},
	}
	for i, tc := range cases {
		out := runner.joinTestAndSubtest(tc.tests, tc.subTests)
		if tc.output != out {
			t.Errorf("case [%d]expected %s, got %s", i, tc.output, out)
		}

	}

}
