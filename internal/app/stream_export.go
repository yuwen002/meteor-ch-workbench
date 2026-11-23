package app

import (
	"io"

	"github.com/yuwen002/meteor-ch-workbench/internal/clickhouse"
)

// ExportQueryTo 重新执行指定 SQL，并将 ClickHouse 游标直接流式写入目标文件。
// 与 ExportQueryResult（导出已缓存行）不同，该方法不受界面行数上限限制。
func (s *Service) ExportQueryTo(w io.Writer, connectionID, sqlText, queryID, format string) (int64, error) {
	return s.ExportQueryToWithProgress(w, connectionID, sqlText, queryID, format, nil)
}

// ExportQueryToWithProgress 将 ClickHouse 游标级导出进度回调到调用方。
func (s *Service) ExportQueryToWithProgress(w io.Writer, connectionID, sqlText, queryID, format string, progress func(int64)) (int64, error) {
	conn, err := s.conf.GetConnection(connectionID)
	if err != nil {
		return 0, err
	}
	return s.ch.ExportQueryToWithProgress(*conn, sqlText, queryID, format, w, progress)
}

// ResultExportFormats 返回目前支持数据库游标级流式导出的格式。
func (s *Service) StreamingResultExportFormats() []clickhouse.ResultExportFormat {
	all := clickhouse.ResultExportFormats()
	formats := make([]clickhouse.ResultExportFormat, 0, 2)
	for _, format := range all {
		if format.ID == "csv" || format.ID == "tsv" {
			formats = append(formats, format)
		}
	}
	return formats
}
