// 本文件负责把多种格式的配置文本解析为统一的 DataFile，并支持覆盖/合并两种导入模式。
// 与 export.go 的注册表对称：新增可导入格式只需在 importRegistry 追加一项。
package config

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ImportFormat 描述一种可导入格式的元信息，供前端展示与自动识别。
type ImportFormat struct {
	ID          string `json:"id"`          // 格式标识
	Name        string `json:"name"`        // 展示名称
	Extension   string `json:"extension"`   // 关联文件扩展名
	Description string `json:"description"` // 格式说明
}

// ImportOptions 控制导入行为。
type ImportOptions struct {
	Format string `json:"format"` // 指定格式；为空时自动识别
	Mode   string `json:"mode"`   // "replace" 覆盖或 "merge" 合并
}

// ImportResult 返回导入结果概要，供前端提示用户。
type ImportResult struct {
	Mode            string `json:"mode"`            // 实际使用的模式
	Format          string `json:"format"`          // 实际识别/使用的格式
	ImportedFolders int    `json:"importedFolders"` // 导入的文件夹数量
	ImportedConns   int    `json:"importedConns"`   // 导入的连接数量
	TotalFolders    int    `json:"totalFolders"`    // 导入后的文件夹总数
	TotalConns      int    `json:"totalConns"`      // 导入后的连接总数
}

// ImportFormats 返回支持的全部导入格式。
func ImportFormats() []ImportFormat {
	formats := make([]ImportFormat, 0, len(importRegistry))
	for _, f := range importRegistry {
		formats = append(formats, f.Meta)
	}
	return formats
}

// importRegistry 按顺序保存各格式解析实现，顺序也是自动识别的尝试顺序。
var importRegistry = []struct {
	Meta  ImportFormat
	Parse func(string) (DataFile, error)
}{
	{
		Meta: ImportFormat{
			ID: "json", Name: "JSON", Extension: "json",
			Description: "本工具或 DBeaver 导出的 JSON",
		},
		Parse: parseJSONOrDBeaver,
	},
	{
		Meta: ImportFormat{
			ID: "yaml", Name: "YAML", Extension: "yaml",
			Description: "本工具导出的 YAML",
		},
		Parse: parseYAML,
	},
	{
		Meta: ImportFormat{
			ID: "csv", Name: "CSV", Extension: "csv",
			Description: "含表头的扁平表格（Excel 可另存为 CSV）",
		},
		Parse: parseCSV,
	},
	{
		Meta: ImportFormat{
			ID: "dsn", Name: "DSN / URI", Extension: "txt",
			Description: "每行一个 clickhouse:// 地址",
		},
		Parse: parseDSN,
	},
}

// ImportDataWithOptions 按指定格式与模式导入配置文本。
// format 为空时自动识别：逐个尝试注册的解析器，首个成功者即为结果。
func (m *Manager) ImportDataWithOptions(raw string, opts ImportOptions) (ImportResult, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ImportResult{}, errors.New("empty config content")
	}

	mode := opts.Mode
	if mode != "merge" {
		mode = "replace"
	}

	parsed, format, err := parseByFormat(raw, opts.Format)
	if err != nil {
		return ImportResult{}, err
	}
	normalizeData(&parsed)

	m.mu.Lock()
	defer m.mu.Unlock()

	var out DataFile
	switch mode {
	case "merge":
		out = mergeData(m.data, parsed)
	default:
		out = parsed
	}
	m.data = out
	if err := m.flush(); err != nil {
		return ImportResult{}, err
	}

	return ImportResult{
		Mode:            mode,
		Format:          format,
		ImportedFolders: len(parsed.Folders),
		ImportedConns:   len(parsed.Connections),
		TotalFolders:    len(out.Folders),
		TotalConns:      len(out.Connections),
	}, nil
}

