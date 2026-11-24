package connection

import (
	"context"
	"errors"
	"testing"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type serviceStub struct {
	items []domain.Connection
	saveErr error
	deleteErr error
	testErr error
}

func (s *serviceStub) List(context.Context) ([]domain.Connection, error) {
	return append([]domain.Connection(nil), s.items...), nil
}
func (s *serviceStub) Save(_ context.Context, item domain.Connection) (domain.Connection, error) {
	if s.saveErr != nil { return domain.Connection{}, s.saveErr }
	for i := range s.items {
		if s.items[i].ID == item.ID { s.items[i] = item; return item, nil }
	}
	s.items = append(s.items, item)
	return item, nil
}
func (s *serviceStub) Delete(_ context.Context, id string) error {
	if s.deleteErr != nil { return s.deleteErr }
	for i := range s.items {
		if s.items[i].ID == id { s.items = append(s.items[:i], s.items[i+1:]...); return nil }
	}
	return errors.New("not found")
}
func (s *serviceStub) Test(context.Context, domain.Connection) error { return s.testErr }

func TestModelReloadAndSelect(t *testing.T) {
	first := domain.Connection{ID:"c1", Name:"Local"}
	model, err := NewModel(&serviceStub{items:[]domain.Connection{first}})
	if err != nil { t.Fatal(err) }
	if err := model.Reload(context.Background()); err != nil { t.Fatal(err) }
	if !model.Select("c1") { t.Fatal("已有连接应可选中") }
	if model.Select("missing") { t.Fatal("不存在的连接不应被选中") }
	state := model.Snapshot()
	if len(state.Items) != 1 || state.SelectedID != "c1" || state.Loading { t.Fatalf("状态不正确: %+v", state) }
}

func TestModelSaveAndDelete(t *testing.T) {
	model, err := NewModel(&serviceStub{})
	if err != nil { t.Fatal(err) }
	saved, err := model.Save(context.Background(), domain.Connection{ID:"c1", Name:"Local"})
	if err != nil { t.Fatal(err) }
	if saved.ID != "c1" || model.Snapshot().SelectedID != "c1" { t.Fatalf("保存状态不正确: %+v", model.Snapshot()) }
	if err := model.Delete(context.Background(), "c1"); err != nil { t.Fatal(err) }
	state := model.Snapshot()
	if len(state.Items) != 0 || state.SelectedID != "" { t.Fatalf("删除后状态未清理: %+v", state) }
}

func TestModelReportsServiceErrors(t *testing.T) {
	want := errors.New("storage unavailable")
	model, err := NewModel(&serviceStub{saveErr:want})
	if err != nil { t.Fatal(err) }
	if _, err := model.Save(context.Background(), domain.Connection{ID:"c1"}); !errors.Is(err, want) { t.Fatalf("错误未透传: %v", err) }
	if model.Snapshot().Error == "" { t.Fatal("应在状态中保留错误提示") }
}
