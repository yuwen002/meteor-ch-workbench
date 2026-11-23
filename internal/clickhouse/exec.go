package clickhouse

import (
	"context"
	"strings"
	"time"

	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// Exec 执行一条不返回结果集的 SQL（DDL / DML）。
// 包括 CREATE / ALTER / DROP / TRUNCATE / INSERT / RENAME 等。
// 与 Query 保持一致：SQL 执行期错误写入 ExecResult.Error，
// 只有连接等基础设施错误才作为 error 返回。
func (s *Service) Exec(conn config.Connection, sqlText string) (*ExecResult, error) {
	db, err := s.getDB(conn)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	res, err := db.ExecContext(ctx, ensureMutationSync(sqlText))
	if err != nil {
		return &ExecResult{Error: err.Error()}, nil
	}

	affected, _ := res.RowsAffected()
	return &ExecResult{RowsAffected: affected}, nil
}

// execFunc 执行单条语句并返回受影响行数，供 ExecBatch 注入以便单测。
type execFunc func(ctx context.Context, sqlText string) (int64, error)

// ExecBatch 在同一个连接池上顺序执行多条 DDL/DML 语句。
//
// 相对逐条调用 Exec，批量执行只建立一次连接、只做一次跨进程往返，
// 避免「改 N 个单元格 = N 次 API 调用」的串行等待。
// 语句始终串行执行：ClickHouse 的 ALTER ... UPDATE 是 mutation，
// 并发提交只会让服务端排队，收益为零。
//
// continueOnError 为 false 时遇到第一条失败即停止，后续语句标记为 skipped；
// 为 true 时继续执行剩余语句，一次收集全部失败点。
//
// 与 Exec 保持一致：SQL 执行期错误写入回执中的 Error，不向上返回 error；
// 只有获取连接等基础设施错误才作为 error 返回。
func (s *Service) ExecBatch(conn config.Connection, statements []string, continueOnError bool) (*ExecBatchResult, error) {
	return s.execBatchWith(statements, continueOnError, func(ctx context.Context, sqlText string) (int64, error) {
		db, err := s.getDB(conn)
		if err != nil {
			return 0, err
		}
		res, err := db.ExecContext(ctx, ensureMutationSync(sqlText))
		if err != nil {
			return 0, err
		}
		affected, _ := res.RowsAffected()
		return affected, nil
	})
}

// execBatchWith 是 ExecBatch 的内部实现，执行动作由 exec 注入，
// 便于在没有真实 ClickHouse 的单元测试中覆盖中断 / 全量收集 / 下标回执等分支。
func (s *Service) execBatchWith(statements []string, continueOnError bool, exec execFunc) (*ExecBatchResult, error) {
	start := time.Now()
	result := &ExecBatchResult{Statements: make([]StatementResult, 0, len(statements))}

	ctx := context.Background()
	stopped := false
	for i, stmt := range statements {
		if stopped {
			result.Statements = append(result.Statements, StatementResult{Index: i, SQL: stmt})
			result.Skipped++
			continue
		}
		affected, err := exec(ctx, stmt)
		entry := StatementResult{Index: i, SQL: stmt}
		if err != nil {
			entry.Error = err.Error()
			result.Statements = append(result.Statements, entry)
			result.Failed++
			if !continueOnError {
				stopped = true
			}
			continue
		}
		entry.RowsAffected = affected
		result.Statements = append(result.Statements, entry)
		result.Ok++
	}
	result.ElapsedMs = elapsedMs(start)
	return result, nil
}

// ensureMutationSync 对 ALTER ... UPDATE / DELETE 类 mutation 语句追加 SETTINGS mutations_sync=2，
// 让服务端等 mutation 在全部副本上完成后才返回，避免调用方紧接着查询时仍读到旧数据。
// 非 mutation 语句，或调用方已显式指定 mutations_sync 时原样返回。
func ensureMutationSync(sqlText string) string {
	if !isMutationStatement(sqlText) {
		return sqlText
	}
	if hasSetting(sqlText, "mutations_sync") {
		return sqlText
	}
	trimmed := strings.TrimRight(strings.TrimSpace(sqlText), ";")
	return trimmed + " SETTINGS mutations_sync = 2"
}

// isMutationStatement 判断语句是否为 ClickHouse 需要等待完成的 mutation。
// 只看首两个关键字：ALTER TABLE ... UPDATE / DELETE，以及独立的 DELETE FROM。
func isMutationStatement(sqlText string) bool {
	fields := strings.Fields(strings.ToUpper(stripLeadingComments(sqlText)))
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "DELETE" {
		return len(fields) > 1 && fields[1] == "FROM"
	}
	if fields[0] != "ALTER" || len(fields) < 3 {
		return false
	}
	for _, f := range fields {
		if f == "UPDATE" || f == "DELETE" {
			return true
		}
	}
	return false
}

// stripLeadingComments 去掉语句开头的空白与 -- / /* */ 注释，便于按关键字判断语句类型。
func stripLeadingComments(sqlText string) string {
	s := strings.TrimSpace(sqlText)
	for {
		if strings.HasPrefix(s, "--") {
			if idx := strings.IndexByte(s, '\n'); idx >= 0 {
				s = strings.TrimSpace(s[idx+1:])
				continue
			}
			return ""
		}
		if strings.HasPrefix(s, "/*") {
			if idx := strings.Index(s, "*/"); idx >= 0 {
				s = strings.TrimSpace(s[idx+2:])
				continue
			}
			return ""
		}
		return s
	}
}

// hasSetting 判断语句中是否已显式包含指定的 SETTINGS 键。
func hasSetting(sqlText, key string) bool {
	lower := strings.ToLower(sqlText)
	return strings.Contains(lower, strings.ToLower(key)+" ")
}