// parseByFormat 按指定格式解析；format 为空则自动识别。
func parseByFormat(raw, format string) (DataFile, string, error) {
	if format != "" {
		for _, f := range importRegistry {
			if f.Meta.ID == format {
				df, err := f.Parse(raw)
				if err != nil {
					return DataFile{}, "", err
				}
				return df, f.Meta.ID, nil
			}
		}
		return DataFile{}, "", errors.New("unsupported import format: " + format)
	}
	var lastErr error
	for _, f := range importRegistry {
		df, err := f.Parse(raw)
		if err == nil && (len(df.Connections) > 0 || len(df.Folders) > 0) {
			return df, f.Meta.ID, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no recognizable data found")
	}
	return DataFile{}, "", lastErr
}

// normalizeData 补齐缺失 ID、归一化协议，并把悬空的 folder_id 置空。
func normalizeData(df *DataFile) {
	if df.Folders == nil {
		df.Folders = []Folder{}
	}
	if df.Connections == nil {
		df.Connections = []Connection{}
	}
	for i := range df.Folders {
		if df.Folders[i].ID == "" {
			df.Folders[i].GenID()
		}
	}
	valid := make(map[string]bool, len(df.Folders))
	for _, f := range df.Folders {
		valid[f.ID] = true
	}
	for i := range df.Connections {
		if df.Connections[i].ID == "" {
			df.Connections[i].GenID()
		}
		df.Connections[i].Normalize()
		if df.Connections[i].FolderID != "" && !valid[df.Connections[i].FolderID] {
			df.Connections[i].FolderID = ""
		}
	}
}

// mergeData 合并策略：同名文件夹复用，ID 冲突时重新生成；连接追加并重新生成 ID。
func mergeData(cur, incoming DataFile) DataFile {
	out := DataFile{
		Folders:     append([]Folder{}, cur.Folders...),
		Connections: append([]Connection{}, cur.Connections...),
	}
	folderIDs := make(map[string]bool, len(out.Folders))
	byName := make(map[string]string, len(out.Folders))
	for _, f := range out.Folders {
		folderIDs[f.ID] = true
		byName[f.Name] = f.ID
	}
	connIDs := make(map[string]bool, len(out.Connections))
	for _, c := range out.Connections {
		connIDs[c.ID] = true
	}
	remap := make(map[string]string)
	for _, f := range incoming.Folders {
		if id, ok := byName[f.Name]; ok {
			remap[f.ID] = id
			continue
		}
		if f.ID == "" || folderIDs[f.ID] {
			f.GenID()
		}
		folderIDs[f.ID] = true
		byName[f.Name] = f.ID
		remap[f.ID] = f.ID
		out.Folders = append(out.Folders, f)
	}
	for _, c := range incoming.Connections {
		if c.ID == "" || connIDs[c.ID] {
			c.GenID()
		}
		connIDs[c.ID] = true
		if mapped, ok := remap[c.FolderID]; ok {
			c.FolderID = mapped
		}
		out.Connections = append(out.Connections, c)
	}
	return out
}

// ---- 各格式解析实现 ----

// parseJSONOrDBeaver 兼容两种 JSON：本工具结构，或 DBeaver 的 connections 数组。
func parseJSONOrDBeaver(raw string) (DataFile, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return DataFile{}, errors.New("invalid json: " + err.Error())
	}

	if _, ok := probe["connections"]; !ok {
		return DataFile{}, errors.New("json has no connections field")
	}

	// 先按本工具结构解析
	var df DataFile
	if err := json.Unmarshal([]byte(raw), &df); err == nil {
		if len(df.Connections) > 0 && df.Connections[0].Host != "" {
			return df, nil
		}
	}

	// 再尝试 DBeaver 结构
	type dbeaverConn struct {
		Provider   string            `json:"provider"`
		Driver     string            `json:"driver"`
		Name       string            `json:"name"`
		Folder     string            `json:"folder"`
		URL        string            `json:"url"`
		Host       string            `json:"host"`
		Port       string            `json:"port"`
		Database   string            `json:"database"`
		User       string            `json:"user"`
		Password   string            `json:"password"`
		Properties map[string]string `json:"properties"`
	}
	var dv struct {
		Connections []dbeaverConn `json:"connections"`
	}
	if err := json.Unmarshal([]byte(raw), &dv); err != nil {
		return DataFile{}, errors.New("invalid json: " + err.Error())
	}
	if len(dv.Connections) == 0 {
		return DataFile{}, errors.New("json has no usable connections")
	}

	folderByName := map[string]*Folder{}
	out := DataFile{}
	for _, dc := range dv.Connections {
		host := dc.Host
		port := 0
		db := dc.Database
		if dc.URL != "" {
			if u, err := url.Parse(dc.URL); err == nil {
				if host == "" {
					host = u.Hostname()
				}
				if port == 0 {
					if p, err := strconv.Atoi(u.Port()); err == nil {
						port = p
					}
				}
				if db == "" {
					db = strings.Trim(u.Path, "/")
				}
			}
		}
		if host == "" {
			continue
		}
		if port == 0 {
			if p, err := strconv.Atoi(dc.Port); err == nil {
				port = p
			}
		}
		folderID := ""
		if dc.Folder != "" {
			f, ok := folderByName[dc.Folder]
			if !ok {
				f = &Folder{Name: dc.Folder}
				f.GenID()
				folderByName[dc.Folder] = f
				out.Folders = append(out.Folders, *f)
			}
			folderID = f.ID
		}
		name := dc.Name
		if name == "" {
			name = host
		}
		c := Connection{
			Name:     name,
			Host:     host,
			Port:     port,
			User:     dc.User,
			Password: dc.Password,
			Database: db,
			FolderID: folderID,
		}
		if dc.Properties["compress"] != "" {
			c.UseCompress = true
		}
		if dc.Properties["ssl"] != "" {
			c.Secure = true
		}
		c.Normalize()
		out.Connections = append(out.Connections, c)
	}
	if len(out.Connections) == 0 {
		return DataFile{}, errors.New("no usable dbeaver connections")
	}
	return out, nil
}

