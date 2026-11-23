package query

import (
	"context"
	"errors"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type fakeLookup struct { item connection.Connection; err error; gotID string }
func (f *fakeLookup) Get(_ context.Context, id string) (connection.Connection, error) { f.gotID = id; return f.item, f.err }

type fakeExecutor struct { called bool; got connection.Connection; request Request }
func (f *fakeExecutor) Execute(_ context.Context, item connection.Connection, request Request, _ func([]Column) error, _ BatchHandler) (Summary, error) {
	f.called = true; f.got = item; f.request = request
	return Summary{RowCount: 2}, nil
}

func TestServiceExecuteLoadsConnectionAndNormalizesRequest(t *testing.T) {
	item := connection.Connection{ID: "conn-1", Name: "Local", Host: "localhost", Port: 9000, Protocol: connection.ProtocolNative}
	lookup := &fakeLookup{item: item}
	executor := &fakeExecutor{}
	service, err := NewService(lookup, executor)
	if err != nil { t.Fatal(err) }
	got, err := service.Execute(context.Background(), Request{ConnectionID: " conn-1 ", SQL: " select 1 "}, nil, nil)
	if err != nil { t.Fatal(err) }
	if got.RowCount != 2 || !executor.called { t.Fatalf("未执行查询用例: %+v", got) }
	if lookup.gotID != item.ID || executor.got.ID != item.ID || executor.request.SQL != "select 1" || executor.request.Mode != ModeRows { t.Fatalf("请求未正确归一化: %+v", executor.request) }
}

func TestServiceExecuteValidatesRequest(t *testing.T) {
	lookup := &fakeLookup{}
	executor := &fakeExecutor{}
	service, err := NewService(lookup, executor)
	if err != nil { t.Fatal(err) }
	if _, err := service.Execute(context.Background(), Request{SQL: "select 1"}, nil, nil); !errors.Is(err, ErrMissingConnectionID) { t.Fatalf("连接 ID 校验错误: %v", err) }
	if _, err := service.Execute(context.Background(), Request{ConnectionID: "conn-1"}, nil, nil); !errors.Is(err, ErrEmptySQL) { t.Fatalf("SQL 校验错误: %v", err) }
	if _, err := service.Execute(context.Background(), Request{ConnectionID: "conn-1", SQL: "select 1", Mode: "unknown"}, nil, nil); !errors.Is(err, ErrInvalidMode) { t.Fatalf("执行模式校验错误: %v", err) }
	if executor.called { t.Fatal("无效请求不应调用执行器") }
}

func TestServiceExecutePropagatesLookupError(t *testing.T) {
	want := errors.New("storage unavailable")
	lookup := &fakeLookup{err: want}
	executor := &fakeExecutor{}
	service, err := NewService(lookup, executor)
	if err != nil { t.Fatal(err) }
	_, err = service.Execute(context.Background(), Request{ConnectionID: "conn-1", SQL: "select 1"}, nil, nil)
	if !errors.Is(err, want) { t.Fatalf("未保留底层错误: %v", err) }
	if executor.called { t.Fatal("连接读取失败后不应执行 SQL") }
}
