// Package settingsfile 将工作台偏好安全地持久化到 settings.json。
package settingsfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/settings"
)

type Repository struct { mu sync.Mutex; path string }

func New(path string) (*Repository, error) {
	path = strings.TrimSpace(path)
	if path == "" { return nil, errors.New("偏好设置文件路径不能为空") }
	absolute, err := filepath.Abs(path)
	if err != nil { return nil, fmt.Errorf("解析偏好设置路径: %w", err) }
	return &Repository{path: absolute}, nil
}

func (r *Repository) Load(ctx context.Context) (domain.Preferences, error) {
	if err := ctx.Err(); err != nil { return domain.Preferences{}, err }
	r.mu.Lock(); defer r.mu.Unlock()
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) { return domain.Default(), nil }
	if err != nil { return domain.Preferences{}, fmt.Errorf("读取偏好设置: %w", err) }
	if len(data) == 0 { return domain.Default(), nil }
	var value domain.Preferences
	if err := json.Unmarshal(data, &value); err != nil { return domain.Preferences{}, fmt.Errorf("解析偏好设置: %w", err) }
	return domain.Normalize(value), nil
}

func (r *Repository) Save(ctx context.Context, value domain.Preferences) error {
	if err := ctx.Err(); err != nil { return err }
	r.mu.Lock(); defer r.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil { return fmt.Errorf("创建偏好设置目录: %w", err) }
	data, err := json.MarshalIndent(domain.Normalize(value), "", "  ")
	if err != nil { return fmt.Errorf("编码偏好设置: %w", err) }
	data = append(data, '\n')
	file, err := os.CreateTemp(filepath.Dir(r.path), ".settings-*.tmp")
	if err != nil { return fmt.Errorf("创建偏好设置临时文件: %w", err) }
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o600); err != nil { _ = file.Close(); return err }
	if _, err := file.Write(data); err != nil { _ = file.Close(); return err }
	if err := file.Sync(); err != nil { _ = file.Close(); return err }
	if err := file.Close(); err != nil { return err }
	if err := ctx.Err(); err != nil { return err }
	if err := os.Rename(temp, r.path); err == nil { return nil }
	backup := r.path + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(r.path, backup); err != nil { return fmt.Errorf("替换偏好设置文件: %w", err) }
	if err := os.Rename(temp, r.path); err != nil {
		if restoreErr := os.Rename(backup, r.path); restoreErr != nil { return fmt.Errorf("替换失败: %v；恢复旧设置失败: %w", err, restoreErr) }
		return fmt.Errorf("替换偏好设置文件: %w", err)
	}
	_ = os.Remove(backup)
	return nil
}
