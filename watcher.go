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
	"github.com/kr/pretty"
)

// TODO do not block on when run cmd exits, properly handler exits
const fileExt = "go"

// TODO add gnome notifications
// TODO test passing args to test run
// TODO define notification expire time
type Watcher struct {
	delay           time.Duration
	debug           bool
	args            string
	excludePrefixes []string
	events          chan fsnotify.Event
	errs            chan error
	stop            chan bool
}

func NewWatcher(delay int, args string, excludePrefixes []string, debug bool) Watcher {
	return Watcher{
		delay:           time.Duration(delay) * time.Millisecond,
		debug:           debug,
		args:            args,
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
	go w.startTestRunner()

	// block
	<-done
}

// main loop to listen all events from all registered directories
// and exec required commands, kill previously started process, build new and start it
// TODO configure analyze strategy, parsing/pointer analyzes
func (w *Watcher) startTestRunner() {
	// run required commands for the first time
	err := w.runTests([]string{"."})
	if err != nil {
		fmt.Println("runTests err", err)
	}
	// keep track of reruns
	rerunCounter := 1
LOOP:
	for {
		select {
		case e := <-w.events:
			for _, excludePrefix := range w.excludePrefixes {
				name := path.Base(e.Name)
				// filter all files except .go
				if e.Op&fsnotify.Write != fsnotify.Write || !strings.HasSuffix(name, "."+fileExt) || strings.HasPrefix(name, excludePrefix) {
					continue LOOP
				}
			}
			// TODO improve message
			log.Println("File changed:", e.Name)
			if strings.HasSuffix(e.Name, "_test.go") {
				fmt.Printf("parseTestFile %+v\n", e.Name) // output for debug

				err := parseTestFile(e.Name)
				if err != nil {
					fmt.Printf("parseTestFile error %+v\n", err) // output for debug
				}
			} else {
				fmt.Printf("%+v\n", "TODO? parse go file") // output for debug

			}
			changes, err := processFileChanges()
			if err != nil {
				fmt.Printf("processFileChanges error %+v\n", err) // output for debug

			} else {
				fmt.Printf("FileBlocks effected %# v\n", pretty.Formatter(changes)) // output for debug

			}

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
			tests, err := findTestsToRun(changes)
			if err != nil {
				fmt.Printf("ERROR findTestsToRun %+v\n", err) // output for debug
			}
			if len(tests) == 0 {
				fmt.Println("[INFO] No test found to run") // output for debug
				continue LOOP
			}
			time.Sleep(w.delay)
			// run required commands
			// TODO previously untracked files after commit should be removed from map
			err = w.runTests(tests)
			if err != nil {
				fmt.Println("runTests err", err)
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

// cmd sequence to run build with some name, check err and run named binary
// TODO use different strategies for testing when no test found to run or
// Type struct changed, run all methods? or just Type it self tested?
func (w *Watcher) runTests(testNames []string) (err error) {
	tests := strings.Join(testNames, "|")
	// run tests
	// do not wait process to finish
	// in case of console blocking programs
	// -vet=off to improve speed
	cmd := newCmd("go", []string{
		"test", "-v", "-vet", "off", "-run", tests, "./...", "-args", w.args,
	})
	w.printDebug(strings.Join(cmd.Args, " "))
	err = cmd.Start()
	if err != nil {
		return
	}
	// TODO notify on test fail
	go func() {
		<-w.stop
		if cmd.ProcessState.Exited() {
			// already exited
			return
		}
		// kill process if already running
		// try to kill process
		w.printDebug("process to kill pid", cmd.Process.Pid)
		err := cmd.Process.Kill()
		if err != nil {
			fmt.Println("cmd process kill returned error" + err.Error())
		}
	}()
	// TODO refactor, configure behavior of notifications
	notifyCmd := exec.Command("notify-send", "-t", "2000", "--hint", "int:transient:1")
	err = cmd.Wait()
	if err != nil {
		fmt.Println("cmd process wait returned error " + err.Error())
		// notify failed tests
		notifyCmd.Args = append(notifyCmd.Args, "Tests FAIL: "+tests)
	} else {
		// notify tests pass
		notifyCmd.Args = append(notifyCmd.Args, "Tests PASS: "+tests)
	}
	err = notifyCmd.Run()
	if err != nil {
		fmt.Printf("notify-send error %+v\n", err) // output for debug
	}

	return
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
			// TODO refactor
			// index all test files
			fmt.Printf("Path: %s fileName %s\n", path, info.Name()) // output for debug
			dir, err := os.Open(info.Name())
			if err == nil {
				fileInfos, err := dir.Readdir(-1)
				if err == nil {
					// close file
					dir.Close()
					for _, f := range fileInfos {
						if strings.HasSuffix(f.Name(), "_test.go") {
							fname := filepath.Join(path, f.Name())
							fmt.Printf("parseTestFile %+v\n", fname) // output for debug

							err = parseTestFile(fname)
							if err != nil {
								fmt.Printf("parseTestFile unexpected error %+v\n", err) // output for debug
							}
						}
					}
				} else {
					fmt.Printf("Readdir unexpected error %+v\n", err) // output for debug

				}

			} else {
				fmt.Printf("open %s unexpected error %+v\n", filepath.Join(path, info.Name()), err) // output for debug
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
	if w.debug {
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
