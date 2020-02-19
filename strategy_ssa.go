package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// ErrBuildFailed is the error returned when source build fails
var ErrBuildFailed = errors.New("build failed")

var _ Strategy = (*SSAStrategy)(nil)

// SSAStrategy finds test to run from git diffs
// and pointer analysis
type SSAStrategy struct {
	analysis string
	workDir  string
	gitCmd   *GitCMD
	log      *log.Logger
}

// NewSSAStrategy returns strategy
func NewSSAStrategy(analysis, workDir string, logger *log.Logger) *SSAStrategy {
	return &SSAStrategy{
		analysis: analysis,
		workDir:  workDir,
		gitCmd:   NewGitCMD(workDir),
		log:      logger,
	}
}

func (ss *SSAStrategy) CoverageEnabled() bool {
	return false
}

// TestsToRun returns names of tests and subtests
// affected by files
// TODO test on different modules and Gopath version
// TODO improve performance, TestsToRun testing takes more than 4s
func (ss *SSAStrategy) TestsToRun(ctx context.Context) (
	runAll bool, testsList, subTestsList []string, err error) {
	changes, err := ss.gitCmd.Diff(ctx)
	if err != nil {
		err = fmt.Errorf("gitCmd.Diff error %s", err)
		return
	}
	// filter out none go files
	n := 0
	for _, x := range changes {
		if strings.HasSuffix(x.fpath, ".go") {
			changes[n] = x
			n++
		}

	}

	changes = changes[:n]
	if len(changes) == 0 {
		// no changes to test
		return
	}

	fileInfos := map[string]FileInfo{}
	var info FileInfo
	for _, change := range changes {
		if _, ok := fileInfos[change.fpath]; ok {
			continue
		}
		info, err = getFileInfo(filepath.Join(ss.workDir, change.fpath), nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "\n=======\033[31m Build Failed \033[39m=======")
			fmt.Fprintf(os.Stderr, "%s", err)
			fmt.Fprintln(os.Stderr, "\n============================")
			err = fmt.Errorf("getFileInfo error %s", err)
			return
		}
		fileInfos[change.fpath] = info
	}
	changedBlocks, cerr := changesToFileBlocks(changes, fileInfos)
	if cerr != nil {
		err = fmt.Errorf("changesToFileBlocks error %s", cerr)
		return
	}

	moduleName, program, filePathToPkg, allPkgs, analyzeErr := analyzeGoCode(ctx, ss.workDir)
	if analyzeErr != nil {
		err = ErrBuildFailed
		return
	}

	// TODO test with libraries without entry point
	var testPkgs []*ssa.Package
	for _, pkg := range allPkgs {
		if strings.HasSuffix(pkg.Pkg.Path(), ".test") {
			testPkgs = append(testPkgs, pkg)
		}
	}
	config := &pointer.Config{
		Mains:          ssautil.MainPackages(testPkgs),
		BuildCallGraph: true,
	}
	var graph *callgraph.Graph
	// configure analysis
	switch ss.analysis {
	case "pointer":
		result, aerr := pointer.Analyze(config)
		if aerr != nil {
			err = aerr
			return
		}
		graph = result.CallGraph
	case "static":
		graph = static.CallGraph(program)
	case "cha":
		graph = cha.CallGraph(program)
	case "rta":
		var ssaFuncs []*ssa.Function
		for fn := range ssautil.AllFunctions(program) {
			if fn != nil {
				ssaFuncs = append(ssaFuncs, fn)
			}
		}
		graph = rta.Analyze(ssaFuncs, true).CallGraph
	default:
		return // unhandled analysis
	}
	graph.DeleteSyntheticNodes() // check
	// find nodes from changed blocks
	changedNodes := map[*callgraph.Node]bool{}
	for fn := range graph.Nodes {
		if fn == nil || fn.Package() == nil {
			continue
		}
		pkgPath := fn.Package().Pkg.Path()
		for fname, info := range changedBlocks {
			if pkgPath != filePathToPkg[fname] {
				continue
			}
			for _, block := range info.blocks {
				// store all nodes
				if (block.typ&BlockFunc > 0 && fn.Name() == block.name) ||
					(block.typ&BlockMethod > 0 && len(fn.Params) > 0 &&
						strings.HasSuffix(fn.Params[0].Type().String()+"."+fn.Name(), block.name)) {
					changedNodes[graph.Nodes[fn]] = true
					break
				}
			}
		}
	}
	if len(changedNodes) == 0 {
		ss.log.Println("no updated nodes found")
		return
	}
	allTests := getAllTestsInModule(moduleName, graph)

	testsSet := map[string]bool{}
	subTests := map[string]bool{}
	for tnode := range allTests {
		callgraph.PathSearch(tnode, func(n *callgraph.Node) bool {
			if !changedNodes[n] {
				return false
			}
			funName := tnode.Func.Name()
			pkgPath := tnode.Func.Pkg.Pkg.Path()
			for {
				idx := strings.LastIndexByte(funName, '$')
				// is anon func
				for tn, dic := range allTests {
					if subName, ok := dic[funName]; ok {
						// add all subtest with this helper
						// TODO maybe use pkg as prefix
						subTests[subName] = true
						if strings.LastIndexByte(tn.Func.Name(), '$') == -1 {
							testsSet[fmt.Sprintf("%s.%s", pkgPath, tn.Func.Name())] = true
						}

					}
				}
				if idx > -1 {
					funName = funName[0:idx]
				} else if len(funName) > 4 && funName[0:4] == "Test" {
					testsSet[fmt.Sprintf("%s.%s", pkgPath, funName)] = true
					break
				} else {
					break
				}
			}
			return true
		})
	}

	return true, mapStrToSlice(testsSet), mapStrToSlice(subTests), nil
}

