package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuwen002/meteor-ch-workbench/internal/infrastructure/storage/connectionfile"
)

// migrateLegacyConnections 在新配置文件尚不存在时，导入旧版 meteor-ch-workbench 配置。
// 旧文件只读不删；新文件已存在时绝不覆盖，避免覆盖用户在新版本中创建的连接。
func migrateLegacyConnections(dataDir string, legacyPath string) error {
	newPath := filepath.Join(dataDir, "connections.json")
	if _, err := os.Stat(newPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查新连接配置: %w", err)
	}

	raw, err := os.ReadFile(legacyPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取旧版连接配置: %w", err)
	}

	repository, err := connectionfile.New(newPath)
	if err != nil {
		return err
	}
	if _, err := repository.ImportLegacyJSON(context.Background(), raw); err != nil {
		return fmt.Errorf("迁移旧版连接配置: %w", err)
	}
	return nil
}

