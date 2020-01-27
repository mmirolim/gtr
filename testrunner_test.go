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

func (ds *dummyStrategy) TestsToRun() (tests []string, subTests []string, err error) {
	return ds.tests, ds.subtests, ds.err
}

func TestGoTestRunnerRun(t *testing.T) {
	cases := []struct {
		strategyErr error
		cmdSuccess  bool
		tests       []string
		subTests    []string
		output      string
		err         string
	}{
		{nil, true, nil, nil, "no test found to run", ""},
		{nil, true, []string{"TestZ$", "TestC"}, nil, "Tests PASS: TestZ$|TestC", ""},
		{nil, true, []string{"TestZ$"}, []string{"b1", "z2"}, "Tests PASS: TestZ$/(b1|z2)", ""},
		{
			errors.New("injected error"), true,
			[]string{"TestZ$"}, []string{"b1"}, "",
			"strategy error injected error",
		},
		{
			nil, false,
			[]string{"TestZ$"}, []string{"b1"},
			"Tests FAIL: TestZ$/(b1)",
			"",
		},
		{ // define only sub tests group
			nil, true,
			nil, []string{"group"},
			"Tests PASS: /(group)",
			"",
		},
	}

	var errOut string
	var ds dummyStrategy
	for i, tc := range cases {
		errOut = ""
		ds.err = tc.strategyErr
		ds.tests = tc.tests
		ds.subtests = tc.subTests
		mockCmd := NewMockCommand(nil, tc.cmdSuccess)

		runner := NewGoTestRunner(&ds, mockCmd.New, "", false)
		out, err := runner.Run(context.TODO())
		if err != nil {
			errOut = err.Error()
		}
		if errOut != tc.err {
			t.Errorf("case [%d]\nexpected error \"%s\"\ngot \"%s\"", i, tc.err, errOut)
			continue
		}
		if tc.output != out {
			t.Errorf("case [%d]\nexpected \"%s\", got \"%s\"", i, tc.output, out)
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
		{[]string{"TestZ$", "TestC"}, nil, "TestZ$|TestC"},
		{nil, []string{"b1", "z2"}, "/(b1|z2)"},
		{[]string{"TestZ$", "TestC", "TestB$"}, []string{"b1", "z2"}, "TestZ$|TestC|TestB$/(b1|z2)"},
	}
	for i, tc := range cases {
		out := runner.joinTestAndSubtest(tc.tests, tc.subTests)
		if tc.output != out {
			t.Errorf("case [%d]expected %s, got %s", i, tc.output, out)
		}

	}

}
