package query

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

var (
	ErrMissingConnectionID = errors.New("连接 ID 不能为空")
	ErrEmptySQL = errors.New("SQL 语句不能为空")
	ErrInvalidMode = errors.New("SQL 执行模式无效")
)

type Service struct {
	connections ConnectionLookup
	executor Executor
}

func NewService(connections ConnectionLookup, executor Executor) (*Service, error) {
	if connections == nil { return nil, errors.New("连接查询端口不能为空") }
	if executor == nil { return nil, errors.New("SQL 执行器不能为空") }
	return &Service{connections: connections, executor: executor}, nil
}

// Execute 读取连接配置并交给基础设施执行；结果通过批次回调交付，不在应用层累积全量行。
func (s *Service) Execute(ctx context.Context, request Request, onColumns func([]Column) error, onBatch BatchHandler) (Summary, error) {
	request.ConnectionID = strings.TrimSpace(request.ConnectionID)
	request.SQL = strings.TrimSpace(request.SQL)
	if request.ConnectionID == "" { return Summary{}, ErrMissingConnectionID }
	if request.SQL == "" { return Summary{}, ErrEmptySQL }
	if request.Mode == "" { request.Mode = ModeRows }
	if request.Mode != ModeRows && request.Mode != ModeExec { return Summary{}, fmt.Errorf("%w: %q", ErrInvalidMode, request.Mode) }
	item, err := s.connections.Get(ctx, request.ConnectionID)
	if err != nil {
		if errors.Is(err, connection.ErrMissingID) { return Summary{}, err }
		return Summary{}, fmt.Errorf("读取连接配置: %w", err)
	}
	if err := ctx.Err(); err != nil { return Summary{}, err }
	return s.executor.Execute(ctx, item, request, onColumns, onBatch)
}
