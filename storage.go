package main

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

// one filestorage by path
var dbs sync.Map

func listDbs() []string {
	var list []string
	dbs.Range(func(k, v interface{}) bool {
		list = append(list, k.(string))
		return true
	})
	return list
}

// FileStorage persistent storage using files
type FileStorage struct {
	mu   sync.Mutex
	path string
}

// NewStorage returns existing or new filestorage
func NewFileStorage(path string) (*FileStorage, error) {
	s, ok := dbs.Load(path)
	if ok {
		return s.(*FileStorage), nil
	}
	_, err := os.Stat(path)
	if err == nil { // exists
		ns := &FileStorage{path: path}
		s, ok = dbs.LoadOrStore(path, ns)
		if ok {
			return s.(*FileStorage), nil
		}
		return ns, nil
	}
	// init
	err = os.MkdirAll(path, 0700)
	if err != nil {
		return nil, err
	}
	ns := &FileStorage{path: path}
	s, ok = dbs.LoadOrStore(path, ns)
	if ok {
		return s.(*FileStorage), nil
	}
	return ns, nil
}

func (fs *FileStorage) Get(key string) ([]byte, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return ioutil.ReadFile(filepath.Join(fs.path, key))
}

func (fs *FileStorage) Store(key string, data []byte) error {
	if len(key) == 0 {
		return errors.New("empty key")
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return ioutil.WriteFile(filepath.Join(fs.path, key), data, 0600)
}
