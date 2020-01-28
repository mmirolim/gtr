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

type Task interface {
	ID() string
	Run(ctx context.Context) (msg string, err error)
}

type TaskCtxKey string

const (
	changedFileNameKey TaskCtxKey = "changed_file_name_key"
	prevTaskOutput     TaskCtxKey = "prev_task_output_key"
)

type NotificationService interface {
	Send(msg string) error
}

// TODO test passing args to test run
type Watcher struct {
	*fsnotify.Watcher
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
		Watcher:             watcher,
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
	if e.Op&fsnotify.Write != fsnotify.Write {
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
	}
	return false
}

func (w *Watcher) skipDir(dir string) bool {
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
	// keep track of reruns
	rerunCounter := 1
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
			fmt.Println("Watcher.runTasks quit")
			return

		case e := <-w.Events:
			if w.skipChange(e, lastModFile, lastModTime) {
				continue LOOP
			}
			log.Println("File changed:", e.Name)
			lastModFile = e.Name
			lastModTime = time.Now()
			if cancel != nil {
				cancel()
			}

			ctx, cancel = context.WithCancel(
				context.WithValue(context.Background(), changedFileNameKey, e.Name))
			//time.Sleep(w.delay)
			// do not block loop
			go func() {
				var output string
				var err error
				// run tasks in provided sequence
				for _, task := range w.tasks {
					ctx = context.WithValue(ctx, prevTaskOutput, output)
					fmt.Printf("Run task.ID %+v\n", task.ID()) // output for debug
					output, err = task.Run(ctx)
					if err != nil {
						fmt.Printf("stop pipeline Task.ID: %s returned err %+v\n", task.ID(), err) // output for debug
						break
					}
				}
				// add loging
				fmt.Println("command executed")
			}()
			// process started incr rerun counter
			rerunCounter++

		case err := <-w.Errors:
			log.Println("Error:", err)
		}
	}
}

// recursively set watcher to all child directories
// and fan-in all events and errors to chan in main loop
// TODO support watching new added directories
func (w *Watcher) addDirs() error {
	// walk current directory and if there is other directory add watcher to it
	err := filepath.Walk(w.workDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}
		if w.skipDir(path) {
			return filepath.SkipDir
		}
		// add watcher to dir
		err = w.Add(path)
		if err != nil {
			fmt.Printf("could not add dir to watcher %s\n", err)
			return filepath.SkipDir
		}
		w.dirs[path] = true
		return nil
	})

	return err
}

// Stop Watcher
func (w *Watcher) Stop() error {
	defer close(w.quit)
	return w.Close()
}
