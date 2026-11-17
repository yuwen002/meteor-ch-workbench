// 本文件负责把配置导出为多种格式：JSON / YAML / CSV / DSN(URI) / DBeaver 等。
// 采用注册表模式，新增格式只需在 exportFormats 中追加一项，前端通过 ExportFormats 自动感知。
package config

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ExportFormat 描述一种导出格式的元信息，供前端下拉/单选展示。
type ExportFormat struct {
	ID          string `json:"id"`          // 格式标识，前端调用 ExportConfig 时传入
	Name        string `json:"name"`        // 展示名称
	Extension   string `json:"extension"`   // 建议文件扩展名（不含点）
	Mime        string `json:"mime"`        // MIME 类型
	Description string `json:"description"` // 格式说明
}

// ExportFormats 返回当前支持的全部导出格式，供前端动态渲染格式选择。
func ExportFormats() []ExportFormat {
	formats := make([]ExportFormat, 0, len(exportRegistry))
	for _, f := range exportRegistry {
		formats = append(formats, f.Meta)
	}
	return formats
}

// exportRegistry 按注册顺序保存所有导出格式实现。
var exportRegistry = []struct {
	Meta   ExportFormat
	Render func(DataFile, bool) (string, error)
}{
	{
		Meta: ExportFormat{
			ID: "json", Name: "JSON", Extension: "json", Mime: "application/json",
			Description: "完整配置结构，用于本工具备份与还原",
		},
		Render: renderJSON,
	},
	{
		Meta: ExportFormat{
			ID: "yaml", Name: "YAML", Extension: "yaml", Mime: "text/yaml",
			Description: "结构清晰、便于阅读与版本管理",
		},
		Render: renderYAML,
	},
	{
		Meta: ExportFormat{
			ID: "csv", Name: "CSV", Extension: "csv", Mime: "text/csv",
			Description: "扁平表格，可直接用 Excel 打开",
		},
		Render: renderCSV,
	},
	{
		Meta: ExportFormat{
			ID: "dsn", Name: "DSN / URI", Extension: "txt", Mime: "text/plain",
			Description: "每条连接一行 clickhouse:// 地址，可直接用于命令行或第三方工具",
		},
		Render: renderDSN,
	},
	{
		Meta: ExportFormat{
			ID: "dbeaver", Name: "DBeaver", Extension: "json", Mime: "application/json",
			Description: "DBeaver 数据源配置文件，便于从本工具迁出",
		},
		Render: renderDBeaver,
	},
}

// ExportByFormat 按指定格式与是否包含密码导出配置文本。
func (m *Manager) ExportByFormat(format string, includePassword bool) (string, string, error) {
	if _, err := m.LoadAll(); err != nil {
		return "", "", err
	}
	m.mu.RLock()
	data := m.data
	m.mu.RUnlock()
	for _, f := range exportRegistry {
		if f.Meta.ID == format {
			out, err := f.Render(data, includePassword)
			if err != nil {
				return "", "", err
			}
			return out, f.Meta.Extension, nil
		}
	}
	return "", "", errors.New("unsupported format: " + format)
}

// 复制一份数据，按 includePassword 决定是否清空密码，避免修改内存中的真实配置。
func sanitize(data DataFile, includePassword bool) DataFile {
	conns := make([]Connection, len(data.Connections))
	copy(conns, data.Connections)
	if !includePassword {
		for i := range conns {
			conns[i].Password = ""
		}
	}
	folders := make([]Folder, len(data.Folders))
	copy(folders, data.Folders)
	return DataFile{Folders: folders, Connections: conns}
}

// folderNameByID 构建文件夹 ID 到名称的映射，便于在表格/注释中展示分组名。
func folderNameByID(data DataFile) map[string]string {
	m := make(map[string]string, len(data.Folders))
	for _, f := range data.Folders {
		m[f.ID] = f.Name
	}
	return m
}

