package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileStorage(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "test-new-storage")
	os.Remove(testDir)
	defer func() {
		if !t.Failed() {
			os.Remove(testDir)
		}
	}()
	db1name := filepath.Join(testDir, "db1")
	s, err := NewFileStorage(db1name)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if s == nil {
		t.Errorf("storage is nil")
	}
	db2name := filepath.Join(testDir, "db2")
	for i := 0; i < 2; i++ {
		s, err := NewFileStorage(db2name)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		}
		if s == nil {
			t.Errorf("storage is nil")
		}
	}
	dbs := listDbs()
	countdb1, countdb2 := 0, 0
	for i := range dbs {
		if dbs[i] == db1name {
			countdb1++
		} else if dbs[i] == db2name {
			countdb2++
		}
	}
	if countdb1 != 1 || countdb2 != 1 {
		t.Errorf("expected to have 1 of each db got db1 %d, db2 %d", countdb1, countdb2)
	}
}

func TestFileStorageAPI(t *testing.T) {
	testDir := filepath.Join(os.TempDir(), "test-file-storage-api")
	os.Remove(testDir)
	defer func() {
		if !t.Failed() {
			os.Remove(testDir)
		}
	}()
	db1name := filepath.Join(testDir, "db")
	storage, err := NewFileStorage(db1name)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	cases := []struct {
		name string
		data []byte
		err  error
	}{
		{"", []byte("empty key data file"), errors.New("empty key")},
		{"testfile_a", []byte("testfile_a_data"), nil},
		{"testfile_b", []byte("testfile_b_data"), nil},
	}
	t.Run("Store", func(t *testing.T) {
		for i, tc := range cases {
			err := storage.Store(tc.name, tc.data)
			if isUnexpectedErr(t, i,
				fmt.Sprintf("Write file name %q", tc.name),
				tc.err, err) {
				continue
			}
		}
	})
	t.Run("Get", func(t *testing.T) {
		for i, tc := range cases {
			data, err := storage.Get(tc.name)
			if isUnexpectedErr(t, i,
				fmt.Sprintf("Get file name %q", tc.name),
				tc.err, err) {
				continue
			}
			if err != nil {
				continue // skip expected errors
			}
			if string(tc.data) != string(data) {
				t.Errorf("case [%d] %s\nexpected data %s\ngot %s", i, fmt.Sprintf("Get file name %q", tc.name),
					string(tc.data), string(data),
				)
			}
		}
	})

}
