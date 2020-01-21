package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/kr/pretty"
)

type Entity struct {
	typ  string
	name string
}

/*
Scope: &ast.Scope{
        Outer:   (*ast.Scope)(nil),
        Objects: {
            "TestMin": &ast.Object{
                Kind: 5,
                Name: "TestMin",
                Decl: &ast.FuncDecl{(CYCLIC REFERENCE)},
                Data: nil,
                Type: nil,
            },
            "TestMax": &ast.Object{
                Kind: 5,
                Name: "TestMax",
                Decl: &ast.FuncDecl{(CYCLIC REFERENCE)},
                Data: nil,
                Type: nil,
            },
        },
    },
    Imports: {
        &ast.ImportSpec{
            Doc:     (*ast.CommentGroup)(nil),
            Name:    (*ast.Ident)(nil),
            Path:    &ast.BasicLit{ValuePos:29, Kind:9, Value:"\"github.com/name/pkga\""},
            Comment: (*ast.CommentGroup)(nil),
            EndPos:  0,
        },
    },
*/

// TODO handle rename imports
// TODO handle methods
// TODO handle variable resolution
func fnNameFromCallExpr(fn *ast.CallExpr) (string, error) {
	var fname string
	var err error
	var combineName func(*ast.SelectorExpr) string

	combineName = func(expr *ast.SelectorExpr) string {
		switch v := expr.X.(type) {
		case *ast.Ident:
			// base case
			return v.Name + "." + expr.Sel.Name
		case *ast.SelectorExpr:
			return combineName(v) + "." + expr.Sel.Name
		default:
			err = fmt.Errorf("unexpected value %T", v)
			return ""
		}
	}

	switch v := fn.Fun.(type) {
	case *ast.Ident:
		// base case
		fname = v.Name
	case *ast.SelectorExpr:
		fname = combineName(v)
	default:
		err = fmt.Errorf("unexpected value %T", v)
	}

	return fname, err
}

var tmu sync.Mutex
var testFiles = map[string]map[string][]Entity{}

func parseTestFile(fname string) error {
	fmt.Printf("parseTestFile %+v\n", fname) // output for debug
	if !strings.HasSuffix(fname, "_test.go") {
		return fmt.Errorf("not a test file %s", fname)
	}
	data, err := ioutil.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("read file error %v", err)
	}
	dic, err := getTestedFuncs(data)
	if err != nil {
		return fmt.Errorf("getTestedFuncs error %v", err)
	}
	tmu.Lock()
	testFiles[fname] = dic
	fmt.Printf("TestFilesParse\n%# v\n", pretty.Formatter(testFiles)) // output for debug
	tmu.Unlock()
	indexFuncsInTestFile(fname)
	return nil
}

var fmu sync.Mutex

// map funcs to [test files] -> [Tests]
var funcToTests = map[string]map[string]map[string]bool{}

// map funcs to test
func indexFuncsInTestFile(fname string) {
	tmu.Lock()
	defer tmu.Unlock()
	fmu.Lock()
	defer fmu.Unlock()
	// TODO incremental
	dic := testFiles[fname]
	index := map[string]map[string]bool{}
	for testName, entities := range dic {
		for _, entity := range entities {
			if entity.typ == "func" {
				if _, ok := index[entity.name]; ok {
					index[entity.name][testName] = true
				} else {
					index[entity.name] = map[string]bool{
						testName: true,
					}
				}
			}
		}
	}
	// replace index for test file for funcs
	for funName := range index {
		funcToTests[funName][fname] = index[funName]
	}
	fmt.Printf("indexFuncsInTestFile\n%# v\n", pretty.Formatter(funcToTests)) // output for debug
}

func getTestedFuncs(testFile []byte) (map[string][]Entity, error) {
	dic := make(map[string][]Entity)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", testFile, parser.ParseComments)
	if err != nil {
		return dic, nil
	}
	// fmt.Printf("%# v\n", pretty.Formatter(f))
	packageName := f.Name
	fmt.Printf("PACKAGE_NAME: %+v\n", packageName) // output for debug
	for k, v := range f.Scope.Objects {
		funcDecl, ok := v.Decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(k, "Test") {
			continue
		}
		var entities []Entity
		posStart := fset.Position(funcDecl.Body.Lbrace)
		posEnd := fset.Position(funcDecl.Body.Rbrace)
		fmt.Printf("Fn Name %v Body start %d end %d\n", k, posStart.Line, posEnd.Line) // output for debug

		// inspect only Test funcs
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if ok && callExpr.Fun != nil {
				// current node is a function!
				// func called
				fname, err := fnNameFromCallExpr(callExpr)
				if err == nil {
					entities = append(entities, Entity{
						typ:  "func",
						name: fname,
					})
					fmt.Printf("CALL expr found %+v\n", fname) // output for debug
				}
			}
			return true
		})
		dic[k] = entities
	}

	return dic, nil
}