func renderJSON(data DataFile, includePassword bool) (string, error) {
	d := sanitize(data, includePassword)
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// renderYAML 手工生成 YAML，避免引入第三方依赖；仅覆盖本项目的简单数据结构。
func renderYAML(data DataFile, includePassword bool) (string, error) {
	d := sanitize(data, includePassword)
	var b strings.Builder
	b.WriteString("# Meteor CH Workbench configuration\n")
	b.WriteString("folders:\n")
	for _, f := range d.Folders {
		b.WriteString("  - id: " + yamlQuote(f.ID) + "\n")
		b.WriteString("    name: " + yamlQuote(f.Name) + "\n")
	}
	if len(d.Folders) == 0 {
		b.WriteString("  []\n")
	}
	b.WriteString("connections:\n")
	if len(d.Connections) == 0 {
		b.WriteString("  []\n")
	}
	for _, c := range d.Connections {
		b.WriteString("  - id: " + yamlQuote(c.ID) + "\n")
		b.WriteString("    folder_id: " + yamlQuote(c.FolderID) + "\n")
		b.WriteString("    name: " + yamlQuote(c.Name) + "\n")
		b.WriteString("    host: " + yamlQuote(c.Host) + "\n")
		b.WriteString(fmt.Sprintf("    port: %d\n", c.Port))
		b.WriteString("    protocol: " + yamlQuote(c.Protocol) + "\n")
		b.WriteString("    user: " + yamlQuote(c.User) + "\n")
		b.WriteString("    password: " + yamlQuote(c.Password) + "\n")
		b.WriteString("    database: " + yamlQuote(c.Database) + "\n")
		b.WriteString(fmt.Sprintf("    secure: %t\n", c.Secure))
		b.WriteString(fmt.Sprintf("    use_compress: %t\n", c.UseCompress))
	}
	return b.String(), nil
}

// yamlQuote 对字符串做最小化 YAML 引号处理：含特殊字符时使用双引号并转义。
func yamlQuote(s string) string {
	if s == "" {
		return "\"\""
	}
	if !strings.ContainsAny(s, ":#{}[],&*!|>'\"%@`\n\t ") {
		return s
	}
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return "\"" + s + "\""
}

func renderCSV(data DataFile, includePassword bool) (string, error) {
	d := sanitize(data, includePassword)
	names := folderNameByID(d)
	var b strings.Builder
	w := csv.NewWriter(&b)
	header := []string{
		"folder", "name", "protocol", "host", "port", "user", "password",
		"database", "secure", "use_compress",
	}
	if err := w.Write(header); err != nil {
		return "", err
	}
	for _, c := range d.Connections {
		row := []string{
			names[c.FolderID],
			c.Name,
			c.Protocol,
			c.Host,
			fmt.Sprintf("%d", c.Port),
			c.User,
			c.Password,
			c.Database,
			fmt.Sprintf("%t", c.Secure),
			fmt.Sprintf("%t", c.UseCompress),
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	w.Flush()
	return b.String(), w.Error()
}

// renderDSN 生成每行一条 clickhouse:// URI。密码按需保留，做 URL 转义。
func renderDSN(data DataFile, includePassword bool) (string, error) {
	d := sanitize(data, includePassword)
	var b strings.Builder
	for _, c := range d.Connections {
		scheme := "clickhouse"
		if c.Secure {
			scheme = "clickhouses"
		}
		userInfo := ""
		if c.User != "" || c.Password != "" {
			userInfo = url.UserPassword(c.User, c.Password).String()
			if c.Password == "" {
				userInfo = url.User(c.User).String()
			}
			userInfo += "@"
		}
		dbPath := ""
		if c.Database != "" {
			dbPath = "/" + url.PathEscape(c.Database)
		}
		query := []string{}
		if c.UseCompress {
			query = append(query, "compress=true")
		}
		if c.Protocol == "native" {
			query = append(query, "protocol=native")
		}
		qs := ""
		if len(query) > 0 {
			qs = "?" + strings.Join(query, "&")
		}
		b.WriteString(fmt.Sprintf("%s://%s%s:%d%s%s\n", scheme, userInfo, c.Host, c.Port, dbPath, qs))
	}
	return b.String(), nil
}

// renderDBeaver 生成 DBeaver 可识别的数据源配置（JSON）。密码按需以明文写入。
func renderDBeaver(data DataFile, includePassword bool) (string, error) {
	d := sanitize(data, includePassword)
	names := folderNameByID(d)
	type dbeaverConn struct {
		Provider   string            `json:"provider"`
		Driver     string            `json:"driver"`
		Name       string            `json:"name"`
		Folder     string            `json:"folder,omitempty"`
		URL        string            `json:"url"`
		Host       string            `json:"host"`
		Port       string            `json:"port"`
		Database   string            `json:"database"`
		User       string            `json:"user"`
		Password   string            `json:"password"`
		Properties map[string]string `json:"properties"`
	}
	conns := make([]dbeaverConn, 0, len(d.Connections))
	for _, c := range d.Connections {
		port := 8123
		if c.Protocol == "native" {
			port = 9000
		}
		if c.Port != 0 {
			port = c.Port
		}
		jdbc := fmt.Sprintf("jdbc:clickhouse://%s:%d/%s", c.Host, port, c.Database)
		props := map[string]string{}
		if c.UseCompress {
			props["compress"] = "1"
		}
		if c.Secure {
			props["ssl"] = "true"
		}
		conns = append(conns, dbeaverConn{
			Provider:   "clickhouse",
			Driver:     "clickhouse",
			Name:       c.Name,
			Folder:     names[c.FolderID],
			URL:        jdbc,
			Host:       c.Host,
			Port:       fmt.Sprintf("%d", port),
			Database:   c.Database,
			User:       c.User,
			Password:   c.Password,
			Properties: props,
		})
	}
	root := map[string]any{
		"version":     "1.0",
		"connections": conns,
		"folders":     d.Folders,
	}
	b, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
