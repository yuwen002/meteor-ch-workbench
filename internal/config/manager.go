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

type Folder struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Connection struct {
	ID          string `json:"id"`
	FolderID    string `json:"folder_id,omitempty"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Password    string `json:"password"`
	Database    string `json:"database"`
	Secure      bool   `json:"secure"`
	UseCompress bool   `json:"use_compress"`
}

func (c *Connection) GenID() {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	c.ID = hex.EncodeToString(b)
}

func (f *Folder) GenID() {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	f.ID = hex.EncodeToString(b)
}

type DataFile struct {
	Folders     []Folder     `json:"folders"`
	Connections []Connection `json:"connections"`
}

type Manager struct {
	mu      sync.RWMutex
	path    string
	dir     string
	data    DataFile
}

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

func (m *Manager) ConfigDir() string {
	return m.dir
}

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
	return &m.data, nil
}

func (m *Manager) ListConnections() ([]Connection, error) {
	_, err := m.LoadAll()
	if err != nil {
		return nil, err
	}
	return m.data.Connections, nil
}

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

func (m *Manager) SaveConnection(conn Connection) error {
	if _, err := m.LoadAll(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn.ID == "" {
		conn.GenID()
	}
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

func (m *Manager) ListFolders() ([]Folder, error) {
	if _, err := m.LoadAll(); err != nil {
		return nil, err
	}
	return m.data.Folders, nil
}

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

func (m *Manager) flush() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o644)
}