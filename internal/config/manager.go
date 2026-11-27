// Package config 负责应用配置的持久化，包括连接列表与文件夹分组的读写。
// 配置以 JSON 文件形式保存在用户配置目录下的 connections.json 中。
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Folder 表示侧边栏中的一个连接分组，用于把多个连接归类管理。
type Folder struct {
	ID   string `json:"id"`   // 文件夹唯一标识，由 GenID 生成的 16 位十六进制串
	Name string `json:"name"` // 文件夹显示名称
}

// Connection 表示一条 ClickHouse 连接配置。
// 字段名与前端 TypeScript 类型保持一致，通过 JSON tag 序列化到配置文件。
type Connection struct {
	ID          string `json:"id"`                  // 连接唯一标识
	FolderID    string `json:"folder_id,omitempty"` // 所属文件夹 ID，为空表示位于根级
	Name        string `json:"name"`                // 连接显示名称
	Host        string `json:"host"`                // 主机地址
	Port        int    `json:"port"`                // 端口，决定默认协议（9000/9440 为 native）
	Protocol    string `json:"protocol"`            // 连接协议：native（TCP）或 http
	User        string `json:"user"`                // 用户名
	Password    string `json:"password"`            // 密码（明文保存，仅存放于本机配置目录）
	Database    string `json:"database"`            // 默认数据库，可为空
	Secure      bool   `json:"secure"`              // 是否启用加密连接（TLS）
	UseCompress bool   `json:"use_compress"`        // 是否启用传输压缩
}

// GenID 为连接生成一个随机的 16 位十六进制 ID。
// 使用加密安全随机源，避免多台机器间出现 ID 冲突。
func (c *Connection) GenID() {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	c.ID = hex.EncodeToString(b)
}

// sanitizeOrphanFolders 清理 folder_id 指向不存在文件夹的连接，将其 FolderID 清空。
// 旧版 Wails 前端可能残留已删除文件夹的 ID，导致这些连接在 Tree 中不可见。
// 返回值表示是否有连接被修改。
func sanitizeOrphanFolders(df *DataFile) bool {
	changed := false
	exists := make(map[string]struct{}, len(df.Folders))
	for _, f := range df.Folders {
		if f.ID != "" {
			exists[f.ID] = struct{}{}
		}
	}
	for i := range df.Connections {
		if df.Connections[i].FolderID == "" {
			continue
		}
		if _, ok := exists[df.Connections[i].FolderID]; !ok {
			df.Connections[i].FolderID = ""
			changed = true
		}
	}
	return changed
}

// Normalize 对连接配置做兼容性归一化。
// 早期版本的配置没有 protocol 字段，这里根据端口自动推断，保证旧配置开箱可用，
// 用户无需手动修改即可正常连接。
func (c *Connection) Normalize() {
	if c.Protocol == "" {
		c.Protocol = GuessProtocolByPort(c.Port)
	}
}

// GuessProtocolByPort 根据端口推断连接协议（仅在旧配置缺少 protocol 时作为兜底）。
// 9000/9440 是 ClickHouse 的默认 Native TCP 端口，其余端口（如 8123/8443 或自定义端口）
// 一律按 HTTP 处理，因为绝大多数对外暴露的自定义端口都是 HTTP 接口。
// 注意：这里只是「猜」，用户显式选择的协议始终以配置中的 protocol 字段为准，不会被覆盖。
func GuessProtocolByPort(port int) string {
	switch port {
	case 9000, 9440:
		return "native"
	default:
		return "http"
	}
}

// GenID 为文件夹生成一个随机的 16 位十六进制 ID。
func (f *Folder) GenID() {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	f.ID = hex.EncodeToString(b)
}

// DataFile 是 connections.json 的完整数据结构，即一个配置文件对应一份数据。
type DataFile struct {
	Folders     []Folder     `json:"folders"`     // 文件夹分组列表
	Connections []Connection `json:"connections"` // 连接列表
}

