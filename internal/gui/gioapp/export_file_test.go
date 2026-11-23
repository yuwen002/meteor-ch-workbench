package gioapp

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

type testWriteCloser struct{}
func (testWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (testWriteCloser) Close() error { return nil }

func TestRemoveIncompleteExportRemovesLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.csv")
	if err := os.WriteFile(path, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	supported, err := removeIncompleteExport(file)
	if err != nil {
		t.Fatal(err)
	}
	if !supported {
		t.Fatal("local file cleanup should be supported")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("partial export still exists, stat error = %v", err)
	}
}

func TestRemoveIncompleteExportSkipsNonLocalHandle(t *testing.T) {
	var file io.WriteCloser = testWriteCloser{}
	supported, err := removeIncompleteExport(file)
	if err != nil {
		t.Fatal(err)
	}
	if supported {
		t.Fatal("non-local file handle must not be treated as removable")
	}
}
