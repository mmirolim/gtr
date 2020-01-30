package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type TaskCtxKey string

const (
	changedFileNameKey TaskCtxKey = "changed_file_name_key"
	prevTaskOutputKey  TaskCtxKey = "prev_task_output_key"
)

var _ Task = (*taskAdapter)(nil)

type Task interface {
	ID() string
	Run(ctx context.Context) (msg string, err error)
}

// TODO test passing args to test run
// TODO use logger
type Watcher struct {
	wt                  *fsnotify.Watcher
	workDir             string
	dirs                map[string]bool
	tasks               []Task
	delay               time.Duration
	excludeFilePrefixes []string
	excludeDirs         []string
	quit                chan bool
}

func NewWatcher(
	workDir string,
	tasks []Task,
	delay int,
	excludeFilePrefixes []string,
	excludeDirs []string,
) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	return &Watcher{
		wt:                  watcher,
		workDir:             workDir,
		dirs:                make(map[string]bool),
		tasks:               tasks,
		delay:               time.Duration(delay) * time.Millisecond,
		excludeFilePrefixes: excludeFilePrefixes,
		excludeDirs:         excludeDirs,
		quit:                make(chan bool),
	}, err
}

// Blocks
func (w *Watcher) Run() error {
	fmt.Println("watcher running...")
	// watch directories recursively
	err := w.addDirs()
	if err != nil {
		return err
	}
	// start listening to notifications in separate goroutine
	go w.runTasks()

	// block
	<-w.quit
	return nil
}

func (w *Watcher) skipChange(
	e fsnotify.Event,
	lastModFile string,
	lastModTime time.Time) bool {
	if e.Op&(fsnotify.Write|fsnotify.Create) == 0 {
		return true
	}
	if lastModFile == e.Name {
		if time.Since(lastModTime) <= w.delay {
			return true
		}
	}
	if !strings.HasSuffix(e.Name, ".go") {
		return true
	}
	name := path.Base(e.Name)
	for _, prefix := range w.excludeFilePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	} // TODO
	return false
}

func (w *Watcher) skipDir(dir string) bool {
	if dir == w.workDir {
		return false
	}
	baseDir := filepath.Base(dir)
	// skip hidden dirs
	if strings.HasPrefix(baseDir, ".") {
		return true
	}
	for _, name := range w.excludeDirs {
		if baseDir == name {
			return true
		}
	}
	return false
}

// main loop to listen all events from all registered directories
// and exec tasks
func (w *Watcher) runTasks() {
	var ctx context.Context
	var cancel context.CancelFunc
	lastModTime := time.Now()
	lastModFile := ""
LOOP:
	for {
		select {
		case <-w.quit:
			// quit task
			if cancel != nil {
				// stop tasks
				cancel()
			}
			return

		case e := <-w.wt.Events:
			if e.Op&fsnotify.Remove > 0 && w.dirs[e.Name] {
				// remove from watching list
				// fsnotify auto cleans on delete
				delete(w.dirs, e.Name)
				continue LOOP
			}
			info, err := os.Stat(e.Name)
			if err != nil {
				continue LOOP
			}
			if info.IsDir() {
				err := w.add(e.Name)
				if err != nil {
					fmt.Printf("watcher add unexpected err %+v\n", err) // output for debug
				}
				continue LOOP
			}
			if w.skipChange(e, lastModFile, lastModTime) {
				continue LOOP
			}
			// add some delay, there is a race
			// for git index lock in current dir, if some other process
			// use git
			time.Sleep(w.delay / 10)
			log.Println("File changed:", e.Name)
			lastModFile = e.Name
			lastModTime = time.Now()
			if cancel != nil {
				cancel()
			}

			ctx, cancel = context.WithCancel(
				context.WithValue(context.Background(), changedFileNameKey, e.Name))
			// do not block loop
			go func() {
				var output string
				var err error
				// run tasks in provided sequence
				for _, task := range w.tasks {
					ctx = context.WithValue(ctx, prevTaskOutputKey, output)
					fmt.Printf("Run task.ID %+v\n", task.ID()) // output for debug
					output, err = task.Run(ctx)
					if err != nil {
						fmt.Printf("stop pipeline Task.ID: %s returned err %+v\n", task.ID(), err) // output for debug
						break
					}
				}
				// add loging
				fmt.Println("tasks executed")
			}()

		case err := <-w.wt.Errors:
			if err != nil {
				log.Println("Error:", err)
			}
		}
	}
}

// add dir to watch list
func (w *Watcher) add(path string) error {
	if w.skipDir(path) {
		return filepath.SkipDir
	}
	// add watcher to dir
	err := w.wt.Add(path) // TEST
	if err != nil {
		fmt.Printf("could not add dir to watcher %s\n", err)
		return filepath.SkipDir
	}
	w.dirs[path] = true
	return nil
}

// recursively adds directories to a watcher
func (w *Watcher) addDirs() error { // TODO
	// walk current directory and if there is other directory add watcher to it
	err := filepath.Walk(w.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() { // TODO
			return nil
		}
		err = w.add(path)
		if err != nil {
			return err
		}
		w.dirs[path] = true
		return nil
	})
	// TOOD check
	return err
}

// Stop Watcher
func (w *Watcher) Stop() error {
	defer close(w.quit)
	return w.wt.Close()
}

func NewTask(id string, fn func(context.Context) (string, error)) Task {
	return taskAdapter{id, fn}
}

type taskAdapter struct {
	id string
	fn func(context.Context) (string, error)
}

func (ta taskAdapter) ID() string {
	return ta.id
}

func (ta taskAdapter) Run(ctx context.Context) (string, error) {
	return ta.fn(ctx)
}
