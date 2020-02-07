[![Build Status](https://travis-ci.org/mmirolim/gtr.svg)](https://travis-ci.org/mmirolim/gtr)
[![GoDoc](https://godoc.org/github.com/mmirolim/gtr?status.svg)](http://godoc.org/github.com/mmirolim/gtr)
[![codecov](https://codecov.io/gh/mmirolim/gtr/branch/master/graph/badge.svg)](https://codecov.io/gh/mmirolim/gtr)
[![Go Report Card](https://goreportcard.com/badge/github.com/mmirolim/gtr)](https://goreportcard.com/badge/github.com/mmirolim/gtr)

# GTR - Go Test Runner

Go auto testing tool with cruise control. GTR uses std lib pointer analysis to find affected tests by file changes (git diff) and runs only them then notifies using desktop notifications. You don't need to define manually a particular test/package to run or wait for all repository tests to run. Get faster and more reliable feedback from your tests without too much context switching between testing and writing code. Good fit for TDD and CD.

[![asciicast](https://asciinema.org/a/NWCyhilRr7bdbrnVd0b9CVaKv.svg)](https://asciinema.org/a/NWCyhilRr7bdbrnVd0b9CVaKv?t=10)

## Edge cases

- Alpha stage
- Works only with go standard testing library
- May not find all affected tests
- Reflection operations are not supported (may not resolve affected code)
- May become slow with big projects
- Basic support for subtests t.Run
- Needs more extensive testing (tested only on linux and darwin)
- Does not work with monorepo

# Installation
	
 gtr requires go and git commands to be available
	
	go get -u github.com/mmirolim/gtr

	
# Usage
	
 Run it in git enabled project folder
 
	gtr
	gtr: watcher running...
	
 
 
	gtr help
	Usage of gtr:
	-C string
		  directory to watch (default ".")
	-args string
		  args to the test binary
	-auto-commit string
		  auto commit on tests pass (default false)
	-delay int
		  delay in Milliseconds (default 1000)
	-exclude-dirs string
		  prefixes to exclude sep by comma (default "vendor,node_modules")
	-exclude-file-prefix string
		  prefixes to exclude sep by comma (default "#")
		
 It uses default go test cmd to run tests, cpu and gtr itself is limited to NumCPU/2 so it will run smoothly along

	go test -v -vet off -failfast -cpu 2 -run TestZ$|TestC$/(A=1|B=2) ./... -args -x -v


 
