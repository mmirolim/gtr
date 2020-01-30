package main

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var ErrBuildFailed = errors.New("build failed")

type Strategy interface {
	TestsToRun(context.Context) (tests []string, subTests []string, err error)
}

var _ Strategy = (*GitDiffStrategy)(nil)

type GitDiffStrategy struct {
	workDir string
	gitCmd  *GitCMD
}

// TODO configure source code analyze algorithm
func NewGitDiffStrategy(workDir string) *GitDiffStrategy {
	return &GitDiffStrategy{
		workDir: workDir,
		gitCmd:  NewGitCMD(workDir),
	}
}

// TODO test on different modules and Gopath version
// TODO improve performance, TestsToRun testing takes more than 2s
// TODO maybe exclude tests on func main changes
// TODO imporove messages on test pass, build fails, no tests found
// TODO uneffected tests running on CommitChangeTask change
func (gds *GitDiffStrategy) TestsToRun(ctx context.Context) (testsList []string, subTestsList []string, err error) {
	changes, err := gds.gitCmd.Diff(ctx)
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
	for _, change := range changes {
		info, ok := fileInfos[change.fpath]
		if !ok {
			info, err = getFileInfo(filepath.Join(gds.workDir, change.fpath), nil)
			if err != nil {
				err = fmt.Errorf("getFileInfo error %s", err)
				return
			}
			fileInfos[change.fpath] = info
		}
	}
	changedBlocks, cerr := changesToFileBlocks(changes, fileInfos)
	if cerr != nil {
		err = fmt.Errorf("changesToFileBlocks error %s", cerr)
		return
	}

	// TODO prog remove from API
	moduleName, filePathToPkg, allSubtests, _, allPkgs, analyzeErr := analyzeGoCode(ctx, gds.workDir)
	if analyzeErr != nil {
		err = ErrBuildFailed
		return
	}
	// TODO make analyze configurable
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

	result, err := pointer.Analyze(config)
	if err != nil {
		panic(err)
	}
	graph := result.CallGraph
	graph.DeleteSyntheticNodes()
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
		fmt.Println("no updated nodes found")
		return
	}
	allTests := getAllTestsInModule(moduleName, graph)

	testsSet := map[string]bool{}
	subTests := map[string]bool{}
	for tnode := range allTests {
		callgraph.PathSearch(tnode, func(n *callgraph.Node) bool {
			if changedNodes[n] {
				funName := tnode.Func.Name()
				// TODO change to test for Run func $ is only for anon funcs
				if idx := strings.IndexByte(funName, '$'); idx != -1 {
					subTestID, err := strconv.Atoi(funName[idx+1:])
					if err != nil {
						fmt.Printf("subtest id parse error %+v\n", err) // output for debug
						return true
					}
					funName = funName[0:idx]
					set, ok := allSubtests[tnode.Func.Package().Pkg.Path()+"."+funName]
					if ok && len(set) >= subTestID {
						subTests[set[subTestID-1]] = true
					} else {
						fmt.Printf("[WARN] expected subtest %d of %s not found \n", subTestID, funName) // output for debug
					}
				}
				testsSet[funName] = true
				return true
			}
			return false
		})
	}

	return mapStrToSlice(testsSet), mapStrToSlice(subTests), nil
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

// TODO do not print errors, just return it
func analyzeGoCode(ctx context.Context, workDir string) (
	moduleName string,
	filePathToPkg map[string]string,
	allSubtests map[string][]string,
	prog *ssa.Program,
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
			fmt.Println("Module build failed\n>>>>>>>>>>>>>>>>>")
			packages.PrintErrors(pkgs)
			fmt.Println(">>>>>>>>>>>>>>>>>")
			err = errors.New("packages.Load error")
			return
		}
	}

	allSubtests = map[string][]string{}
	moduleName = pkgs[0].ID
	for i := 0; i < len(pkgs); i++ {
		pkg := pkgs[i]
		if len(moduleName) > len(pkg.ID) {
			moduleName = pkg.ID
		}
		// find all sub tests
		for _, astf := range pkg.Syntax {
			for i := range astf.Decls {
				fun, ok := astf.Decls[i].(*ast.FuncDecl)
				if !ok {
					continue
				}
				if !strings.HasPrefix(fun.Name.Name, "Test") {
					continue
				}
				ast.Inspect(fun.Body, func(n ast.Node) bool {
					callExpr, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}

					calleeName, err := fnNameFromCallExpr(callExpr)
					if err != nil || calleeName != "t.Run" {
						return true
					}

					for i := range callExpr.Args {
						if lit, ok := callExpr.Args[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							id := pkg.PkgPath + "." + fun.Name.Name
							allSubtests[id] = append(allSubtests[id],
								strings.ReplaceAll(lit.Value[1:len(lit.Value)-1], " ", "_"))
						}
					}

					return true
				})
			}
		}

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
	prog, allPkgs = ssautil.Packages(pkgs, ssa.NaiveForm|ssa.SanityCheckFunctions)
	prog.Build()
	return
}

func getAllTestsInModule(moduleName string, graph *callgraph.Graph) (
	allTests map[*callgraph.Node]bool,
) {
	allTests = map[*callgraph.Node]bool{}
	for k := range graph.Nodes {
		if k == nil || k.Package() == nil ||
			!strings.HasPrefix(k.Package().Pkg.Path(), moduleName) {
			continue
		}
		nodeType := k.Type().String()
		// filter exported Test funcs
		if strings.HasPrefix(k.Name(), "Test") &&
			(strings.Contains(nodeType, "*testing.T") ||
				strings.Contains(nodeType, "*testing.M")) {
			allTests[graph.Nodes[k]] = true
		}
	}
	return
}
