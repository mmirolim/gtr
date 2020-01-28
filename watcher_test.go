package main

import (
	"os"
	"path/filepath"
	"testing"
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
