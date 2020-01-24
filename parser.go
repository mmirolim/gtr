package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/kr/pretty"
)

var reFnameUntrackedFiles = regexp.MustCompile(`\?\? (?P<fname>[a-zA-Z0-9_\-/]+\.go)`)

type Change struct {
	fpathOld string // a/
	fpath    string // b/ current
	start    int
	count    int
}

// TODO handle rename/copy/delete, /dev/null is used to signal created or deleted files.
func changesFromGitDiff(diff bytes.Buffer) ([]Change, error) {
	var changes []Change
	var serr error
	skipLine := func() {
		for {
			r, _, err := diff.ReadRune()
			if err != nil {
				serr = err
				return
			}
			if r == '\n' {
				break
			}
		}
	}
	var line []rune
	consumeLine := func() {
		line = line[:0]
		for {
			r, _, err := diff.ReadRune()
			if err != nil {
				serr = err
				return
			}
			if r == '\n' {
				break
			}
			line = append(line, r)
		}
	}
	readTokenInLineAt := func(i int) (string, int) {
		start := i
		for ; i < len(line); i++ {
			if line[i] == ' ' || line[i] == ',' {
				break
			}
		}
		return string(line[start:i]), i
	}
	readFileNames := func() (a string, b string) {
		var prev rune
		for i := 1; i < len(line); i++ {
			prev = line[i-1]
			if prev == 'a' && line[i] == '/' {
				a, i = readTokenInLineAt(i + 1)
			} else if prev == 'b' && line[i] == '/' {
				b, i = readTokenInLineAt(i + 1)
			}
		}
		return
	}
	readStartLineAndCount := func() (d1, d2 int) {
		var number string
		var err error
		for i := 0; i < len(line); i++ {
			if line[i] == '+' {
				// read start line
				number, i = readTokenInLineAt(i + 1)
				d1, err = strconv.Atoi(number)
				if err != nil {
					serr = err
					return
				}
				if line[i] == ',' {
					// read line count
					number, i = readTokenInLineAt(i + 1)
					d2, err = strconv.Atoi(number)
					if err != nil {
						serr = err
						return
					}
				}
			}
		}
		return
	}
	var r rune
	var f1, f2 string
	for {
		r, _, serr = diff.ReadRune()
		if serr != nil {
			if serr == io.EOF {
				break
			}
			return nil, serr
		}
		if r == '+' || r == '-' {
			skipLine()
			continue
		} else {
			diff.UnreadRune()
		}
		consumeLine()
		if line[0] == 'd' && line[1] == 'e' { //deleted
			// skip deleted files
			f1, f2 = "", ""
			continue
		}
		if line[0] == 'd' {
			f1, f2 = readFileNames()
		} else if line[0] == '@' && f2 != "" {
			d1, d2 := readStartLineAndCount()
			changes = append(changes, Change{f1, f2, d1, d2})
		}

	}
	if serr == io.EOF {
		serr = nil
	}

	return changes, serr
}

// TODO comments
// TODO maybe use existing git-diff parsers for unified format
// TODO do not use global states
func GetDiff(workdir string) ([]Change, error) {
	// TODO store hashes of new files and return untracked new files to run
	var gitOut bytes.Buffer
	var results []Change
	// get not yet commited go files
	gitCmd := exec.Command("git", "-C", workdir, "status", "--short")
	gitCmd.Stdout = &gitOut
	err := gitCmd.Run()
	if err != nil {
		return nil, err
	}
	matches := reFnameUntrackedFiles.FindAllString(gitOut.String(), -1)
	for i := range matches {
		fname := reFnameUntrackedFiles.ReplaceAllString(matches[i], "${fname}")
		results = append(results, Change{fname, fname, 0, 0})
	}
	gitOut.Reset()
	// get changes in go files
	// Disallow external diff drivers.
	gitCmd = exec.Command("git", "-C", workdir, "diff", "--no-ext-diff")
	gitCmd.Stdout = &gitOut
	err = gitCmd.Run()
	if err != nil {
		return nil, err
	}
	changes, err := changesFromGitDiff(gitOut)
	if err != nil {
		return nil, err
	}
	results = append(results, changes...)
	return results, nil
}

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

// TODO handle other types of entities like Types, Interfaces
// parse go test file and returns entities by Test name
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
						typ:  "func", // TODO handle method
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