func changesToFileBlocks(changes []Change, fileInfos map[string]FileInfo) (map[string]FileInfo, error) {
	changedBlocks := map[string]FileInfo{}
	// process all changes
	for _, change := range changes {
		info, ok := fileInfos[change.fpath]
		if !ok {
			return nil, errors.New("missing FileInfo of " + change.fpath)
		}
		changeInfo, ok := changedBlocks[change.fpath]
		if !ok {
			changeInfo.fname = info.fname
			changeInfo.pkgName = info.pkgName
			changeInfo.endLine = info.endLine
		}

		// expect blocks sorted by start line
		for _, block := range info.blocks {
			if change.start == 0 && change.count == 0 {
				// new untracked file
				changeInfo.blocks = append(changeInfo.blocks, block)
				continue
			}
			// changes are from unified diff format
			// with 0 lines of context
			start := change.start
			end := change.count + start
			if start < end {
				end--
			}

			if end < block.start {
				break
			}
			if (start >= block.start && start <= block.end) ||
				(end >= block.start && end <= block.end) ||
				(block.start >= start && block.end <= end) {
				if len(changeInfo.blocks) > 0 {
					last := changeInfo.blocks[len(changeInfo.blocks)-1]
					if last.name == block.name && last.typ == block.typ {
						// skip multiple changes for same file block
						continue
					}
				}
				changeInfo.blocks = append(changeInfo.blocks, block)
			}
		}
		if len(changeInfo.blocks) > 0 {
			changedBlocks[change.fpath] = changeInfo
		}

	}
	return changedBlocks, nil
}

func analyzeGoCode(ctx context.Context, workDir string) (
	moduleName string,
	prog *ssa.Program,
	filePathToPkg map[string]string,
	allPkgs []*ssa.Package,
	err error,
) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     workDir,
		Mode: packages.NeedName |
			packages.NeedFiles |
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
	pkgs, err = packages.Load(cfg, workDir+"/...")
	if err != nil {
		return
	}

	for i := range pkgs {
		if len(pkgs[i].Errors) > 0 {
			fmt.Fprintln(os.Stderr, "\n=======\033[31m Build Failed \033[39m=======")
			select {
			case <-ctx.Done():
				fmt.Fprintln(os.Stderr, "task canceled")
				err = errors.New("task canceled")
				return
			default:
			}
			packages.PrintErrors(pkgs)
			fmt.Fprintln(os.Stderr, "\n============================")
			err = errors.New("packages.Load error")
			return
		}
	}

	moduleName, err = getModuleName(workDir)
	if err != nil {
		return
	}

	// TODO test without go mod, in GOPATH
	filePathToPkg = map[string]string{}
	for _, pkg := range pkgs {
		path := pkg.PkgPath
		if !strings.HasPrefix(path, moduleName) {
			// skip none module packages
			continue
		}
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
	// create program
	prog, allPkgs = ssautil.Packages(pkgs, ssa.NaiveForm|ssa.SanityCheckFunctions)
	prog.Build()
	return
}

func getAllTestsInModule(moduleName string, graph *callgraph.Graph) (
	allTests map[*callgraph.Node]map[string]string,
) {
	// Top level test node -> t.Run helper func -> t.Run name
	allTests = map[*callgraph.Node]map[string]string{}
	for k, n := range graph.Nodes {
		if k == nil || k.Package() == nil ||
			!strings.HasPrefix(k.Package().Pkg.Path(), moduleName) {
			continue
		}
		nodeType := k.Type().String()
		// TODO what to do with helperTesting(t *testing.T[, args ...]) ?
		// find testing funcs
		if strings.Contains(nodeType, "*testing.T") ||
			strings.Contains(nodeType, "*testing.M") {
			allTests[n] = nil
			for _, block := range k.Blocks {
				for j, instr := range block.Instrs {
					if !strings.HasPrefix(block.Instrs[j].String(), "(*testing.T).Run") {
						continue
					}
					// (*testing.T).Run(t7, "max":string, helperMax)
					opers := instr.Operands(nil)
					if (*opers[2]).Name()[0] == 't' {
						// skip
						continue
					}
					if allTests[n] == nil {
						allTests[n] = map[string]string{}
					}
					subTestName := (*opers[2]).Name()
					allTests[n][(*opers[3]).Name()] = subTestName[1 : len(subTestName)-len(":string")-1]

				}
			}
		}

	}
	return
}
