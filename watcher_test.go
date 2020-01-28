package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatcherAddDirs(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "test_watcher_add_dirs")
	// teardown
	defer func() {
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(testDir)
		}
	}()

	var watcher *Watcher
	var err error
	cases := []struct {
		desc            string
		setup, tearDown func() error
		dirsWatching    int
		err             error
	}{

		{
			desc: "Create directories and add to watcher",
			setup: func() error {
				_ = os.Mkdir(filepath.Join(testDir, ".hidden"), 0700)
				_ = os.MkdirAll(filepath.Join(testDir, "ad", "add", "addd"), 0700)
				err = os.MkdirAll(filepath.Join(testDir, "bd", "bdd"), 0700)
				if err != nil {
					return err
				}

				watcher, err = NewWatcher(testDir, nil, 0, nil, nil)
				return err
			},
			tearDown: func() error {
				return watcher.Stop()
			},
			dirsWatching: 6, err: nil,
		},
		{
			desc: "Exclude add directory from watching",
			setup: func() error {
				watcher, err = NewWatcher(testDir, nil, 0, nil, []string{"add"})
				return err
			},
			tearDown: func() error {
				return watcher.Stop()
			},
			dirsWatching: 4, err: nil,
		},
	}
	for i, tc := range cases {
		// setup()
		execTestHelper(t, i, tc.desc, tc.setup)

		err = watcher.addDirs()
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}

		if len(watcher.dirs) != tc.dirsWatching {
			t.Errorf("case [%d] %s\nexpected %+v\ngot %+v",
				i, tc.desc, tc.dirsWatching, len(watcher.dirs))
		}
		// teardown()
		execTestHelper(t, i, tc.desc, tc.tearDown)

	}
}

func TestWatcherSkipChange(t *testing.T) {
	cases := []struct {
		desc                string
		excludeFilePrefixes []string
		event               fsnotify.Event
		lastModTime         time.Time
		lastModFile         string
		expect              bool
	}{

		{
			desc:   "new file Create event",
			event:  fsnotify.Event{"file.go", fsnotify.Create},
			expect: true,
		},
		{
			desc:   "file.go Rename event",
			event:  fsnotify.Event{"file.go", fsnotify.Rename},
			expect: true,
		},
		{
			desc:   "file.go Remove event",
			event:  fsnotify.Event{"file.go", fsnotify.Remove},
			expect: true,
		},
		{
			desc:   "file.go Write event",
			event:  fsnotify.Event{"file.go", fsnotify.Write},
			expect: false,
		},
		{
			desc:        "file Write event, updated < delay",
			event:       fsnotify.Event{"file.go", fsnotify.Write},
			lastModTime: time.Now().Add(-100 * time.Millisecond),
			lastModFile: "file.go",
			expect:      true,
		},
		{
			desc:   "file.js file Write event",
			event:  fsnotify.Event{"file.js", fsnotify.Write},
			expect: true,
		},
		{
			desc:                "prefixfile.go skipped by prefix Write event",
			excludeFilePrefixes: []string{"prefix", "otherprefix"},
			event:               fsnotify.Event{"prefixfile.go", fsnotify.Write},
			lastModTime:         time.Time{},
			lastModFile:         "file.go",
			expect:              true,
		},
	}

	for i, tc := range cases {
		watcher := &Watcher{
			delay:               1000 * time.Millisecond,
			excludeFilePrefixes: tc.excludeFilePrefixes,
		}
		if watcher.skipChange(tc.event, tc.lastModFile, tc.lastModTime) != tc.expect {
			t.Errorf("case [%d] %s\nexpected %+v\ngot %+v",
				i, tc.desc, tc.expect, !tc.expect)
		}
	}
}
