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
// 如果新版文件已存在但为空，会尝试从历史配置路径恢复数据；非空新版配置始终优先。
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
		if isLegacy {
			return migrateLegacyJSON(newPath, raw)
		}
		empty, emptyErr := isEmptyVersionedConnectionFile(raw)
		if emptyErr != nil {
			return fmt.Errorf("检查现有连接配置数据: %w", emptyErr)
		}
		if !empty {
			return nil
		}
		// 新版空文件可能是在迁移前首次启动时创建的。此时不能直接跳过历史文件。
		return mergeLegacyConnections(newPath, legacyPath)
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

func isEmptyVersionedConnectionFile(raw []byte) (bool, error) {
	var data struct {
		Version     int               `json:"version"`
		Folders     []json.RawMessage `json:"folders"`
		Connections []json.RawMessage `json:"connections"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return false, err
	}
	if data.Version != 1 {
		return false, fmt.Errorf("不支持版本 %d", data.Version)
	}
	return len(data.Folders) == 0 && len(data.Connections) == 0, nil
}

// mergeLegacyConnections 仅在目标配置为空时导入历史文件，不覆盖已有数据。
// ImportLegacyJSON 按 ID 合并，并保留旧文件作为恢复副本。
func mergeLegacyConnections(target, legacyPath string) error {
	raw, err := os.ReadFile(legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取历史连接配置: %w", err)
	}
	// 历史目录中的文件可能已经是版本化格式（version=1），不能只接受最早期
	// 没有 version 字段的 JSON。ImportLegacyJSON 能按 ID 合并两种格式，
	// 并且不会覆盖目标文件中已有的连接和分组。
	var header struct {
		Version *int `json:"version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return fmt.Errorf("检查历史连接配置: %w", err)
	}
	if header.Version != nil && *header.Version != 1 {
		return fmt.Errorf("历史连接配置版本不受支持: %d", *header.Version)
	}
	repository, err := connectionfile.New(target)
	if err != nil {
		return err
	}
	if _, err := repository.ImportLegacyJSON(context.Background(), raw); err != nil {
		return fmt.Errorf("恢复历史连接配置: %w", err)
	}
	return nil
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
