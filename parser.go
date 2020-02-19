package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kr/pretty"
)

// Change of file lines
type Change struct {
	fpathOld string // a/
	fpath    string // b/ current
	start    int
	count    int
}

// changesFromGitDiff parses git diff output and returns
// slice of Changes
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
		if line[0] == 'd' && line[1] == 'e' { // deleted
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

// FileInfo file metadata
// with file name, package it depends
// number of lines and FileBlocks
type FileInfo struct {
	fname, pkgName string
	endLine        int
	blocks         []FileBlock
}

// FileBlock defines blocks of entities
// in a file, func/method [start, end] line
type FileBlock struct {
	typ        BlockKind
	name       string
	start, end int // lines [start, end] from 1
}

// BlockKind custom type for blocks types
type BlockKind uint32

const (
	// BlockType Type definition
	BlockType BlockKind = 1 << iota
	// BlockFunc Func def
	BlockFunc
	// BlockMethod Method def
	BlockMethod
)

// getFileInfo returns FileInfo struct
// with entities divided in blocks according to line position
// blocks sorted by start line
// if src is nil, reads from fname
func getFileInfo(fname string, src interface{}) (FileInfo, error) {
	var fileInfo FileInfo
	var blocks []FileBlock

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fname, src, parser.ParseComments)
	if err != nil {
		return fileInfo, err
	}
	fileInfo.pkgName = f.Name.Name
	_, fname = filepath.Split(fname)
	fileInfo.fname = fname
	parseSpecs := func(d *ast.GenDecl) []FileBlock {
		var blocks []FileBlock
		var block FileBlock
		for _, v := range d.Specs {
			switch spec := v.(type) {
			case *ast.ImportSpec:
				// TODO handle
			case *ast.TypeSpec:
				block.name = spec.Name.Name
				block.typ = BlockType
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
		return blocks
	}
	parseFuncDecl := func(fn *ast.FuncDecl) (FileBlock, error) {
		var block FileBlock
		if fn == nil {
			return block, errors.New("parseFuncDecl FuncDecl is nil")
		}
		block.typ = BlockFunc
		block.name = fn.Name.Name
		block.start = fset.Position(fn.Body.Lbrace).Line
		block.end = fset.Position(fn.Body.Rbrace).Line
		if fn.Recv != nil {
			// method
			block.typ = BlockMethod
			fld := fn.Recv.List[0]
			switch v := fld.Type.(type) {
			case *ast.Ident:
				block.name = v.Name + "." + block.name
			case *ast.StarExpr:
				ident, ok := v.X.(*ast.Ident)
				if ok {
					block.name = ident.Name + "." + block.name
				} else {
					return block, fmt.Errorf("unexpected ast type %T", v.X)
				}
			default:
				fmt.Printf("[WARN] unhandled method reciver case %T\n", v) // output for debug

			}
		}
		return block, nil
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			blocks = append(blocks, parseSpecs(d)...)
		case *ast.FuncDecl:
			block, err := parseFuncDecl(d)
			if err != nil {
				return fileInfo, err
			}
			blocks = append(blocks, block)
		default:
			continue
		}

	}

	fileInfo.endLine = fset.Position(f.End()).Line
	fileInfo.blocks = blocks
	return fileInfo, nil
}

// splitStr splits string by sep and trims each entry
func splitStr(str, sep string) []string {
	out := strings.Split(str, sep)
	for i := range out {
		out[i] = strings.Trim(out[i], " ")
	}
	return out
}

// mapStrToSlice returns slice of keys from map
func mapStrToSlice(set map[string]bool) []string {
	var out []string
	for k := range set {
		out = append(out, k)
	}
	return out
}

// parseFlags parses provided args and returns config
func parseFlags(args []string) (config, error) {
	var err error
	args = args[1:]
	if len(args) > 0 && (args[0] == "help" || args[0] == "-help") {
		return config{}, errors.New(flagUsage())
	}
	cfg := newConfig()
	var flagName, nextArg string
LOOP:
	for i := 0; i < len(args); i++ {
		eqID := strings.IndexByte(args[i], '=')
		if eqID >= 0 {
			flagName, nextArg = args[i][:eqID], args[i][eqID+1:]
		} else {
			if i+1 >= len(args) {
				return config{}, fmt.Errorf("%s value missing", args[i])
			}
			flagName, nextArg = args[i], args[i+1]
			i++
		}
		nextArg = strings.Trim(nextArg, "\"")
		switch flagName {
		case "-C":
			cfg.workDir = nextArg
		case "-strategy":
			if isValidStrategy(nextArg) {
				cfg.strategy = nextArg
			} else {
				return config{}, fmt.Errorf("-strategy invalid value %v", nextArg)
			}
		case "-analysis":
			if isValidAnalysis(nextArg) {
				cfg.analysis = nextArg
			} else {
				return config{}, fmt.Errorf("-analysis invalid value %v", nextArg)
			}
		case "-run-init":
			cfg.runInit, err = strconv.ParseBool(nextArg)
			if err != nil {
				return config{}, fmt.Errorf("-run-init invalid value %v", nextArg)
			}
		case "-delay":
			cfg.delay, err = strconv.Atoi(nextArg)
			if err != nil {
				return config{}, fmt.Errorf("-delay invalid value %v", nextArg)
			}
		case "-exclude-file-prefix":
			cfg.excludeFilePrefix = splitStr(nextArg, ",")
		case "-exclude-dirs":
			cfg.excludeDirs = splitStr(nextArg, ",")
		case "-auto-commit":
			cfg.autoCommit, err = strconv.ParseBool(nextArg)
			if err != nil {
				return config{}, fmt.Errorf("-auto-commit invalid value %v", nextArg)
			}
		case "-args":
			cfg.argsToTestBinary = strings.Join(args[i:], " ")
			break LOOP
		default:
			return cfg, fmt.Errorf("invalid option -- %s", flagName)
		}
	}

	return cfg, nil
}

func isValidAnalysis(analysis string) bool {
	if analysis == "pointer" ||
		analysis == "cha" ||
		analysis == "rta" ||
		analysis == "static" {
		return true
	}
	return false
}

func isValidStrategy(strategy string) bool {
	if strategy == "coverage" || strategy == "analysis" {
		return true
	}
	return false
}

// getModuleName returns module name
// in gived workDir
func getModuleName(workDir string) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join(workDir, "go.mod"))
	if err != nil {
		// get from GOPATH
		gopath := os.Getenv("GOPATH")
		if gopath == "" {
			return "", errors.New("GOPATH and go.mod not found")
		}
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Rel(filepath.Join(gopath, "src"), dir)
	}

	var line []byte
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			break
		}
		line = data[0 : i+1]
	}

	return strings.Split(string(line), " ")[1], nil
}

