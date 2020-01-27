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

var debug = true

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
	tasks               []Task
	delay               time.Duration
	excludeFilePrefixes []string
	excludeDirs         []string
	events              chan fsnotify.Event
	errs                chan error
}

func NewWatcher(tasks []Task,
	delay int,
	excludeFilePrefixes []string,
	excludeDirs []string,
) Watcher {
	return Watcher{
		tasks:               tasks,
		delay:               time.Duration(delay) * time.Millisecond,
		excludeFilePrefixes: excludeFilePrefixes,
		excludeDirs:         excludeDirs,
		events:              make(chan fsnotify.Event),
		errs:                make(chan error),
	}
}

// Blocks
func (w *Watcher) Run() {
	fmt.Println("watcher running...")
	done := make(chan bool)
	// watch files
	go w.watchFiles()
	// start listening to notifications in separate goroutine
	go w.runTasks()

	// block
	<-done
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
		case e := <-w.events:
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
				w.printDebug("command executed")
			}()
			// process started incr rerun counter
			rerunCounter++

		case err := <-w.errs:
			log.Println("Error:", err)
		}
	}
}

// recursively set watcher to all child directories
// and fan-in all events and errors to chan in main loop
// TODO support watching new added directories
func (w *Watcher) watchFiles() {
	// walk current directory and if there is other directory add watcher to it
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if len(path) > 1 {
				if w.skipDir(path) {
					return filepath.SkipDir
				}
			}
			// create new watcher
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				log.Fatal(err)
			}
			// add watcher to dir
			err = watcher.Add(path)
			if err != nil {
				errClose := watcher.Close()
				log.Fatal(errClose, err)
			}
			w.printDebug("dir to watch", path)
			go func() {
				for {
					select {
					case v := <-watcher.Events:
						// on event send data to shared event chan
						w.events <- v
					case err := <-watcher.Errors:
						// on error send data to shared error chan
						w.errs <- err
					}
				}
			}()
		}
		return err
	})
	if err != nil {
		fmt.Println("filepath walk err " + err.Error())
	}
}

func (w *Watcher) printDebug(args ...interface{}) {
	// TODO get call stack previous pc to correctly show line numbers
	if debug {
		log.Println(args...)
	}
}
