package app

import (
	"fmt"
	"strings"
)

// createTableColumn 描述建表时的一列。Default 与 Type 是 ClickHouse SQL 表达式，
// 由用户在建表表单中提供；名称和注释会由生成器转义。
type createTableColumn struct {
	Name    string
	Type    string
	Default string
	Comment string
}

// createTableDefinition 是 Gio 建表流程与 SQL 预览共用的纯数据模型。
type createTableDefinition struct {
	Database   string
	Table      string
	Comment    string
	Engine     string
	OrderBy    string
	PartitionBy string
	PrimaryKey string
	Settings   string
	Columns    []createTableColumn
}

var mergeTreeEngines = map[string]bool{
	"MergeTree": true,
	"ReplacingMergeTree": true,
	"ReplacingMergeTree(version)": true,
	"CollapsingMergeTree(sign)": true,
	"SummingMergeTree": true,
	"AggregatingMergeTree": true,
	"VersionedCollapsingMergeTree(sign, version)": true,
}

func quoteDDLIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func quoteDDLComment(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// buildCreateTableSQL 校验建表定义并生成可预览、可执行的 ClickHouse DDL。
func buildCreateTableSQL(def createTableDefinition) (string, error) {
	def.Database = strings.TrimSpace(def.Database)
	def.Table = strings.TrimSpace(def.Table)
	def.Engine = strings.TrimSpace(def.Engine)
	if def.Database == "" {
		return "", fmt.Errorf("请选择目标数据库")
	}
	if def.Table == "" {
		return "", fmt.Errorf("请输入表名")
	}
	if def.Engine == "" {
		def.Engine = "MergeTree"
	}
	if len(def.Columns) == 0 {
		return "", fmt.Errorf("至少需要定义一列")
	}

	columns := make([]createTableColumn, 0, len(def.Columns))
	seen := make(map[string]struct{}, len(def.Columns))
	for i, column := range def.Columns {
		column.Name = strings.TrimSpace(column.Name)
		column.Type = strings.TrimSpace(column.Type)
		if column.Name == "" || column.Type == "" {
			return "", fmt.Errorf("第 %d 列的名称和类型不能为空", i+1)
		}
		if _, exists := seen[column.Name]; exists {
			return "", fmt.Errorf("列名重复：%s", column.Name)
		}
		seen[column.Name] = struct{}{}
		columns = append(columns, column)
	}

	lines := make([]string, 0, len(columns))
	for _, column := range columns {
		line := "    " + quoteDDLIdentifier(column.Name) + " " + column.Type
		if strings.TrimSpace(column.Default) != "" {
			line += " DEFAULT " + strings.TrimSpace(column.Default)
		}
		if column.Comment != "" {
			line += " COMMENT " + quoteDDLComment(column.Comment)
		}
		lines = append(lines, line)
	}

	var sql strings.Builder
	fmt.Fprintf(&sql, "CREATE TABLE %s.%s (\n%s\n) ENGINE = %s",
		quoteDDLIdentifier(def.Database), quoteDDLIdentifier(def.Table),
		strings.Join(lines, ",\n"), def.Engine)

	if strings.TrimSpace(def.OrderBy) != "" {
		fmt.Fprintf(&sql, "\nORDER BY (%s)", strings.TrimSpace(def.OrderBy))
	} else if mergeTreeEngines[def.Engine] {
		sql.WriteString("\nORDER BY tuple()")
	}
	if value := strings.TrimSpace(def.PartitionBy); value != "" {
		fmt.Fprintf(&sql, "\nPARTITION BY %s", value)
	}
	if value := strings.TrimSpace(def.PrimaryKey); value != "" {
		fmt.Fprintf(&sql, "\nPRIMARY KEY (%s)", value)
	}
	if def.Comment != "" {
		fmt.Fprintf(&sql, "\nCOMMENT %s", quoteDDLComment(def.Comment))
	}
	if value := strings.TrimSpace(def.Settings); value != "" {
		fmt.Fprintf(&sql, "\nSETTINGS %s", value)
	}
	return sql.String(), nil
}
