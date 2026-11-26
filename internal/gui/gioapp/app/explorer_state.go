package app

// isCurrentObjectsResult 判断数据库对象列表响应是否仍属于当前请求上下文。
// 旧请求可能在切换连接或数据库后才返回，不能覆盖当前导航状态。
func isCurrentObjectsResult(result objectsResult, requestSeq uint64, connectionID, database string) bool {
	return result.seq == requestSeq && result.connectionID == connectionID && result.database == database
}

// isCurrentSchemaResult 判断表结构响应是否仍属于当前连接、数据库和表。
func isCurrentSchemaResult(result schemaResult, requestSeq uint64, connectionID, database, table string) bool {
	return result.seq == requestSeq && result.connectionID == connectionID && result.database == database && result.table == table
}