// parseYAML 解析本工具导出的简化 YAML（folders 与 connections 两个列表）。
func parseYAML(raw string) (DataFile, error) {
	lines := strings.Split(raw, "\n")
	var df DataFile
	section := ""
	var curFolder *Folder
	var curConn *Connection

	flushConn := func() {
		if curConn != nil {
			df.Connections = append(df.Connections, *curConn)
			curConn = nil
		}
	}
	flushFolder := func() {
		if curFolder != nil {
			df.Folders = append(df.Folders, *curFolder)
			curFolder = nil
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		s := strings.TrimSpace(trimmed)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, " ") && strings.HasSuffix(s, ":") {
			flushConn()
			flushFolder()
			section = strings.TrimSuffix(s, ":")
			continue
		}
		body := strings.TrimSpace(trimmed)
		if strings.HasPrefix(body, "- ") || body == "-" {
			body = strings.TrimPrefix(body, "- ")
			flushConn()
			flushFolder()
			if section == "folders" {
				curFolder = &Folder{}
			} else if section == "connections" {
				curConn = &Connection{}
			}
		}
		kv := body
		idx := strings.Index(kv, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(kv[:idx])
		val := parseYAMLScalar(strings.TrimSpace(kv[idx+1:]))

		switch section {
		case "folders":
			if curFolder == nil {
				curFolder = &Folder{}
			}
			if key == "id" {
				curFolder.ID = val
			} else if key == "name" {
				curFolder.Name = val
			}
		case "connections":
			if curConn == nil {
				curConn = &Connection{}
			}
			applyConnField(curConn, key, val)
		}
	}
	flushConn()
	flushFolder()

	if section == "" {
		return DataFile{}, errors.New("not a yaml config")
	}
	if len(df.Connections) == 0 && len(df.Folders) == 0 {
		return DataFile{}, errors.New("yaml has no data")
	}
	return df, nil
}

// parseYAMLScalar 去除 YAML 标量的引号并还原转义。
func parseYAMLScalar(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, `\"`, `"`)
		s = strings.ReplaceAll(s, `\\`, `\`)
		s = strings.ReplaceAll(s, `\n`, "\n")
		return s
	}
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return s
}

// parseCSV 解析带表头的 CSV，表头名兼容导出时使用的字段名。
func parseCSV(raw string) (DataFile, error) {
	r := csv.NewReader(strings.NewReader(raw))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return DataFile{}, errors.New("invalid csv: " + err.Error())
	}
	if len(records) < 1 {
		return DataFile{}, errors.New("empty csv")
	}
	header := records[0]
	col := make(map[string]int, len(header))
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	get := func(row []string, key string) string {
		i, ok := col[key]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	required := []string{"name", "host", "port"}
	for _, k := range required {
		if _, ok := col[k]; !ok {
			return DataFile{}, fmt.Errorf("csv missing required column: %s", k)
		}
	}

	folderByName := map[string]*Folder{}
	var df DataFile
	for _, row := range records[1:] {
		if len(row) == 0 || strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		host := get(row, "host")
		if host == "" {
			continue
		}
		folderName := get(row, "folder")
		folderID := ""
		if folderName != "" {
			f, ok := folderByName[folderName]
			if !ok {
				f = &Folder{Name: folderName}
				f.GenID()
				folderByName[folderName] = f
				df.Folders = append(df.Folders, *f)
			}
			folderID = f.ID
		}
		port, _ := strconv.Atoi(get(row, "port"))
		c := Connection{
			Name:        get(row, "name"),
			Host:        host,
			Port:        port,
			Protocol:    get(row, "protocol"),
			User:        get(row, "user"),
			Password:    get(row, "password"),
			Database:    get(row, "database"),
			Secure:      parseBool(get(row, "secure")),
			UseCompress: parseBool(get(row, "use_compress")),
			FolderID:    folderID,
		}
		if c.Name == "" {
			c.Name = host
		}
		c.Normalize()
		df.Connections = append(df.Connections, c)
	}
	if len(df.Connections) == 0 {
		return DataFile{}, errors.New("csv has no usable connections")
	}
	return df, nil
}

// parseDSN 解析每行一个 clickhouse(s):// URI 的文本。
func parseDSN(raw string) (DataFile, error) {
	var df DataFile
	seen := 0
	for _, line := range strings.Split(raw, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if !strings.HasPrefix(s, "clickhouse://") && !strings.HasPrefix(s, "clickhouses://") {
			return DataFile{}, errors.New("not a clickhouse dsn line: " + s)
		}
		u, err := url.Parse(s)
		if err != nil {
			continue
		}
		c := Connection{
			Host:     u.Hostname(),
			Database: strings.Trim(u.Path, "/"),
		}
		if p, err := strconv.Atoi(u.Port()); err == nil {
			c.Port = p
		} else {
			c.Port = 8123
		}
		if u.User != nil {
			c.User = u.User.Username()
			c.Password, _ = u.User.Password()
		}
		if strings.HasPrefix(s, "clickhouses://") {
			c.Secure = true
		}
		q := u.Query()
		if parseBool(q.Get("compress")) {
			c.UseCompress = true
		}
		if q.Get("protocol") == "native" {
			c.Protocol = "native"
		}
		c.Name = c.Host
		c.Normalize()
		df.Connections = append(df.Connections, c)
		seen++
	}
	if seen == 0 {
		return DataFile{}, errors.New("dsn has no entries")
	}
	return df, nil
}

// applyConnField 把 YAML 单个键值写入连接对象，供 parseYAML 复用。
func applyConnField(c *Connection, key, val string) {
	switch key {
	case "id":
		c.ID = val
	case "folder_id":
		c.FolderID = val
	case "name":
		c.Name = val
	case "host":
		c.Host = val
	case "port":
		if n, err := strconv.Atoi(val); err == nil {
			c.Port = n
		}
	case "protocol":
		c.Protocol = val
	case "user":
		c.User = val
	case "password":
		c.Password = val
	case "database":
		c.Database = val
	case "secure":
		c.Secure = parseBool(val)
	case "use_compress":
		c.UseCompress = parseBool(val)
	}
}

// parseBool 宽松解析布尔文本，兼容 true/1/yes/on 等常见写法。
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on", "y", "t":
		return true
	default:
		return false
	}
}
