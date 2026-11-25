package connection

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type memoryRepository struct { items []connection.Connection; saveErr, deleteErr error }
func (r *memoryRepository) Get(_ context.Context, id string) (connection.Connection, error) {
	for _, item := range r.items {
		if item.ID == id { return item, nil }
	}
	return connection.Connection{}, ErrNotFound
}
func (r *memoryRepository) List(context.Context) ([]connection.Connection, error) {
	return append([]connection.Connection(nil), r.items...), nil
}
func (r *memoryRepository) Save(_ context.Context, item connection.Connection) error {
	if r.saveErr != nil { return r.saveErr }
	for i := range r.items { if r.items[i].ID == item.ID { r.items[i] = item; return nil } }
	r.items = append(r.items, item); return nil
}
func (r *memoryRepository) Reorder(_ context.Context, ids []string) error {
	byID := make(map[string]connection.Connection, len(r.items))
	for _, item := range r.items { byID[item.ID] = item }
	seen := map[string]bool{}
	ordered := make([]connection.Connection, 0, len(r.items))
	for _, id := range ids { if item, ok := byID[id]; ok && !seen[id] { ordered = append(ordered, item); seen[id] = true } }
	for _, item := range r.items { if !seen[item.ID] { ordered = append(ordered, item) } }
	r.items = ordered
	return nil
}
func (r *memoryRepository) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil { return r.deleteErr }
	for i := range r.items { if r.items[i].ID == id { r.items = append(r.items[:i], r.items[i+1:]...); return nil } }
	return ErrNotFound
}
type fakeTester struct { err error; calls int }
func (f *fakeTester) Test(context.Context, connection.Connection) error { f.calls++; return f.err }
type fixedIDs struct { id string; err error }
func (f fixedIDs) NewID() (string, error) { return f.id, f.err }

func newTestService(t *testing.T, repo *memoryRepository, tester *fakeTester) *Service {
	t.Helper()
	s, err := NewService(repo, tester, fixedIDs{id:"generated-id"})
	if err != nil { t.Fatal(err) }
	return s
}
func validConnection() connection.Connection {
	return connection.Connection{ID:"", Name:"  analytics ", Host:"  127.0.0.1 ", Port:9000, Protocol:connection.ProtocolNative, User:" default "}
}

func TestGetReturnsConnectionByID(t *testing.T) {
	want := connection.Connection{ID:"c1", Name:"analytics"}
	s := newTestService(t, &memoryRepository{items:[]connection.Connection{want}}, &fakeTester{})
	got, err := s.Get(context.Background(), " c1 ")
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(got, want) { t.Fatalf("读取连接=%+v，期望 %+v", got, want) }
}
func TestGetRejectsEmptyID(t *testing.T) {
	s := newTestService(t, &memoryRepository{}, &fakeTester{})
	if _, err := s.Get(context.Background(), " "); !errors.Is(err, connection.ErrMissingID) {
		t.Fatalf("期望连接 ID 校验错误，得到 %v", err)
	}
}

func TestSaveGeneratesIDAndNormalizesInput(t *testing.T) {
	repo, tester := &memoryRepository{}, &fakeTester{}
	s := newTestService(t, repo, tester)
	got, err := s.Save(context.Background(), validConnection())
	if err != nil { t.Fatal(err) }
	if got.ID != "generated-id" || got.Name != "analytics" || got.Host != "127.0.0.1" || got.User != "default" {
		t.Fatalf("保存结果未归一化: %+v", got)
	}
	if len(repo.items) != 1 || repo.items[0].ID != got.ID { t.Fatalf("未持久化连接: %+v", repo.items) }
}
func TestSaveRejectsInvalidConnection(t *testing.T) {
	s := newTestService(t, &memoryRepository{}, &fakeTester{})
	item := validConnection(); item.Port = 70000
	if _, err := s.Save(context.Background(), item); !errors.Is(err, connection.ErrInvalidPort) {
		t.Fatalf("期望端口校验错误，得到 %v", err)
	}
}
func TestSavePropagatesIDGenerationFailure(t *testing.T) {
	repo := &memoryRepository{}
	s, err := NewService(repo, &fakeTester{}, fixedIDs{err:errors.New("entropy unavailable")})
	if err != nil { t.Fatal(err) }
	if _, err := s.Save(context.Background(), validConnection()); err == nil { t.Fatal("期望 ID 生成失败") }
	if len(repo.items) != 0 { t.Fatal("ID 生成失败时不应写入") }
}
func TestDeleteRequiresExistingConnection(t *testing.T) {
	s := newTestService(t, &memoryRepository{}, &fakeTester{})
	if err := s.Delete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) { t.Fatalf("期望 ErrNotFound，得到 %v", err) }
}
func TestDeleteRemovesConnection(t *testing.T) {
	repo := &memoryRepository{items:[]connection.Connection{{ID:"c1"}}}
	s := newTestService(t, repo, &fakeTester{})
	if err := s.Delete(context.Background(), "c1"); err != nil { t.Fatal(err) }
	if len(repo.items) != 0 { t.Fatalf("连接未删除: %+v", repo.items) }
}
func TestTestDoesNotPersistConnection(t *testing.T) {
	repo, tester := &memoryRepository{}, &fakeTester{}
	s := newTestService(t, repo, tester)
	item := validConnection()
	if err := s.Test(context.Background(), item); err != nil { t.Fatal(err) }
	if tester.calls != 1 { t.Fatalf("测试调用次数=%d", tester.calls) }
	if !reflect.DeepEqual(repo.items, []connection.Connection(nil)) { t.Fatal("连通性测试不应保存配置") }
}
func TestTestPropagatesDriverError(t *testing.T) {
	expected := errors.New("connection refused")
	s := newTestService(t, &memoryRepository{}, &fakeTester{err:expected})
	item := validConnection(); item.ID = "test-id"
	if err := s.Test(context.Background(), item); !errors.Is(err, expected) { t.Fatalf("错误链丢失: %v", err) }
}
