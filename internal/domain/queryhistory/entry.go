// Package queryhistory 定义本地 SQL 查询历史记录。
package queryhistory

import "time"

// Entry 是一次 SQL 执行的历史摘要；不保存查询结果集。
type Entry struct {
	ID           string    `json:"id"`
	SQL          string    `json:"sql"`
	ConnectionID string    `json:"connection_id"`
	Timestamp    time.Time `json:"timestamp"`
	DurationMs   float64   `json:"duration_ms"`
	Success      bool      `json:"success"`
}
