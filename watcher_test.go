package main

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
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

		// teardown()
		execTestHelper(t, i, tc.desc, tc.tearDown)
		if isUnexpectedErr(t, i, tc.desc, tc.err, err) {
			continue
		}

		if len(watcher.dirs) != tc.dirsWatching {
			t.Errorf("case [%d] %s\nexpected %+v\ngot %+v",
				i, tc.desc, tc.dirsWatching, len(watcher.dirs))
		}

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
			expect: false,
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

func TestWatcherRunTasks(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "test_watcher_run_tasks")
	os.RemoveAll(testDir)
	// teardown
	defer func() {
		if !t.Failed() {
			// clean tmp dir on test success
			_ = os.RemoveAll(testDir)
		}
	}()

	var watcher *Watcher
	var err error
	var taskOutput string
	var taskErr error

	taskCanceledErr := errors.New("task canceled")
	cases := []struct {
		desc               string
		setup, tearDown    func() error
		expectedTaskOutput string
		expectedTaskErr    error
	}{

		{
			desc: "Run tasks/stop tasks",
			setup: func() error {
				// prepare dir tree
				_ = os.Mkdir(filepath.Join(testDir, ".hidden"), 0700)
				_ = os.MkdirAll(filepath.Join(testDir, "ad", "add", "addd"), 0700)
				err = os.MkdirAll(filepath.Join(testDir, "bd", "bdd"), 0700)
				if err != nil {
					return err
				}
				task1 := NewTask("task1", func(ctx context.Context) (string, error) {
					<-ctx.Done()
					taskErr = taskCanceledErr
					return taskErr.Error(), taskErr

				})
				watcher, _ = NewWatcher(testDir, []Task{task1}, 0, nil, nil)
				_ = watcher.addDirs()
				// run tasks
				go watcher.runTasks()
				return ioutil.WriteFile(filepath.Join(testDir, "file.go"), nil, 0600)
			},
			tearDown: func() error {
				taskErr = nil
				taskOutput = ""
				return nil
			},
			expectedTaskOutput: "", expectedTaskErr: taskCanceledErr,
		},
		{
			desc: "Run multiple tasks in order file > task1 > task2",
			setup: func() error {
				task1 := NewTask("task1", func(ctx context.Context) (string, error) {
					fname, ok := ctx.Value(changedFileNameKey).(string)
					if !ok {
						return "", taskCanceledErr
					}
					taskOutput = fname + ">task1"
					return taskOutput, nil

				})
				task2 := NewTask("task2", func(ctx context.Context) (string, error) {
					prevOut, ok := ctx.Value(prevTaskOutputKey).(string)
					if !ok {
						return "", taskCanceledErr
					}
					taskOutput = prevOut + ">task2"
					return taskOutput, nil

				})
				watcher, _ = NewWatcher(testDir, []Task{task1, task2}, 0, nil, nil)
				_ = watcher.addDirs()
				// run tasks
				go watcher.runTasks()
				return ioutil.WriteFile(filepath.Join(testDir, "file.go"), nil, 0600)
			},
			tearDown: func() error {
				taskErr = nil
				taskOutput = ""
				return nil
			},
			expectedTaskOutput: filepath.Join(testDir, "file.go>task1>task2"),
			expectedTaskErr:    nil,
		},
		{
			desc: "Do not trigger task on none go files",
			setup: func() error {
				gotask := NewTask("gotask", func(ctx context.Context) (string, error) {
					return "should not run", errors.New("should not run")
				})
				watcher, _ = NewWatcher(testDir, []Task{gotask}, 0, nil, nil)
				_ = watcher.addDirs()
				// run tasks
				go watcher.runTasks()
				return ioutil.WriteFile(filepath.Join(testDir, "file.js"), nil, 0600)
			},
			tearDown: func() error {
				taskErr = nil
				taskOutput = ""
				return nil
			},
			expectedTaskOutput: "", expectedTaskErr: nil,
		},
		{
			desc: "Add new directory to a watch list",
			setup: func() error {
				task := NewTask("task", func(ctx context.Context) (string, error) {
					fname := ctx.Value(changedFileNameKey).(string)

					_, file := filepath.Split(fname)
					if file != "file_in_new_dir.go" {
						return "unexpected file", taskCanceledErr
					}
					taskOutput = "OK"
					return taskOutput, nil
				})
				watcher, _ = NewWatcher(testDir, []Task{task}, 0, nil, nil)
				_ = watcher.addDirs()
				// run tasks
				go watcher.runTasks()
				// create new directory
				newDir := filepath.Join(testDir, "newdir")
				os.Mkdir(newDir, 0700)
				// delay for a watcher to add newdir
				time.Sleep(time.Millisecond)
				return ioutil.WriteFile(
					filepath.Join(newDir, "file_in_new_dir.go"), nil, 0600)
			},
			tearDown: func() error {
				taskErr = nil
				taskOutput = ""
				return nil
			},
			expectedTaskOutput: "OK", expectedTaskErr: nil,
		},
		{
			desc: "Remove deleted directory from a watch list",
			setup: func() error {
				task := NewTask("task_return_dirs", func(ctx context.Context) (string, error) {
					taskOutput = strconv.Itoa(len(watcher.dirs))
					return taskOutput, nil
				})
				watcher, _ = NewWatcher(testDir, []Task{task}, 0, nil, nil)
				_ = watcher.addDirs()
				// run tasks
				go watcher.runTasks()
				// remove new directory
				time.Sleep(time.Millisecond)
				os.RemoveAll(filepath.Join(testDir, "newdir"))
				return ioutil.WriteFile(
					filepath.Join(testDir, "file_update.go"), nil, 0600)
			},
			tearDown: func() error {
				taskErr = nil
				taskOutput = ""
				return nil
			},
			expectedTaskOutput: "6", expectedTaskErr: nil,
		},
	}
	for i, tc := range cases {
		if i > 0 {
			// teardown
			prev := i - 1
			execTestHelper(t, prev, cases[prev].desc, cases[prev].tearDown)
		}
		// setup()
		execTestHelper(t, i, tc.desc, tc.setup)
		// wait watcher to process changes
		time.Sleep(time.Millisecond)

		err = watcher.Stop()
		if isUnexpectedErr(t, i, tc.desc, nil, err) {
			continue
		}
		// wait task watcher runTasks returns
		time.Sleep(time.Millisecond)
		if isUnexpectedErr(t, i, tc.desc, tc.expectedTaskErr, taskErr) {
			continue
		}

		if tc.expectedTaskOutput != taskOutput {
			t.Errorf("case [%d] %s\nexpected %+v\ngot %+v",
				i, tc.desc, tc.expectedTaskOutput, taskOutput)
		}
	}
}
