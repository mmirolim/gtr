package main

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

var _ Strategy = (*CoverStrategy)(nil)

type CoverStrategy struct {
	firstRun bool
	workDir  string
	gitCmd   *GitCMD
	log      *log.Logger
}

func NewCoverStrategy(workDir string, logger *log.Logger) *CoverStrategy {
	return &CoverStrategy{
		firstRun: true,
		workDir:  workDir,
		gitCmd:   NewGitCMD(workDir),
		log:      logger,
	}
}
func (cs *CoverStrategy) CoverageEnabled() bool {
	return true
}

func (cs *CoverStrategy) TestsToRun(ctx context.Context) (
	runAll bool, testsList, subTestsList []string,
	err error) {
	runAll = false
	// check if dir with profile exists
	// TODO handle old cover profile, if not changed no need to update
	// or just run every day?
	profileDir := filepath.Join(cs.workDir, ".gtr")
	dir, err := os.Open(profileDir)
	if err != nil {
		// create
		err = os.Mkdir(profileDir, 0700)
		if err != nil {
			return
		}
	}

	if cs.firstRun {
		// run all tests for first time
		testsList, err = findAllTestInDir(ctx, cs.workDir)
		cs.firstRun = false
		return
	}

	// get git diffs
	// find file blocks changed
	changes, err := cs.gitCmd.Diff(ctx)
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
		info, err = getFileInfo(filepath.Join(cs.workDir, change.fpath), nil)
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

	// load all coverprofiles TODO load only changed use ts
	allFiles, err := dir.Readdirnames(0)
	if err != nil {
		return
	}

	moduleName := ""
	moduleName, err = getModuleName(cs.workDir)
	if err != nil {
		return
	}

	// [source_file_name][cover_file_name]*FileCoverInfo
	fileBlocksToTest := map[string]map[string]*FileCoverInfo{}
	var coverProfile map[string]*FileCoverInfo
	// parse files
	filePrefix := strings.ReplaceAll(moduleName, "/", "_")
	for _, name := range allFiles {
		if !strings.HasPrefix(name, filePrefix) {
			continue // skip other files
		}
		var data []byte
		data, err = ioutil.ReadFile(filepath.Join(profileDir, name))
		if err != nil {
			return
		}
		coverProfile, err = ParseCoverProfile(data)
		if err != nil {
			return
		}

		for fname, coverInfo := range coverProfile {
			if len(coverInfo.Blocks) == 0 {
				continue
			}
			testToFileCover, ok := fileBlocksToTest[fname]
			if !ok {
				testToFileCover = make(map[string]*FileCoverInfo)
			}
			testToFileCover[name] = coverInfo
			fileBlocksToTest[fname] = testToFileCover
		}
	}

	// TODO refactor
	// find tests which covers changed code blocks
	for fname, info := range changedBlocks {
		for _, block := range info.blocks {
			if block.typ&BlockFunc > 0 && strings.HasPrefix(block.name, "Test") {
				testName := block.name
				if info.pkgName == "main" {
					testName = fmt.Sprintf("%s.%s", moduleName, testName)
				} else {
					testName = fmt.Sprintf("%s.%s", filepath.Join(moduleName, info.pkgName), testName)
				}
				testsList = append(testsList, testName)
			}
		}
		fileName := filepath.Join(moduleName, fname)
		testToFileCover, ok := fileBlocksToTest[fileName]
		if !ok {
			continue
		}
		for _, block := range info.blocks {
			// find test which cover changes
			// create list of tests with packages
			for testName, profile := range testToFileCover {
				for _, profBlock := range profile.Blocks {
					if (block.start >= profBlock[0] && block.start <= profBlock[1]) ||
						(block.end >= profBlock[0] && block.end <= profBlock[1]) ||
						(profBlock[0] >= block.start && profBlock[1] <= block.end) {
						id := strings.LastIndexByte(testName, '.')
						dir := filepath.Dir(profile.File)
						testName = fmt.Sprintf("%s.%s", dir, testName[id+1:])
						testsList = append(testsList, testName)
					}
				}
			}
		}
	}
	return
}

func findAllTestInDir(ctx context.Context, dir string) ([]string, error) {
	cfg := &packages.Config{
		Context: ctx,
		Dir:     dir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedImports |
			packages.NeedDeps,
		Tests: true,
	}
	var err error
	var pkgs []*packages.Package
	// find all packages
	pkgs, err = packages.Load(cfg, filepath.Join(dir, "..."))
	if err != nil {
		return nil, err
	}

	for i := range pkgs {
		if len(pkgs[i].Errors) > 0 {
			if ctx.Err() != nil {
				err = errors.New("task canceled")
				return nil, err
			}
			err = errors.New("packages.Load error")
			return nil, err
		}
	}
	if dir == "." {
		dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	var tests []string
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		for i, file := range pkg.Syntax {
			// skip non local files
			// TODO check net package have more pkg.Syntax than pkg.GoFiles
			if i >= len(pkg.GoFiles) || !strings.HasPrefix(pkg.GoFiles[i], dir) {
				continue
			}
			// only tests
			if !strings.HasSuffix(pkg.GoFiles[i], "_test.go") {
				continue
			}
			for _, decl := range file.Decls {
				funDecl, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if strings.HasPrefix(funDecl.Name.Name, "Test") {
					tests = append(tests, fmt.Sprintf("%s.%s",
						pkg.PkgPath, funDecl.Name.Name))
				}
			}
		}
		return true
	}, nil)

	return tests, nil
}
