package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// TODO do not block on when run cmd exits, properly handler exits
var debug = true

type Task interface {
	ID() string
	Run(fname string, stop <-chan bool) (msg string, err error)
}

type NotificationService interface {
	Send(msg string) error
}

// TODO add gnome notifications
// TODO test passing args to test run
// TODO define notification expire time
// TODO add notification service
type Watcher struct {
	notificator     NotificationService
	tasks           []Task
	delay           time.Duration
	excludePrefixes []string
	events          chan fsnotify.Event
	errs            chan error
	stop            chan bool
}

func NewWatcher(tasks []Task,
	notificator NotificationService,
	delay int,
	excludePrefixes []string,
) Watcher {
	return Watcher{
		notificator:     notificator,
		tasks:           tasks,
		delay:           time.Duration(delay) * time.Millisecond,
		excludePrefixes: excludePrefixes,
		events:          make(chan fsnotify.Event),
		errs:            make(chan error),
		stop:            make(chan bool),
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

// main loop to listen all events from all registered directories
// and exec required commands, kill previously started process, build new and start it
// TODO configure analyze strategy, parsing/pointer analyzes
func (w *Watcher) runTasks() {
	// keep track of reruns
	rerunCounter := 1
LOOP:
	for {
		select {
		case e := <-w.events:
			for _, excludePrefix := range w.excludePrefixes {
				name := path.Base(e.Name)
				// TODO move to task responsibility
				// filter all files except .go
				if e.Op&fsnotify.Write != fsnotify.Write || strings.HasPrefix(name, excludePrefix) {
					continue LOOP
				}
			}
			// TODO improve message
			log.Println("File changed:", e.Name)
			// send signal to stop previous command
			select {
			case w.stop <- true:
			default:
				// if blocking it may prev process may be dead
				go func() {
					// drain stop ch
					<-w.stop
				}()
			}
			//@TODO check for better solution without sleep, had some issues with flymake emacs go plugin
			time.Sleep(w.delay)
			// run required commands
			// TODO previously untracked files after commit should be removed from map
			for _, task := range w.tasks {
				// run all tasks
				fmt.Printf("Run task.ID %+v\n", task.ID()) // output for debug
				msg, err := task.Run(e.Name, w.stop)
				if err != nil {
					fmt.Printf("Task.ID: %s returned err %+v\n", task.ID(), err) // output for debug
				} else {
					fmt.Printf("Task.ID: %s %s\n", task.ID(), msg) // output for debug
				}
				err = w.notificator.Send(msg)
				if err != nil {
					fmt.Printf("NotificationService error  %+v\n", err)
				}
			}
			// TODO on success commit changes? or update untracked file state
			// process started incr rerun counter
			rerunCounter++

			// add loging
			w.printDebug("command executed")

		case err := <-w.errs:
			log.Println("Error:", err)
		}
	}
}

// recursively set watcher to all child directories
// and fan-in all events and errors to chan in main loop
func (w *Watcher) watchFiles() {
	// walk current directory and if there is other directory add watcher to it
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if len(path) > 1 {
				// skip hidden and vendor dirs
				if strings.HasPrefix(filepath.Base(path), ".") || strings.HasPrefix(path, "vendor") {
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

// create new cmd in standard way
func newCmd(bin string, args []string) *exec.Cmd {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd
}
