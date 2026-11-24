package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuwen002/meteor-ch-workbench/internal/infrastructure/storage/connectionfile"
)

// migrateLegacyConnections 识别并迁移旧版连接配置。
// Windows 上新旧目录名仅大小写不同，实际指向同一目录，不能仅凭目标文件存在就跳过。
func migrateLegacyConnections(dataDir string, legacyPath string) error {
	newPath := filepath.Join(dataDir, "connections.json")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("创建连接配置目录: %w", err)
	}

	raw, err := os.ReadFile(newPath)
	if err == nil {
		isLegacy, detectErr := isLegacyConnectionFile(raw)
		if detectErr != nil {
			return fmt.Errorf("检查现有连接配置: %w", detectErr)
		}
		if !isLegacy {
			return nil
		}
		return migrateLegacyJSON(newPath, raw)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("读取现有连接配置: %w", err)
	}

	raw, err = os.ReadFile(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取旧版连接配置: %w", err)
	}
	return migrateLegacyJSON(newPath, raw)
}

func isLegacyConnectionFile(raw []byte) (bool, error) {
	var header struct {
		Version *int `json:"version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return false, fmt.Errorf("JSON 格式无效: %w", err)
	}
	return header.Version == nil, nil
}

// migrateLegacyJSON 先在同目录临时文件中完整转换并校验，再替换目标文件。
// 转换失败不会触碰原配置。
func migrateLegacyJSON(target string, raw []byte) error {
	dir := filepath.Dir(target)
	stageDir, err := os.MkdirTemp(dir, ".connection-migration-*")
	if err != nil {
		return fmt.Errorf("创建迁移暂存目录: %w", err)
	}
	defer os.RemoveAll(stageDir)

	stagePath := filepath.Join(stageDir, "connections.json")
	repository, err := connectionfile.New(stagePath)
	if err != nil {
		return err
	}
	if _, err := repository.ImportLegacyJSON(context.Background(), raw); err != nil {
		return fmt.Errorf("迁移旧版连接配置: %w", err)
	}

	backupPath := target + ".legacy-backup"
	_ = os.Remove(backupPath)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, backupPath); err != nil {
			return fmt.Errorf("备份旧连接配置: %w", err)
		}
		hadTarget = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("检查旧连接配置: %w", err)
	}

	if err := os.Rename(stagePath, target); err != nil {
		if hadTarget {
			if restoreErr := os.Rename(backupPath, target); restoreErr != nil {
				return fmt.Errorf("写入新连接配置失败: %v；恢复旧配置也失败: %w", err, restoreErr)
			}
		}
		return fmt.Errorf("替换为新版连接配置: %w", err)
	}
	if hadTarget {
		_ = os.Remove(backupPath)
	}
	return nil
}
