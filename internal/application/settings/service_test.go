package settings

import (
	"context"
	"errors"
	"testing"

	domain "github.com/yuwen002/meteor-ch-workbench/internal/domain/settings"
)

type memoryRepository struct { value domain.Preferences; loadErr, saveErr error; loads, saves int }
func (r *memoryRepository) Load(context.Context) (domain.Preferences, error) { r.loads++; return r.value, r.loadErr }
func (r *memoryRepository) Save(_ context.Context, value domain.Preferences) error { r.saves++; if r.saveErr != nil { return r.saveErr }; r.value = value; return nil }

func TestServiceCachesLoadedPreferences(t *testing.T) {
	repo := &memoryRepository{value: domain.Preferences{QueryRowLimit: 6000, EditorRatio: 0.7}}
	service, err := NewService(repo); if err != nil { t.Fatal(err) }
	for i:=0;i<2;i++ { got, err := service.Load(context.Background()); if err != nil { t.Fatal(err) }; if got.QueryRowLimit != 6000 { t.Fatalf("unexpected preferences: %+v", got) } }
	if repo.loads != 1 { t.Fatalf("expected one repository load, got %d", repo.loads) }
}

func TestServiceSaveNormalizesAndCaches(t *testing.T) {
	repo := &memoryRepository{}
	service, _ := NewService(repo)
	got, err := service.Save(context.Background(), domain.Preferences{QueryRowLimit: -1, EditorRatio: 0.9})
	if err != nil { t.Fatal(err) }
	if got.QueryRowLimit != domain.DefaultRowLimit || got.EditorRatio != domain.DefaultEditorRatio { t.Fatalf("not normalized: %+v", got) }
	loaded, err := service.Load(context.Background()); if err != nil { t.Fatal(err) }
	if loaded != got || repo.loads != 0 { t.Fatalf("saved value should be cached: %+v", loaded) }
}

func TestServiceDoesNotCacheFailedSave(t *testing.T) {
	repo := &memoryRepository{saveErr: errors.New("disk full")}
	service, _ := NewService(repo)
	if _, err := service.Save(context.Background(), domain.Default()); err == nil { t.Fatal("expected save error") }
	if service.loaded { t.Fatal("failed save must not mark cache loaded") }
}
