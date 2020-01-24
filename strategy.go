package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Strategy interface {
	TestsToRun() ([]string, error)
}

type Storage interface {
	Store(fname string, data []byte) error
	// TODO change entity to fileBlock?
	FindEntityCallers(fname string, ent Entity) (map[string][]Entity, error)
}

type GitDiffStrategy struct {
	store  Storage
	gitCmd *GitCMD
}

func (str *GitDiffStrategy) TestsToRun() ([]string, error) {
	changes, err := str.gitCmd.Diff()
	if err != nil {
		return nil, err
	}

	changedBlocks := map[string]FileInfo{}
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
	// TODO handle test file changes
	testsSet := map[string]struct{}{}
	for fname, info := range changedBlocks {
		for _, block := range info.blocks {
			if block.typ == "func" && strings.HasPrefix(block.name, "Test") &&
				strings.HasSuffix(fname, "_test.go") {
				// test func
				testsSet[block.name] = struct{}{}
			} else {
				// TODO filter only unique type, name blocks
				dic, err := str.store.FindEntityCallers(fname,
					Entity{typ: block.typ, name: block.name})
				if err != nil {
					if err == ErrUnsupportedType {
						continue
					}
					return nil, fmt.Errorf("store.FindEntityCallers unexpected error %+v", err)
				}
				for _, entities := range dic {
					for i := range entities {
						testsSet[entities[i].name] = struct{}{}
					}
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

func NewGitDiffStrategy(workdir string, cmd *GitCMD, store Storage) (*GitDiffStrategy, error) {
	strategy := &GitDiffStrategy{
		gitCmd: cmd, store: store,
	}
	// store all go files
	// walk current directory and all subdirectories
	err := filepath.Walk(workdir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if len(path) > 1 {
				// skip hidden and vendor dirs
				if strings.HasPrefix(filepath.Base(path), ".") || strings.HasPrefix(path, "vendor") {
					return filepath.SkipDir
				}
			}
			dir, err := os.Open(info.Name())
			if err != nil {
				return err
			}
			fileInfos, err := dir.Readdir(-1)
			if err != nil {
				return err
			}
			// close file
			dir.Close()
			for _, f := range fileInfos {
				if f.IsDir() {
					// skip
					continue
				}
				fname := filepath.Join(path, f.Name())
				// TODO parse other entities
				data, err := ioutil.ReadFile(fname)
				if err != nil {
					fmt.Printf("skip ReadFile error %s %+v\n", fname, err) // output for debug
					continue
				}
				err = store.Store(fname, data)
				if err != nil {
					fmt.Printf("getTestedFuncs unexpected error %+v\n", err) // output for debug
				}
			}
		}
		return err
	})

	return strategy, err
}
