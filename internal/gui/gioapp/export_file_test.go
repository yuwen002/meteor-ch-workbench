package gioapp

import (
	"bytes"
	"errors"
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


type shortFailWriter struct {
	written int
}

func (w *shortFailWriter) Write(p []byte) (int, error) {
	if w.written > 0 {
		return 0, errors.New("write failed")
	}
	n := len(p) / 2
	w.written += n
	return n, nil
}

func TestCountingWriterCountsWrittenBytes(t *testing.T) {
	var output bytes.Buffer
	writer := &countingWriter{writer: &output}
	payload := []byte("ClickHouse 导出")
	n, err := writer.Write(payload)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(payload) {
		t.Fatalf("written = %d, want %d", n, len(payload))
	}
	if writer.bytes != int64(len(payload)) {
		t.Fatalf("bytes = %d, want %d", writer.bytes, len(payload))
	}
}

func TestCountingWriterCountsPartialWriteOnError(t *testing.T) {
	underlying := &partialErrorWriter{}
	writer := &countingWriter{writer: underlying}
	_, err := writer.Write([]byte("123456"))
	if err == nil {
		t.Fatal("expected underlying write error")
	}
	if writer.bytes != 3 {
		t.Fatalf("bytes = %d, want 3", writer.bytes)
	}
}

type partialErrorWriter struct{}

func (*partialErrorWriter) Write(p []byte) (int, error) {
	return len(p) / 2, errors.New("partial write")
}
