package main

import (
	"fmt"
	"go/ast"
	"testing"

	"github.com/kr/pretty"
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
		{
			&ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X: &ast.SelectorExpr{
						X: &ast.Ident{
							NamePos: 95,
							Name:    "pk",
						},
						Sel: &ast.Ident{
							NamePos: 98,
							Name:    "Type",
						},
					},
					Sel: &ast.Ident{
						NamePos: 103,
						Name:    "Min",
					},
				}},
			"pk.Type.Min", "",
		},
		{
			&ast.CallExpr{
				Fun: &ast.IndexExpr{
					X: &ast.Ident{
						NamePos: 307,
						Name:    "m",
						Obj: &ast.Object{
							Kind: 4,
							Name: "m",
							Data: int(0),
							Type: nil,
						},
					},
				},
			}, "", "unexpected value *ast.IndexExpr",
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

func Test_getTestedFuncs(t *testing.T) {
	var testFile = []byte(`
package pkga

import (
	"testing"
	"checktestrun/pkgc"
        pk "github.com/name/pkga"
        pkg1 "checktestrun/pkga"
)

func TestMin(t *testing.T) {
	res := pk.TA.Min(10, 20)
	if res != 10 {
		t.Errorf("expected 10, got %v", res)
	}
}

func TestMax(t *testing.T) {
	res := max(10, 20)
	if res != 20 {
		t.Errorf("expected 20, got %v", res)
	}
	var m map[int]func()

	m[0]()
}

func TestF1(t *testing.T) {
	F1()
	t.Error("update test pkga.T1")
}

func TestF2(t *testing.T) {
	pkg1.F2()
	t.Error("pkga.T2")
}

func TestDiff(t *testing.T) {
	out := pkgc.Diff(10, 1)
	if out != 9 {
		t.Errorf("expected %v, got %v", 9, out)
	}
}
`)

	// TODO add cases for Interfaces, Methods, Types
	cases := []struct {
		fileData       []byte
		testedEntities map[string][]Entity
		err            string
	}{
		{testFile, map[string][]Entity{
			"TestMax": []Entity{
				{typ: "func", name: "max"},
				{typ: "func", name: "t.Errorf"},
			},
			"TestF1": []Entity{
				{typ: "func", name: "F1"},
				{typ: "func", name: "t.Error"},
			},
			"TestF2": []Entity{
				{typ: "func", name: "pkg1.F2"},
				{typ: "func", name: "t.Error"},
			},
			"TestDiff": []Entity{
				{typ: "func", name: "pkgc.Diff"},
				{typ: "func", name: "t.Errorf"},
			},
			"TestMin": []Entity{
				{typ: "func", name: "pk.TA.Min"},
				{typ: "func", name: "t.Errorf"},
			},
		}, ""},
	}
	var errOut string
	for i, tc := range cases {
		errOut = ""
		entities, err := getTestedFuncs(tc.fileData)
		if err != nil {
			errOut = err.Error()
		}
		if errOut != tc.err {
			t.Errorf("case [%d]\nexpected error %#v\ngot %#v", i, tc.err, errOut)
			continue
		}
		diffs := pretty.Diff(tc.testedEntities, entities)
		if len(diffs) > 0 {
			fmt.Printf("%# v\n", pretty.Formatter(entities)) // output for debug

			t.Errorf("case [%d]\n%#v", i, diffs)
		}
	}
}

var gofile = []byte(`
package main

import "github.com/pkg"

type T1 struct{
     a int
}

func (t *T1) M1(a, b int) int {
	z := a + b
	a += b
	return z + a + b
}

func (t T1) M2(a, b int) int {
	return b - a
}

func F2() int {
	return pkg.F()
}

func Perimeter(d, h int) int {
	return 2*d + 2*h
}

func Area(d, h int) int {
	return d * h
}
`)

func TestGetFileBlocks(t *testing.T) {
	cases := []struct {
		fileName string
		fileData []byte
		output   FileInfo
		err      string
	}{
		{
			fileName: "gofile.go", fileData: gofile, output: FileInfo{
				fname:   "gofile.go",
				pkgName: "main",
				blocks: []FileBlock{
					{typ: "type", name: "T1", start: 6, end: 8},
					{typ: "method", name: "T1.M1", start: 10, end: 14},
					{typ: "method", name: "T1.M2", start: 16, end: 18},
					{typ: "func", name: "F2", start: 20, end: 22},
					{typ: "func", name: "Perimeter", start: 24, end: 26},
					{typ: "func", name: "Area", start: 28, end: 30},
				},
			},
		},
	}
	var errOut string
	for i, tc := range cases {
		errOut = ""
		fileInfo, err := getFileInfo("gofile.go", gofile)
		if err != nil {
			errOut = err.Error()
		}
		if errOut != tc.err {
			t.Errorf("case [%d]\nexpected error %#v\ngot %#v", i, tc.err, errOut)
			continue
		}

		diffs := pretty.Diff(tc.output, fileInfo)
		if len(diffs) > 0 {
			fmt.Printf("%# v\n", pretty.Formatter(fileInfo)) // output for debug
			t.Errorf("%# v", pretty.Formatter(diffs))
		}
	}
}
