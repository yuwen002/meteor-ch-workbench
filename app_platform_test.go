package main

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/platform"
)

type fakePlatform struct {
	openPath string
	openErr  error
	savePath string
	saveErr  error
}

func (f *fakePlatform) SaveFile(platform.SaveFileRequest) (string, error) {
	return f.savePath, f.saveErr
}

func (f *fakePlatform) OpenFile(platform.OpenFileRequest) (string, error) {
	return f.openPath, f.openErr
}

func (f *fakePlatform) OpenDirectory(string) error { return nil }

type fakeWriteCloser struct {
	strings.Builder
	writeErr error
	closeErr error
	closed   bool
}

func (f *fakeWriteCloser) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return f.Builder.Write(p)
}

func (f *fakeWriteCloser) Close() error {
	f.closed = true
	return f.closeErr
}

type fakeFileSystem struct {
	readContent []byte
	readErr     error
	writeErr    error
	createErr   error
	file        *fakeWriteCloser
	readCalls   int
	writeCalls  int
	createCalls int
}

func (f *fakeFileSystem) ReadFile(string) ([]byte, error) {
	f.readCalls++
	return f.readContent, f.readErr
}

func (f *fakeFileSystem) WriteFile(string, []byte, io/fs.FileMode) error {
	f.writeCalls++
	return f.writeErr
}

func (f *fakeFileSystem) Create(string) (io.WriteCloser, error) {
	f.createCalls++
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.file == nil {
		f.file = &fakeWriteCloser{}
	}
	return f.file, nil
}

func newPlatformTestApp(p *fakePlatform, fs *fakeFileSystem) *App {
	return NewAppWithDependencies(p, fs)
}

func TestPickImportFileCancelDoesNotReadFile(t *testing.T) {
	fs := &fakeFileSystem{}
	app := newPlatformTestApp(&fakePlatform{}, fs)

	content, name, err := app.PickImportFile()
	if err != nil {
		t.Fatalf("取消选择不应返回错误：%v", err)
	}
	if content != "" || name != "" {
		t.Fatalf("取消选择应返回空内容和文件名，得到 %q、%q", content, name)
	}
	if fs.readCalls != 0 {
		t.Fatalf("取消选择不应读取文件，实际读取 %d 次", fs.readCalls)
	}
}

func TestPickImportFileReadsAndExtractsWindowsBaseName(t *testing.T) {
	fs := &fakeFileSystem{readContent: []byte("配置内容")}
	p := &fakePlatform{openPath: `C:\Users\tester\config.json`}
	app := newPlatformTestApp(p, fs)

	content, name, err := app.PickImportFile()
	if err != nil {
		t.Fatalf("选择文件失败：%v", err)
	}
	if content != "配置内容" || name != "config.json" {
		t.Fatalf("返回值不正确：content=%q name=%q", content, name)
	}
}

func TestPickImportFilePropagatesReadError(t *testing.T) {
	wantErr := errors.New("读取失败")
	fs := &fakeFileSystem{readErr: wantErr}
	p := &fakePlatform{openPath: "/tmp/config.json"}
	app := newPlatformTestApp(p, fs)

	_, _, err := app.PickImportFile()
	if !errors.Is(err, wantErr) {
		t.Fatalf("应返回文件读取错误 %v，实际为 %v", wantErr, err)
	}
}

func TestPickImportFilePropagatesDialogError(t *testing.T) {
	wantErr := errors.New("对话框失败")
	p := &fakePlatform{openErr: wantErr}
	app := newPlatformTestApp(p, &fakeFileSystem{})

	_, _, err := app.PickImportFile()
	if !errors.Is(err, wantErr) {
		t.Fatalf("应返回对话框错误 %v，实际为 %v", wantErr, err)
	}
}

func TestSaveQueryResultAsWritesAndClosesFile(t *testing.T) {
	fs := &fakeFileSystem{file: &fakeWriteCloser{}}
	p := &fakePlatform{savePath: "/tmp/result.csv"}
	app := newPlatformTestApp(p, fs)

	path, err := app.SaveQueryResultAs("csv", []string{"name"}, [][]interface{}{{"meteor"}}, "")
	if err != nil {
		t.Fatalf("导出结果失败：%v", err)
	}
	if path != p.savePath {
		t.Fatalf("保存路径不正确：得到 %q", path)
	}
	if fs.createCalls != 1 || !fs.file.closed {
		t.Fatalf("应创建并关闭文件：createCalls=%d closed=%v", fs.createCalls, fs.file.closed)
	}
	if got := fs.file.String(); !strings.Contains(got, "meteor") {
		t.Fatalf("导出内容缺少结果值：%q", got)
	}
}

func TestSaveQueryResultAsPropagatesCreateWriteAndCloseErrors(t *testing.T) {
	createErr := errors.New("创建失败")
	app := newPlatformTestApp(&fakePlatform{savePath: "/tmp/result.csv"}, &fakeFileSystem{createErr: createErr})
	if _, err := app.SaveQueryResultAs("csv", []string{"name"}, nil, ""); !errors.Is(err, createErr) {
		t.Fatalf("应返回创建错误 %v，实际为 %v", createErr, err)
	}

	writeErr := errors.New("写入失败")
	writeFS := &fakeFileSystem{file: &fakeWriteCloser{writeErr: writeErr}}
	app = newPlatformTestApp(&fakePlatform{savePath: "/tmp/result.csv"}, writeFS)
	if _, err := app.SaveQueryResultAs("csv", []string{"name"}, nil, ""); !errors.Is(err, writeErr) {
		t.Fatalf("应返回写入错误 %v，实际为 %v", writeErr, err)
	}
	if !writeFS.file.closed {
		t.Fatal("写入失败后仍应关闭文件")
	}

	closeErr := errors.New("关闭失败")
	closeFS := &fakeFileSystem{file: &fakeWriteCloser{closeErr: closeErr}}
	app = newPlatformTestApp(&fakePlatform{savePath: "/tmp/result.csv"}, closeFS)
	if _, err := app.SaveQueryResultAs("csv", []string{"name"}, nil, ""); !errors.Is(err, closeErr) {
		t.Fatalf("应返回关闭错误 %v，实际为 %v", closeErr, err)
	}
}

func TestSaveQueryResultAsCancelDoesNotCreateFile(t *testing.T) {
	fs := &fakeFileSystem{}
	app := newPlatformTestApp(&fakePlatform{}, fs)

	path, err := app.SaveQueryResultAs("csv", []string{"name"}, nil, "")
	if err != nil {
		t.Fatalf("取消保存不应返回错误：%v", err)
	}
	if path != "" || fs.createCalls != 0 {
		t.Fatalf("取消保存应返回空路径且不创建文件：path=%q createCalls=%d", path, fs.createCalls)
	}
}
