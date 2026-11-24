package metadata

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

var (
	ErrMissingConnectionID = errors.New("连接 ID 不能为空")
	ErrMissingDatabase = errors.New("数据库名称不能为空")
	ErrMissingTable = errors.New("表名称不能为空")
)

type Service struct {
	connections ConnectionLookup
	reader Reader
}

func NewService(connections ConnectionLookup, reader Reader) (*Service, error) {
	if connections == nil { return nil, errors.New("连接查询端口不能为空") }
	if reader == nil { return nil, errors.New("元数据读取端口不能为空") }
	return &Service{connections: connections, reader: reader}, nil
}

func (s *Service) connection(ctx context.Context, id string) (connection.Connection, error) {
	id = strings.TrimSpace(id)
	if id == "" { return connection.Connection{}, ErrMissingConnectionID }
	item, err := s.connections.Get(ctx, id)
	if err != nil { return connection.Connection{}, fmt.Errorf("读取连接配置: %w", err) }
	if err := ctx.Err(); err != nil { return connection.Connection{}, err }
	return item, nil
}

func (s *Service) ListDatabases(ctx context.Context, connectionID string) ([]string, error) {
	item, err := s.connection(ctx, connectionID)
	if err != nil { return nil, err }
	return s.reader.ListDatabases(ctx, item)
}

func (s *Service) ListTables(ctx context.Context, connectionID, database string) ([]Object, error) {
	database = strings.TrimSpace(database)
	if database == "" { return nil, ErrMissingDatabase }
	item, err := s.connection(ctx, connectionID)
	if err != nil { return nil, err }
	return s.reader.ListTables(ctx, item, database)
}

func (s *Service) ListViews(ctx context.Context, connectionID, database string) ([]Object, error) {
	database = strings.TrimSpace(database)
	if database == "" { return nil, ErrMissingDatabase }
	item, err := s.connection(ctx, connectionID)
	if err != nil { return nil, err }
	return s.reader.ListViews(ctx, item, database)
}

func (s *Service) GetTableSchema(ctx context.Context, connectionID, database, table string) ([]Column, error) {
	database, table = strings.TrimSpace(database), strings.TrimSpace(table)
	if database == "" { return nil, ErrMissingDatabase }
	if table == "" { return nil, ErrMissingTable }
	item, err := s.connection(ctx, connectionID)
	if err != nil { return nil, err }
	return s.reader.GetTableSchema(ctx, item, database, table)
}

func (s *Service) GetTableDetails(ctx context.Context, connectionID, database, table string) (TableDetails, error) {
	database, table = strings.TrimSpace(database), strings.TrimSpace(table)
	if database == "" { return TableDetails{}, ErrMissingDatabase }
	if table == "" { return TableDetails{}, ErrMissingTable }
	item, err := s.connection(ctx, connectionID)
	if err != nil { return TableDetails{}, err }
	return s.reader.GetTableDetails(ctx, item, database, table)
}
