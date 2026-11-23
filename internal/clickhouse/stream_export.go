package clickhouse

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	"github.com/yuwen002/meteor-ch-workbench/internal/config"
)

// sqlRowScanner 是 database/sql.Rows 所需的最小接口，便于对游标写出过程做单元测试。
type sqlRowScanner interface {
	Columns() ([]string, error)
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// ExportQueryTo 直接执行 SQL 并将数据库游标逐行写入目标流。
// 该方法不受界面查询行数上限影响，内存占用只与单行数据及格式化缓冲区有关。
// 当前支持 CSV / TSV；queryID 非空时可通过 CancelQuery 中止导出。
func (s *Service) ExportQueryTo(conn config.Connection, sqlText, queryID, format string, dst io.Writer) (int64, error) {
	return s.ExportQueryToWithProgress(conn, sqlText, queryID, format, dst, nil)
}

// ExportQueryToWithProgress 每累计 1000 行及结束时回报一次导出进度。
func (s *Service) ExportQueryToWithProgress(conn config.Connection, sqlText, queryID, format string, dst io.Writer, progress func(int64)) (int64, error) {
	if dst == nil {
		return 0, errors.New("export destination is nil")
	}
	if format != "csv" && format != "tsv" {
		return 0, fmt.Errorf("unsupported streaming export format: %s", format)
	}
	db, err := s.getDB(conn)
	if err != nil {
		return 0, err
	}
	ctx, unregister := s.register(queryID)
	defer unregister()

	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		if isCanceled(ctx) {
			return 0, errors.New(errQueryCanceled)
		}
		return 0, err
	}
	count, streamErr := streamSQLRowsToWithProgress(ctx, rows, dst, format, progress)
	closeErr := rows.Close()
	if streamErr != nil {
		return count, streamErr
	}
	if closeErr != nil {
		return count, closeErr
	}
	return count, nil
}

// streamSQLRowsTo 从数据库游标读取一行、写出一行，不累积完整结果集。
func streamSQLRowsTo(ctx context.Context, rows sqlRowScanner, dst io.Writer, format string) (int64, error) {
	return streamSQLRowsToWithProgress(ctx, rows, dst, format, nil)
}

func streamSQLRowsToWithProgress(ctx context.Context, rows sqlRowScanner, dst io.Writer, format string, progress func(int64)) (int64, error) {
	columns, err := rows.Columns()
	if err != nil {
		return 0, err
	}
	switch format {
	case "csv":
		return streamCSVRowsWithProgress(ctx, rows, dst, columns, progress)
	case "tsv":
		return streamTSVRowsWithProgress(ctx, rows, dst, columns, progress)
	default:
		return 0, fmt.Errorf("unsupported streaming export format: %s", format)
	}
}

func streamCSVRows(ctx context.Context, rows sqlRowScanner, dst io.Writer, columns []string) (int64, error) {
	return streamCSVRowsWithProgress(ctx, rows, dst, columns, nil)
}

func streamCSVRowsWithProgress(ctx context.Context, rows sqlRowScanner, dst io.Writer, columns []string, progress func(int64)) (int64, error) {
	writer := csv.NewWriter(dst)
	if err := writer.Write(columns); err != nil {
		return 0, err
	}
	count, err := streamRowsWithProgress(ctx, rows, len(columns), func(values []interface{}) error {
		record := make([]string, len(values))
		for i, value := range values {
			record[i] = cellText(value)
		}
		return writer.Write(record)
	}, progress)
	writer.Flush()
	if err != nil {
		return count, err
	}
	if err := writer.Error(); err != nil {
		return count, err
	}
	return count, nil
}

func streamTSVRows(ctx context.Context, rows sqlRowScanner, dst io.Writer, columns []string) (int64, error) {
	return streamTSVRowsWithProgress(ctx, rows, dst, columns, nil)
}

func streamTSVRowsWithProgress(ctx context.Context, rows sqlRowScanner, dst io.Writer, columns []string, progress func(int64)) (int64, error) {
	writer := bufio.NewWriter(dst)
	writeRecord := func(values []string) error {
		for i, value := range values {
			if i > 0 {
				if err := writer.WriteByte('\t'); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(writer, escapeTSVCell(value)); err != nil {
				return err
			}
		}
		return writer.WriteByte('\n')
	}
	if err := writeRecord(columns); err != nil {
		return 0, err
	}
	count, err := streamRowsWithProgress(ctx, rows, len(columns), func(values []interface{}) error {
		record := make([]string, len(values))
		for i, value := range values {
			record[i] = cellText(value)
		}
		return writeRecord(record)
	}, progress)
	if flushErr := writer.Flush(); err == nil {
		err = flushErr
	}
	return count, err
}

func streamRows(ctx context.Context, rows sqlRowScanner, columnCount int, writeRow func([]interface{}) error) (int64, error) {
	return streamRowsWithProgress(ctx, rows, columnCount, writeRow, nil)
}

func streamRowsWithProgress(ctx context.Context, rows sqlRowScanner, columnCount int, writeRow func([]interface{}) error, progress func(int64)) (int64, error) {
	var count int64
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		values := make([]interface{}, columnCount)
		dest := make([]interface{}, columnCount)
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return count, err
		}
		for i, value := range values {
			values[i] = serializeValue(value)
		}
		if err := writeRow(values); err != nil {
			return count, err
		}
		count++
		if progress != nil && count%1000 == 0 {
			progress(count)
		}
	}
	if err := rows.Err(); err != nil {
		return count, err
	}
	if progress != nil {
		progress(count)
	}
	return count, nil
}

// compile-time assertion：database/sql.Rows 满足游标读取接口。
var _ sqlRowScanner = (*sql.Rows)(nil)
