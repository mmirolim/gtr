package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var ErrUnsupportedType = errors.New("unsupported type")

type Strategy interface {
	TestsToRun() ([]string, error)
}

type GitDiffStrategy struct {
	gitCmd *GitCMD
}

// TODO test on different modules and Gopath version like go-radix
func (str *GitDiffStrategy) TestsToRun() ([]string, error) {
	changes, err := str.gitCmd.Diff()
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		// no changes to test
		return nil, nil
	}
	changedBlocks, err := changesToFileBlocks(changes)
	if err != nil {
		return nil, err
	}

	moduleName, filePathToPkg, prog, err := analyzeGoCode(".")
	if err != nil {
		return nil, err
	}
	// TODO make analyze configurable
	graph := cha.CallGraph(prog)
	nodesToTest := mapEntitiesToTests(moduleName, graph)
	// for k := range nodesToTest {
	// 	for node := range nodesToTest[k] {
	// 		fmt.Printf("Name %s %+v\n", k, node) // output for debug
	// 	}
	// }

	// fmt.Printf("%+v\n", "filePathToPkg") // output for debug

	testsSet := map[string]struct{}{}
	for fname, info := range changedBlocks {
		for _, block := range info.blocks {
			if block.typ == "func" && strings.HasPrefix(block.name, "Test") &&
				strings.HasSuffix(fname, "_test.go") {
				// test func
				testsSet[block.name] = struct{}{}
			} else {
				nodeName := filePathToPkg[fname] + "." + block.name
				for node := range nodesToTest[nodeName] {
					testsSet[node.Func.Name()] = struct{}{}
				}
			}
		}
	}
	tests := make([]string, 0, len(testsSet))
	for k := range testsSet {
		tests = append(tests, k)
	}
	return tests, nil
}

// TODO configure source code analyze algorithm
func NewGitDiffStrategy(cmd *GitCMD) *GitDiffStrategy {
	return &GitDiffStrategy{
		gitCmd: cmd,
	}
}

func changesToFileBlocks(changes []Change) (map[string]FileInfo, error) {
	changedBlocks := map[string]FileInfo{}
	fileInfos := map[string]FileInfo{}
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

func analyzeGoCode(workDir string) (
	moduleName string,
	filePathToPkg map[string]string,
	prog *ssa.Program,
	err error,
) {
	cfg := &packages.Config{
		Dir: workDir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedTypesSizes |
			packages.NeedTypesInfo,
		Tests: true,
	}
	var pkgs []*packages.Package
	// find all packages
	pkgs, err = packages.Load(cfg, "./...")
	if err != nil {
		return
	}
	if packages.PrintErrors(pkgs) > 0 {
		err = errors.New("analyzeGoCode error")
		return
	}
	moduleName = pkgs[0].ID
	for i := 1; i < len(pkgs); i++ {
		pkg := pkgs[i]
		if len(moduleName) > len(pkg.ID) {
			moduleName = pkg.ID
		}
	}
	filePathToPkg = map[string]string{}
	for _, pkg := range pkgs {
		path := pkg.PkgPath
		for _, file := range pkg.GoFiles {
			lid := strings.LastIndexByte(file, '/')
			if moduleName == path {
				// root dir
				filePathToPkg[file[lid+1:]] = path
			} else {
				filePathToPkg[path[len(moduleName)+1:]+"/"+file[lid+1:]] = path
			}
		}
	}
	prog, _ = ssautil.Packages(pkgs, ssa.NaiveForm|ssa.SanityCheckFunctions)
	prog.Build()
	return
}

func mapEntitiesToTests(moduleName string, graph *callgraph.Graph) map[string]map[*callgraph.Node]bool {
	nodesToTest := map[string]map[*callgraph.Node]bool{}
	for k := range graph.Nodes {
		if k == nil || k.Package() == nil ||
			!strings.HasPrefix(k.Package().Pkg.Path(), moduleName) {
			continue
		}
		nodeType := k.Type().String()
		if strings.Contains(nodeType, "*testing.T") ||
			strings.Contains(nodeType, "*testing.M") {
			testFuncNode := graph.Nodes[k]
			for _, edge := range testFuncNode.Out {
				fun := edge.Callee.Func
				path := fun.Package().Pkg.Path()
				nodeName := edge.Callee.Func.Name()
				if fun.Signature.Recv() != nil {
					// method
					path = fun.Signature.Recv().Type().String()
					if path[0] == '*' { // pointer type
						path = path[1:]
					}
					// store all Test for Type method as test for a Type
					tests, ok := nodesToTest[path]
					if !ok {
						tests = map[*callgraph.Node]bool{}
						nodesToTest[path] = tests
					}

					tests[edge.Caller] = true
				}

				nodeName = path + "." + nodeName
				tests, ok := nodesToTest[nodeName]
				if !ok {
					tests = map[*callgraph.Node]bool{}
					nodesToTest[nodeName] = tests
				}

				tests[edge.Caller] = true
			}
		}
	}
	return nodesToTest
}
