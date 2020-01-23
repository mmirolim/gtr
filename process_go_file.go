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
	//fmt.Printf("TestFilesParse\n%# v\n", pretty.Formatter(testFiles)) // output for debug
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
		_, ok := funcToTests[funName]
		if ok {
			funcToTests[funName][fname] = index[funName]
		} else {
			funcToTests[funName] = map[string]map[string]bool{
				fname: index[funName],
			}
		}
	}
	//fmt.Printf("indexFuncsInTestFile\n%# v\n", pretty.Formatter(funcToTests)) // output for debug
}

func getTestedFuncs(testFile []byte) (map[string][]Entity, error) {
	dic := make(map[string][]Entity)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", testFile, parser.ParseComments)
	if err != nil {
		return dic, err
	}

	// TODO need to parse each FuncDecl and Typespec
	// not everything in scope
	for k, v := range f.Scope.Objects {
		funcDecl, ok := v.Decl.(*ast.FuncDecl)
		if !ok || !strings.HasPrefix(k, "Test") {
			continue
		}
		var entities []Entity

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
				}
			}
			return true
		})
		dic[k] = entities
	}

	return dic, nil
}

// returns map FileInfo by file name with effected blocks
func processFileChanges() (map[string]FileInfo, error) {
	changedBlocks := map[string]FileInfo{}
	// get changes
	changes, err := GetDiff(".")
	if err != nil {
		return nil, err
	}
	//fmt.Printf("Process changes\n%+v\n", changes) // output for debug
	fileInfos := make(map[string]FileInfo)
	// process all changes
	for _, change := range changes {
		info, ok := fileInfos[change.fpath]
		if !ok {
			data, err := ioutil.ReadFile(change.fpath)
			if err != nil {
				fmt.Printf("ReadFile %s error %+v\n", change.fpath, err) // output for debug

				return nil, err
			}
			info, err = getFileInfo(change.fpath, data)
			if err != nil {
				fmt.Printf("getFileInfo %s error %+v\n", change.fpath, err) // output for debug
				return nil, err
			}
			fileInfos[change.fpath] = info
		}
		//fmt.Printf("fpath %s %# v\n", change.fpath, pretty.Formatter(info)) // output for debug
		changeInfo, ok := changedBlocks[change.fpath]
		if !ok {
			changeInfo.fname = info.fname
			changeInfo.pkgName = info.pkgName
		}

		// expect blocks sorted by start line
		for _, block := range info.blocks {
			start := change.start
			end := change.count + change.start
			if start == 0 && end == 0 {
				// new untracked file
				changeInfo.blocks = append(changeInfo.blocks, block)
				continue
			}
			if end < block.start {
				break
			}
			if (start >= block.start && start <= block.end) ||
				(end >= block.start && end <= block.end) ||
				(block.start >= start && block.end <= end) {
				changeInfo.blocks = append(changeInfo.blocks, block)
			}
		}
		if len(changeInfo.blocks) > 0 {
			changedBlocks[change.fpath] = changeInfo
		}

	}
	return changedBlocks, nil
}

type FileInfo struct {
	fname, pkgName string
	blocks         []FileBlock
}
type FileBlock struct {
	typ, name  string
	start, end int // lines [start, end] from 1
}

// getFileInfo returns FileInfo struct with
// with entities divided in blocks according to line position
// blocks sorted by start line
// TODO add Type Decl blocks
func getFileInfo(fname string, file []byte) (FileInfo, error) {
	var fileInfo FileInfo
	var blocks []FileBlock
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", file, parser.ParseComments)
	if err != nil {
		return fileInfo, err
	}
	fileInfo.pkgName = f.Name.Name
	fileInfo.fname = fname
	for _, decl := range f.Decls {
		var block FileBlock
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, v := range d.Specs {
				switch spec := v.(type) {
				case *ast.ImportSpec:
					// TODO handle
				case *ast.TypeSpec:
					block.name = spec.Name.Name
					block.typ = "type"
					// handle struct type
					typ, ok := spec.Type.(*ast.StructType)
					if ok {
						block.start = fset.Position(typ.Fields.Opening).Line
						block.end = fset.Position(typ.Fields.Closing).Line
						blocks = append(blocks, block)
					}
				case *ast.ValueSpec:
					// TODO handle variable decl
				default:
					fmt.Printf("[WARN] unhandled GenDecl Spec case %# v\n", pretty.Formatter(spec)) // output for debug

				}
			}
			continue
		case *ast.FuncDecl:
			fn := d
			block.typ = "func"
			block.name = fn.Name.Name
			block.start = fset.Position(fn.Body.Lbrace).Line
			block.end = fset.Position(fn.Body.Rbrace).Line
			if fn.Recv != nil {
				// method
				block.typ = "method"
				fld := fn.Recv.List[0]
				switch v := fld.Type.(type) {
				case *ast.Ident:
					block.name = v.Name + "." + block.name
				case *ast.StarExpr:
					ident, ok := v.X.(*ast.Ident)
					if ok {
						block.name = ident.Name + "." + block.name
					} else {
						return fileInfo, fmt.Errorf("unexpected ast type %T", v.X)
					}
				default:
					fmt.Printf("[WARN] unhandled method reciver case %T\n", v) // output for debug

				}
			}
			blocks = append(blocks, block)
		default:
			continue
		}

	}
	fileInfo.blocks = blocks
	return fileInfo, nil
}

// TODO handle Type struct changes
func findTestsToRun(effectedBlocks map[string]FileInfo) ([]string, error) {
	testsSet := map[string]struct{}{}

	for fname, info := range effectedBlocks {
		testFile := false
		// test file
		// TODO FileInfo match as test file or not
		if strings.HasSuffix(fname, "_test.go") {
			testFile = true
		}

		for _, block := range info.blocks {
			if testFile && block.typ == "func" && strings.HasPrefix(block.name, "Test") {
				testsSet[block.name] = struct{}{}
				continue
			}
			fmu.Lock()
			testNames, ok := funcToTests[block.name]
			if !ok {
				fmt.Printf("[WARN] No test found for %s %+v\n", info.fname, block) // output for debug

			} else {
				for _, testNames := range testNames {
					for k := range testNames {
						testsSet[k] = struct{}{}
					}
				}
			}
			fmu.Unlock()

		}

	}

	tests := make([]string, 0, len(testsSet))
	for k := range testsSet {
		tests = append(tests, k)
	}
	return tests, nil
}
