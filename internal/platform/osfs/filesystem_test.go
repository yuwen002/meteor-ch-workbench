package osfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileSystemReadWriteAndCreate(t *testing.T) {
	fs := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := fs.WriteFile(path, []byte("初始内容"), 0o600); err != nil {
		t.Fatalf("写入文件失败：%v", err)
	}
	got, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件失败：%v", err)
	}
	if string(got) != "初始内容" {
		t.Fatalf("读取内容不匹配：得到 %q", got)
	}

	file, err := fs.Create(path)
	if err != nil {
		t.Fatalf("创建文件失败：%v", err)
	}
	if _, err := file.Write([]byte("导出内容")); err != nil {
		_ = file.Close()
		t.Fatalf("写入文件句柄失败：%v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("关闭文件失败：%v", err)
	}

	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取导出文件失败：%v", err)
	}
	if string(got) != "导出内容" {
		t.Fatalf("文件未被正确覆盖：得到 %q", got)
	}
}

func TestFileSystemReadMissingFile(t *testing.T) {
	_, err := New().ReadFile(filepath.Join(t.TempDir(), "不存在.json"))
	if err == nil {
		t.Fatal("读取不存在的文件应返回错误")
	}
}
