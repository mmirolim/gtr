package main

import (
	"go/ast"
	"testing"
)

func Test_fnNameFromCallExpr(t *testing.T) {
	cases := []struct {
		callExpr *ast.CallExpr
		output   string
		err      string
	}{
		{
			&ast.CallExpr{
				Fun: &ast.Ident{
					Name: "F1",
				},
			}, "F1", ""},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "t",
					},
					Sel: &ast.Ident{
						Name: "Error",
					},
				},
			}, "t.Error", ""},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "pkg1",
					},
					Sel: &ast.Ident{
						Name: "F2",
					},
				},
			}, "pkg1.F2", "",
		},
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.Ident{
						Name: "pkgc",
					},
					Sel: &ast.Ident{
						Name: "Diff",
					},
				},
			}, "pkgc.Diff", "",
		},
	}
	var errOut string
	for i, tc := range cases {
		errOut = ""
		fnName, err := fnNameFromCallExpr(tc.callExpr)
		if err != nil {
			errOut = err.Error()
		}
		if errOut != tc.err {
			t.Errorf("case [%d]\nexpected error %#v\ngot %#v", i, tc.err, errOut)
			continue
		}
		if fnName != tc.output {
			t.Errorf("case [%d]\nexpected %#v\ngot %#v", i, tc.output, fnName)
		}

	}
}
