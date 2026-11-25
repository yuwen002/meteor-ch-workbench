package connection

import (
	"context"
	"errors"
	"testing"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type serviceStub struct {
	items []domain.Connection
	listErr error
	saveErr error
	deleteErr error
	testErr error
}

func (s *serviceStub) List(context.Context) ([]domain.Connection, error) {
	if s.listErr != nil { return nil, s.listErr }
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
func (s *serviceStub) Reorder(_ context.Context, ids []string) error {
	byID := make(map[string]domain.Connection, len(s.items))
	for _, item := range s.items { byID[item.ID] = item }
	seen := map[string]bool{}
	ordered := make([]domain.Connection, 0, len(s.items))
	for _, id := range ids { if item, ok := byID[id]; ok && !seen[id] { ordered = append(ordered, item); seen[id] = true } }
	for _, item := range s.items { if !seen[item.ID] { ordered = append(ordered, item) } }
	s.items = ordered
	return nil
}
func (s *serviceStub) Delete(_ context.Context, id string) error {
	if s.deleteErr != nil { return s.deleteErr }
	for i := range s.items {
		if s.items[i].ID == id { s.items = append(s.items[:i], s.items[i+1:]...); return nil }
	}
	return errors.New("not found")
}
func (s *serviceStub) Test(context.Context, domain.Connection) error { return s.testErr }

type folderStub struct { items []domain.Folder; deleteErr error }
func (s *folderStub) List(context.Context) ([]domain.Folder, error) { return append([]domain.Folder(nil), s.items...), nil }
func (s *folderStub) Save(_ context.Context, item domain.Folder) (domain.Folder, error) {
 for i := range s.items { if s.items[i].ID == item.ID { s.items[i] = item; return item, nil } }
 s.items = append(s.items, item); return item, nil
}
func (s *folderStub) Reorder(_ context.Context, ids []string) error {
	byID := make(map[string]domain.Folder, len(s.items))
	for _, item := range s.items { byID[item.ID] = item }
	seen := map[string]bool{}
	ordered := make([]domain.Folder, 0, len(s.items))
	for _, id := range ids { if item, ok := byID[id]; ok && !seen[id] { ordered = append(ordered, item); seen[id] = true } }
	for _, item := range s.items { if !seen[item.ID] { ordered = append(ordered, item) } }
	s.items = ordered
	return nil
}
func (s *folderStub) Delete(_ context.Context, id string) error {
 if s.deleteErr != nil { return s.deleteErr }
 for i := range s.items { if s.items[i].ID == id { s.items = append(s.items[:i], s.items[i+1:]...); return nil } }
 return errors.New("not found")
}
func newTestModel(t *testing.T, service Service) *Model {
 t.Helper()
 model, err := NewModel(service, &folderStub{})
 if err != nil { t.Fatal(err) }
 return model
}

func TestModelReloadAndSelect(t *testing.T) {
	first := domain.Connection{ID:"c1", Name:"Local"}
	model := newTestModel(t, &serviceStub{items:[]domain.Connection{first}})
	if err := model.Reload(context.Background()); err != nil { t.Fatal(err) }
	if !model.Select("c1") { t.Fatal("已有连接应可选中") }
	if model.Select("missing") { t.Fatal("不存在的连接不应被选中") }
	state := model.Snapshot()
	if len(state.Items) != 1 || state.SelectedID != "c1" || state.Loading { t.Fatalf("状态不正确: %+v", state) }
}

func TestModelSaveAndDelete(t *testing.T) {
	model := newTestModel(t, &serviceStub{})
	saved, err := model.Save(context.Background(), domain.Connection{ID:"c1", Name:"Local"})
	if err != nil { t.Fatal(err) }
	if saved.ID != "c1" || model.Snapshot().SelectedID != "c1" { t.Fatalf("保存状态不正确: %+v", model.Snapshot()) }
	if err := model.Delete(context.Background(), "c1"); err != nil { t.Fatal(err) }
	state := model.Snapshot()
	if len(state.Items) != 0 || state.SelectedID != "" { t.Fatalf("删除后状态未清理: %+v", state) }
}

func TestModelReportsServiceErrors(t *testing.T) {
	want := errors.New("storage unavailable")
	model := newTestModel(t, &serviceStub{saveErr:want})
	if _, err := model.Save(context.Background(), domain.Connection{ID:"c1"}); !errors.Is(err, want) { t.Fatalf("错误未透传: %v", err) }
	if model.Snapshot().Error == "" { t.Fatal("应在状态中保留错误提示") }
}

type blockingTestService struct {
	serviceStub
	started chan chan struct{}
}

func (s *blockingTestService) Test(context.Context, domain.Connection) error {
	release := make(chan struct{})
	s.started <- release
	<-release
	return nil
}

func TestModelNewerConnectionTestSupersedesOlderResult(t *testing.T) {
	service := &blockingTestService{started: make(chan chan struct{}, 2)}
	model := newTestModel(t, service)
	firstDone := make(chan struct{})
	go func() {
		_ = model.Test(context.Background(), domain.Connection{ID: "first"})
		close(firstDone)
	}()
	releaseFirst := <-service.started

	secondDone := make(chan struct{})
	go func() {
		_ = model.Test(context.Background(), domain.Connection{ID: "second"})
		close(secondDone)
	}()
	releaseSecond := <-service.started
	close(releaseSecond)
	<-secondDone
	if got := model.Snapshot().TestingID; got != "" {
		t.Fatalf("较新的测试完成后应清除测试状态，得到 %q", got)
	}
	close(releaseFirst)
	<-firstDone
	if got := model.Snapshot().TestingID; got != "" {
		t.Fatalf("较旧测试结束不应覆盖新状态，得到 %q", got)
	}
}

func TestModelReordersFoldersAndConnections(t *testing.T) {
	service := &serviceStub{items: []domain.Connection{{ID:"c1"},{ID:"c2"},{ID:"c3"}}}
	folders := &folderStub{items: []domain.Folder{{ID:"f1",Name:"one"},{ID:"f2",Name:"two"},{ID:"f3",Name:"three"}}}
	model, err := NewModel(service, folders)
	if err != nil { t.Fatal(err) }
	if err := model.Reload(context.Background()); err != nil { t.Fatal(err) }
	if err := model.ReorderFolders(context.Background(), []string{"f3","f1"}); err != nil { t.Fatal(err) }
	if err := model.ReorderConnections(context.Background(), []string{"c2","c1"}); err != nil { t.Fatal(err) }
	state := model.Snapshot()
	if got, want := []string{state.Folders[0].ID,state.Folders[1].ID,state.Folders[2].ID}, []string{"f3","f1","f2"}; !reflect.DeepEqual(got,want) {
		t.Fatalf("模型分组顺序=%v，期望 %v",got,want)
	}
	if got, want := []string{state.Items[0].ID,state.Items[1].ID,state.Items[2].ID}, []string{"c2","c1","c3"}; !reflect.DeepEqual(got,want) {
		t.Fatalf("模型连接顺序=%v，期望 %v",got,want)
	}
}

func TestModelFolderCRUD(t *testing.T) {
 folders := &folderStub{}
 model, err := NewModel(&serviceStub{}, folders)
 if err != nil { t.Fatal(err) }
 created, err := model.SaveFolder(context.Background(), domain.Folder{ID:"f1", Name:"生产"})
 if err != nil { t.Fatal(err) }
 if created.ID != "f1" || len(model.Snapshot().Folders) != 1 { t.Fatalf("新建分组失败: %+v", model.Snapshot()) }
 created.Name = "分析"
 if _, err := model.SaveFolder(context.Background(), created); err != nil { t.Fatal(err) }
 if got := model.Snapshot().Folders[0].Name; got != "分析" { t.Fatalf("重命名未生效: %s", got) }
 if err := model.DeleteFolder(context.Background(), "f1"); err != nil { t.Fatal(err) }
 if len(model.Snapshot().Folders) != 0 { t.Fatal("删除后分组仍存在") }
}


func TestBuildSidebarRowsKeepsConnectionsWithMissingFoldersVisible(t *testing.T) {
	items := []domain.Connection{
		{ID: "assigned", FolderID: "folder-existing", Name: "正常分组连接"},
		{ID: "orphaned", FolderID: "folder-missing", Name: "失效分组连接"},
		{ID: "unassigned", Name: "未分组连接"},
	}
	folders := []domain.Folder{{ID: "folder-existing", Name: "生产"}}

	rows, orphanedCount := buildSidebarRows(items, folders, nil)
	if orphanedCount != 1 {
		t.Fatalf("失效分组连接数量 = %d，期望 1", orphanedCount)
	}
	visible := make(map[string]bool)
	for _, row := range rows {
		if row.item != nil {
			visible[row.item.ID] = true
		}
	}
	for _, id := range []string{"assigned", "orphaned", "unassigned"} {
		if !visible[id] {
			t.Errorf("连接 %q 未出现在侧栏行中: %+v", id, rows)
		}
	}
}

func TestBuildSidebarRowsKeepsConnectionsWhenAllFoldersAreMissing(t *testing.T) {
	items := []domain.Connection{
		{ID: "orphaned-1", FolderID: "missing-1"},
		{ID: "orphaned-2", FolderID: "missing-2"},
	}
	rows, orphanedCount := buildSidebarRows(items, nil, nil)
	if orphanedCount != 2 {
		t.Fatalf("失效分组连接数量 = %d，期望 2", orphanedCount)
	}
	visible := 0
	for _, row := range rows {
		if row.item != nil {
			visible++
		}
	}
	if visible != len(items) {
		t.Fatalf("应展示全部连接，实际展示 %d / %d", visible, len(items))
	}
}

func TestModelReloadFailureKeepsPreviouslyLoadedConnections(t *testing.T) {
	want := domain.Connection{ID: "saved", Name: "历史连接"}
	service := &serviceStub{items: []domain.Connection{want}}
	model := newTestModel(t, service)
	if err := model.Reload(context.Background()); err != nil { t.Fatal(err) }

	service.listErr = errors.New("配置文件不可读")
	if err := model.Reload(context.Background()); err == nil { t.Fatal("配置读取失败应返回错误") }

	state := model.Snapshot()
	if state.Loading { t.Fatal("加载失败后 Loading 应恢复为 false") }
	if state.Error == "" { t.Fatal("加载失败原因应保留在状态中") }
	if len(state.Items) != 1 || state.Items[0].ID != want.ID {
		t.Fatalf("读取失败不应清空已加载连接: %+v", state.Items)
	}
}
