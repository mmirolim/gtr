[![Build Status](https://travis-ci.org/mmirolim/gtr.svg)](https://travis-ci.org/mmirolim/gtr)
[![GoDoc](https://godoc.org/github.com/mmirolim/gtr?status.svg)](http://godoc.org/github.com/mmirolim/gtr)
[![codecov](https://codecov.io/gh/mmirolim/gtr/branch/master/graph/badge.svg)](https://codecov.io/gh/mmirolim/gtr)
[![Go Report Card](https://goreportcard.com/badge/github.com/mmirolim/gtr)](https://goreportcard.com/badge/github.com/mmirolim/gtr)

# GTR - Go Test Runner

Go auto testing tool with cruise control. GTR uses coverage profile data and/or pointer analysis to find affected tests by file changes (git diff) and runs only identified subset of tests then notifies using desktop notifications. You don't need to define manually a particular test or package to run or wait for all repository tests to run. GTR helps to get faster and more reliable feedback from your tests without too much context switching between testing and writing code. Good fit for TDD and CD.

[![asciicast](https://asciinema.org/a/NWCyhilRr7bdbrnVd0b9CVaKv.svg)](https://asciinema.org/a/NWCyhilRr7bdbrnVd0b9CVaKv?t=10)

## Edge cases

- At an early stage of development
- Works (tested) only with go standard testing library
- May not find all affected tests (using different strategies and/or analysis may help)
- Reflection operations are not supported (may not resolve affected code, coverage strategy may help)
- May become slow with big projects (try coverage strategy)
- May fail with “too many open files” error if file-descriptors limit is not enough
- Needs more extensive testing (tested only on linux and darwin)

# Installation
	
 gtr requires go and git commands to be available
	
	go get -u github.com/mmirolim/gtr

	
# Usage
	
 Run it in git enabled project root folder or pass -C flag with a path to the git root directory.
 
 There are 2 strategies:
 
- analysis - uses source code analysis using pointer/static/cha/rta algorithm from golang.org/x/tools/go
- coverage - uses coverage profile data to find tests which are affected by file changes
    
 By default -strategy=analysis -analysis=pointer is used.
 If -strategy=coverage used, gtr runs all tests on startup to update coverage data. Coverage data will be stored in .gtr directory. To use old data set -run-init flag to false. 
 
	gtr
	gtr: watcher running...
	
 
 
	gtr help
	Usage of gtr:
	  -C string
			directory to watch (default ".")
	  -strategy string
			strategy analysis or coverage (default analysis)
	  -analysis string
			source code analysis to use pointer, static, rta, cha (default pointer)
	  -run-init bool
			runs init steps like on first run get coverage for all tests on coverage strategy (default true)
	  -args string
			args to the test binary
	  -auto-commit bool
			auto commit on tests pass (default false)
	  -delay int
			delay in Milliseconds (default 1000)
	  -exclude-dirs string
			prefixes to exclude sep by comma (default "vendor,node_modules")
	  -exclude-file-prefix string
			prefixes to exclude sep by comma (default "#")
			
 It uses default go test cmd to run tests, cpu and gtr itself is limited to NumCPU/2 so it will run smoothly along

	go test -v -vet off -failfast -cpu 2 -run TestZ$|TestC$/(A=1|B=2) pkga pkgb -args -x -v


 
