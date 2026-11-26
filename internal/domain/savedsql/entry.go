// Package savedsql 定义用户主动收藏的 SQL 资产。
package savedsql

import "time"

// Entry 是一条收藏 SQL；收藏不受查询历史条数上限影响。
type Entry struct {
	ID string `json:"id"`
	Name string `json:"name"`
	SQL string `json:"sql"`
	ConnectionID string `json:"connection_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
