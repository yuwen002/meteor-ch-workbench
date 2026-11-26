package app

import metadataapp "github.com/yuwen002/meteor-ch-workbench/internal/application/metadata"

// explorerSelection 聚合对象导航自身的数据与选中上下文。
// 它不持有连接服务、请求取消函数或 Gio 控件，便于独立测试状态切换规则。
type explorerSelection struct {
	databases        []string
	selectedDatabase string
	selectedTable    string
	selectedObject   string
	tables           []metadataapp.Object
	views            []metadataapp.Object
	columns          []metadataapp.Column
	tableDetails     metadataapp.TableDetails
}

// ResetConnection 清空连接切换后所有数据库与对象导航状态。
func (s *explorerSelection) ResetConnection() {
	s.databases = nil
	s.ClearDatabase()
}

// SetDatabases 更新当前连接可见的数据库列表。
func (s *explorerSelection) SetDatabases(databases []string) {
	s.databases = databases
}

// SelectDatabase 切换数据库，并清除上一个数据库下的对象与表结构状态。
func (s *explorerSelection) SelectDatabase(database string) {
	s.selectedDatabase = database
	s.selectedTable = ""
	s.selectedObject = ""
	s.tables = nil
	s.views = nil
	s.ClearSchema()
}

// ClearDatabase 折叠当前数据库并清理其下属对象状态。
func (s *explorerSelection) ClearDatabase() {
	s.SelectDatabase("")
}

// SetObjects 设置当前数据库的表和视图列表。
func (s *explorerSelection) SetObjects(tables, views []metadataapp.Object) {
	s.tables = tables
	s.views = views
}

// SelectTable 选中表并清理上一次读取的表结构。
func (s *explorerSelection) SelectTable(table string) {
	s.selectedTable = table
	s.ClearSchema()
}

// SetSelectedObject 更新资源树中的高亮对象键。
func (s *explorerSelection) SetSelectedObject(key string) {
	s.selectedObject = key
}

// SetSchema 设置当前表的字段与详情。
func (s *explorerSelection) SetSchema(columns []metadataapp.Column, details metadataapp.TableDetails) {
	s.columns = columns
	s.tableDetails = details
}

// ClearSchema 清理表结构与 DDL 详情。
func (s *explorerSelection) ClearSchema() {
	s.columns = nil
	s.tableDetails = metadataapp.TableDetails{}
}
