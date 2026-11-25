// Package historyfile 将 SQL 查询历史持久化到 history.json。
package historyfile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/queryhistory"
)

// Repository 是线程安全的本地查询历史文件仓库。
type Repository struct {
	mu sync.Mutex
	path string
}

func New(path string) (*Repository, error) {
	path = strings.TrimSpace(path)
	if path == "" { return nil, errors.New("查询历史文件路径不能为空") }
	absolute, err := filepath.Abs(path)
	if err != nil { return nil, fmt.Errorf("解析查询历史路径: %w", err) }
	return &Repository{path:absolute}, nil
}

func (r *Repository) List(ctx context.Context) ([]queryhistory.Entry, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) { return []queryhistory.Entry{}, nil }
	if err != nil { return nil, fmt.Errorf("读取查询历史: %w", err) }
	if len(data) == 0 { return []queryhistory.Entry{}, nil }
	var entries []queryhistory.Entry
	if err := json.Unmarshal(data, &entries); err != nil { return nil, fmt.Errorf("解析查询历史: %w", err) }
	if entries == nil { entries = []queryhistory.Entry{} }
	return entries, nil
}

func (r *Repository) Save(ctx context.Context, entries []queryhistory.Entry) error {
	if err := ctx.Err(); err != nil { return err }
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil { return fmt.Errorf("创建查询历史目录: %w", err) }
	if entries == nil { entries = []queryhistory.Entry{} }
	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil { return fmt.Errorf("编码查询历史: %w", err) }
	encoded = append(encoded, '\n')
	file, err := os.CreateTemp(filepath.Dir(r.path), ".history-*.tmp")
	if err != nil { return fmt.Errorf("创建查询历史临时文件: %w", err) }
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o600); err != nil { _ = file.Close(); return err }
	if _, err := file.Write(encoded); err != nil { _ = file.Close(); return err }
	if err := file.Sync(); err != nil { _ = file.Close(); return err }
	if err := file.Close(); err != nil { return err }
	if err := ctx.Err(); err != nil { return err }
	if err := replaceFile(temp, r.path); err != nil { return fmt.Errorf("替换查询历史文件: %w", err) }
	return nil
}

// RandomIDGenerator 使用 crypto/rand 生成 128 位随机历史 ID。
type RandomIDGenerator struct{}

func (RandomIDGenerator) NewID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil { return "", err }
	return hex.EncodeToString(raw[:]), nil
}

// replaceFile 在支持覆盖重命名的平台上使用原子替换；Windows 目标文件已存在时，
// 先保留备份再替换，失败则恢复旧文件，避免多次写入历史时丢失数据。
func replaceFile(temp, target string) error {
	if err := os.Rename(temp, target); err == nil { return nil }
	backup := target + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil { return err }
	if err := os.Rename(temp, target); err != nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return fmt.Errorf("替换失败: %v；恢复旧文件失败: %w", err, restoreErr)
		}
		return err
	}
	_ = os.Remove(backup)
	return nil
}
