package folder

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

type memoryRepository struct { items []connection.Folder; deleteErr error }
func (r *memoryRepository) ListFolders(context.Context) ([]connection.Folder, error) {
	return append([]connection.Folder(nil), r.items...), nil
}
func (r *memoryRepository) SaveFolder(_ context.Context, item connection.Folder) error {
	for i := range r.items { if r.items[i].ID == item.ID { r.items[i] = item; return nil } }
	r.items = append(r.items, item)
	return nil
}
func (r *memoryRepository) ReorderFolders(_ context.Context, ids []string) error {
	byID := make(map[string]connection.Folder, len(r.items))
	for _, item := range r.items { byID[item.ID] = item }
	seen := map[string]bool{}
	ordered := make([]connection.Folder, 0, len(r.items))
	for _, id := range ids { if item, ok := byID[id]; ok && !seen[id] { ordered = append(ordered, item); seen[id] = true } }
	for _, item := range r.items { if !seen[item.ID] { ordered = append(ordered, item) } }
	r.items = ordered
	return nil
}
func (r *memoryRepository) DeleteFolder(_ context.Context, id string) error {
	if r.deleteErr != nil { return r.deleteErr }
	for i := range r.items { if r.items[i].ID == id { r.items = append(r.items[:i], r.items[i+1:]...); return nil } }
	return errors.New("not found")
}
type fixedIDs struct { id string; err error }
func (f fixedIDs) NewID() (string, error) { return f.id, f.err }

func TestSaveCreatesAndRenamesFolder(t *testing.T) {
	repo := &memoryRepository{}
	s, err := NewService(repo, fixedIDs{id:"folder-1"})
	if err != nil { t.Fatal(err) }
	created, err := s.Save(context.Background(), connection.Folder{Name:"  Production "})
	if err != nil { t.Fatal(err) }
	if created.ID != "folder-1" || created.Name != "Production" { t.Fatalf("新建分组不正确: %+v", created) }
	created.Name = "Analytics"
	updated, err := s.Save(context.Background(), created)
	if err != nil { t.Fatal(err) }
	if updated.Name != "Analytics" || !reflect.DeepEqual(repo.items, []connection.Folder{updated}) { t.Fatalf("重命名结果不正确: %+v", repo.items) }
}

func TestSaveRejectsEmptyName(t *testing.T) {
	s, err := NewService(&memoryRepository{}, fixedIDs{id:"folder-1"})
	if err != nil { t.Fatal(err) }
	if _, err := s.Save(context.Background(), connection.Folder{Name:" "}); !errors.Is(err, connection.ErrMissingFolderName) { t.Fatalf("期望分组名称校验错误，得到 %v", err) }
}

func TestReorderDelegatesOrderedIDs(t *testing.T) {
	repo := &memoryRepository{items: []connection.Folder{{ID:"f1",Name:"one"},{ID:"f2",Name:"two"},{ID:"f3",Name:"three"}}}
	s, err := NewService(repo, fixedIDs{id:"unused"})
	if err != nil { t.Fatal(err) }
	if err := s.Reorder(context.Background(), []string{"f3","f1"}); err != nil { t.Fatal(err) }
	if got, want := []string{repo.items[0].ID,repo.items[1].ID,repo.items[2].ID}, []string{"f3","f1","f2"}; !reflect.DeepEqual(got,want) {
		t.Fatalf("排序结果=%v，期望 %v",got,want)
	}
}

func TestDeleteRequiresID(t *testing.T) {
	s, err := NewService(&memoryRepository{}, fixedIDs{id:"folder-1"})
	if err != nil { t.Fatal(err) }
	if err := s.Delete(context.Background(), " "); !errors.Is(err, connection.ErrMissingFolderID) { t.Fatalf("期望分组 ID 校验错误，得到 %v", err) }
}