// Manager 是配置管理器，负责 connections.json 的读写与内存缓存。
// 所有公开方法都通过读写锁保证并发安全（Wails 的每个前端调用可能来自不同 goroutine）。
type Manager struct {
	mu   sync.RWMutex // 保护 data 字段的读写锁
	path string       // 配置文件完整路径
	dir  string       // 配置目录，用于「打开配置目录」功能
	data DataFile     // 内存中的配置缓存
}

// NewManager 创建配置管理器，并确保配置目录存在。
// 配置目录位于系统用户配置目录下（如 Windows 的 %APPDATA%），子目录名为 meteor-ch-workbench。
func NewManager() *Manager {
	dir, _ := os.UserConfigDir()
	if dir == "" {
		dir = "."
	}
	dir = filepath.Join(dir, "meteor-ch-workbench")
	_ = os.MkdirAll(dir, 0o755)
	return &Manager{
		path: filepath.Join(dir, "connections.json"),
		dir:  dir,
		data: DataFile{
			Folders:     []Folder{},
			Connections: []Connection{},
		},
	}
}

// ConfigDir 返回配置文件所在目录，供「打开配置目录」按钮使用。
func (m *Manager) ConfigDir() string {
	return m.dir
}

// ExportData 导出当前全部配置，序列化为带缩进的 JSON 文本，供前端下载为文件。
func (m *Manager) ExportData() (string, error) {
	if _, err := m.LoadAll(); err != nil {
		return "", err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportData 导入 JSON 配置文本，覆盖当前全部连接与文件夹并写回磁盘。
// 导入前会校验 JSON 格式，并对缺失 ID 的条目自动生成 ID、对缺失 protocol 的连接做归一化，
// 避免导入后出现无法使用的配置。空字段（folders/connections）按空列表处理。
func (m *Manager) ImportData(raw string) error {
	var df DataFile
	if err := json.Unmarshal([]byte(raw), &df); err != nil {
		return errors.New("invalid config file: " + err.Error())
	}
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
	for i := range df.Connections {
		if df.Connections[i].ID == "" {
			df.Connections[i].GenID()
		}
		df.Connections[i].Normalize()
	}
	sanitizeOrphanFolders(&df)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = df
	return m.flush()
}

// LoadAll 从磁盘读取配置文件并刷新内存缓存。
// 若文件不存在或内容为空，则返回空数据结构而不报错（首次启动的正常情况）。
// 加载过程中会对缺失 protocol 的旧连接做自动补全，并在确有变更时回写磁盘，
// 实现旧配置的自动升级。
func (m *Manager) LoadAll() (*DataFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, err := os.ReadFile(m.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &m.data, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return &m.data, nil
	}
	var df DataFile
	if err := json.Unmarshal(data, &df); err != nil {
		return nil, err
	}
	m.data = df
	if m.data.Folders == nil {
		m.data.Folders = []Folder{}
	}
	if m.data.Connections == nil {
		m.data.Connections = []Connection{}
	}
	changed := false
	for i := range m.data.Connections {
		if m.data.Connections[i].Protocol == "" {
			m.data.Connections[i].Normalize()
			changed = true
		}
	}
	if sanitizeOrphanFolders(&m.data) {
		changed = true
	}
	if changed {
		_ = m.flush()
	}
	return &m.data, nil
}

// ListConnections 返回全部连接配置，加载失败时返回错误。
func (m *Manager) ListConnections() ([]Connection, error) {
	_, err := m.LoadAll()
	if err != nil {
		return nil, err
	}
	return m.data.Connections, nil
}

// GetConnection 按 ID 查找连接配置，返回副本以避免调用方误改缓存数据。
// 找不到时返回错误。
func (m *Manager) GetConnection(id string) (*Connection, error) {
	if _, err := m.LoadAll(); err != nil {
		return nil, err
	}
	for _, c := range m.data.Connections {
		if c.ID == id {
			cp := c
			return &cp, nil
		}
	}
	return nil, errors.New("connection not found: " + id)
}

// SaveConnection 保存一条连接配置：ID 为空时自动生成，已存在则覆盖，否则追加。
// 保存前会执行 Normalize 以确保协议字段完整，最后写回磁盘。
func (m *Manager) SaveConnection(conn Connection) error {
	if _, err := m.LoadAll(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn.ID == "" {
		conn.GenID()
	}
	conn.Normalize()
	idx := -1
	for i, c := range m.data.Connections {
		if c.ID == conn.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		m.data.Connections[idx] = conn
	} else {
		m.data.Connections = append(m.data.Connections, conn)
	}
	return m.flush()
}

// DeleteConnection 按 ID 删除连接配置并写回磁盘。
func (m *Manager) DeleteConnection(id string) error {
	if _, err := m.LoadAll(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := m.data.Connections[:0]
	for _, c := range m.data.Connections {
		if c.ID != id {
			filtered = append(filtered, c)
		}
	}
	m.data.Connections = filtered
	return m.flush()
}

// ListFolders 返回全部文件夹分组。
func (m *Manager) ListFolders() ([]Folder, error) {
	if _, err := m.LoadAll(); err != nil {
		return nil, err
	}
	return m.data.Folders, nil
}

// SaveFolder 保存一个文件夹分组：ID 为空时自动生成，已存在则覆盖，否则追加。
func (m *Manager) SaveFolder(f Folder) error {
	if _, err := m.LoadAll(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if f.ID == "" {
		f.GenID()
	}
	idx := -1
	for i, fl := range m.data.Folders {
		if fl.ID == f.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		m.data.Folders[idx] = f
	} else {
		m.data.Folders = append(m.data.Folders, f)
	}
	return m.flush()
}

// ReorderFolders 按传入的 ID 顺序重排文件夹列表，用于侧边栏拖拽排序。
// 传入列表未覆盖的文件夹（理论上不会发生）会保留原相对顺序追加到末尾。
func (m *Manager) ReorderFolders(ids []string) error {
	if _, err := m.LoadAll(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	byID := make(map[string]Folder, len(m.data.Folders))
	for _, f := range m.data.Folders {
		byID[f.ID] = f
	}
	seen := make(map[string]bool, len(ids))
	ordered := make([]Folder, 0, len(m.data.Folders))
	for _, id := range ids {
		f, ok := byID[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		ordered = append(ordered, f)
	}
	for _, f := range m.data.Folders {
		if !seen[f.ID] {
			ordered = append(ordered, f)
		}
	}
	m.data.Folders = ordered
	return m.flush()
}

// ReorderConnections 按传入的 ID 顺序重排连接列表，用于侧边栏拖拽排序。
// 连接以扁平数组存储，其相对顺序即侧边栏分组内的展示顺序，
// 因此重排数组即可实现组内的排序以及跨组时的先后关系。
// 传入列表未覆盖的连接会保留原相对顺序追加到末尾。
func (m *Manager) ReorderConnections(ids []string) error {
	if _, err := m.LoadAll(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	byID := make(map[string]Connection, len(m.data.Connections))
	for _, c := range m.data.Connections {
		byID[c.ID] = c
	}
	seen := make(map[string]bool, len(ids))
	ordered := make([]Connection, 0, len(m.data.Connections))
	for _, id := range ids {
		c, ok := byID[id]
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		ordered = append(ordered, c)
	}
	for _, c := range m.data.Connections {
		if !seen[c.ID] {
			ordered = append(ordered, c)
		}
	}
	m.data.Connections = ordered
	return m.flush()
}

// DeleteFolder 删除文件夹分组，并把原本归属它的连接移动回根级，
// 避免连接因分组被删而丢失。
func (m *Manager) DeleteFolder(id string) error {
	if _, err := m.LoadAll(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	filteredF := m.data.Folders[:0]
	for _, f := range m.data.Folders {
		if f.ID != id {
			filteredF = append(filteredF, f)
		}
	}
	m.data.Folders = filteredF
	for i := range m.data.Connections {
		if m.data.Connections[i].FolderID == id {
			m.data.Connections[i].FolderID = ""
		}
	}
	return m.flush()
}

// flush 把内存中的配置以缩进 JSON 格式写回磁盘。
// 调用方需自行持有写锁，本方法不做额外加锁。
func (m *Manager) flush() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o644)
}