var CoverProflineRe = regexp.MustCompile(`^(.+):([0-9]+).([0-9]+),([0-9]+).([0-9]+) ([0-9]+) ([0-9]+)$`)

type FileCoverInfo struct {
	File   string
	Blocks [][2]int // [start, end]
}

// ParseCoverProfile returns map of FileCoverInfo by file name or error
func ParseCoverProfile(data []byte) (map[string]*FileCoverInfo, error) {
	mapByFile := map[string]*FileCoverInfo{}
	buf := bytes.NewBuffer(data)
	// skip first mode line
	_, err := buf.ReadString('\n')
	if err != nil {
		return nil, err
	}
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				return nil, err
			} else if line == "" {
				break
			}
		}

		// clean from \r\n
		if line[len(line)-2] == '\r' {
			line = line[:len(line)-2]
		} else {
			line = line[:len(line)-1]
		}
		cline := CoverProflineRe.FindStringSubmatch(line)
		if cline == nil {
			return nil, fmt.Errorf("unexpected cover line format %q", line)
		}
		// process line
		fn := cline[1]
		info := mapByFile[fn]
		if info == nil {
			info = &FileCoverInfo{
				File: fn,
			}
			mapByFile[fn] = info
		}
		if cline[7] == "0" {
			// skip uncovered lines
			continue
		}
		start, err := strconv.Atoi(cline[2])
		if err != nil {
			return nil, err
		}
		end, err := strconv.Atoi(cline[4])
		if err != nil {
			return nil, err
		}

		info.Blocks = append(info.Blocks, [2]int{start, end})
	}

	// sort blocks to get bigger blocks
	for _, info := range mapByFile {
		sort.Slice(info.Blocks, func(i, j int) bool {
			// sort by start line
			return info.Blocks[i][0] < info.Blocks[j][0]
		})
		// merge blocks
		var blocks [][2]int
		s := 0
		for i := 1; i < len(info.Blocks); i++ {
			if info.Blocks[s][1] < info.Blocks[i][0] {
				// does not intersect
				blocks = append(blocks, info.Blocks[s])
				s = i
				continue
			}
			if info.Blocks[s][1] >= info.Blocks[i][1] {
				continue
			}
			if info.Blocks[s][1] < info.Blocks[i][1] {
				// intersects
				info.Blocks[s][1] = info.Blocks[i][1]
			}
		}
		if len(info.Blocks) > 0 {
			info.Blocks = append(blocks, info.Blocks[s])
		}
	}

	return mapByFile, err
}
