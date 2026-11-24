package connectionfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuwen002/meteor-ch-workbench/internal/domain/connection"
)

func testRepository(t *testing.T) (*Repository, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "private", "connections.json")
	repo, err := New(path)
	if err != nil { t.Fatal(err) }
	return repo, path
}

func sampleConnection(id string) connection.Connection {
	return connection.Connection{ID: id, Name: "Analytics", Host: "127.0.0.1", Port: 9000, Protocol: connection.ProtocolNative, User: "default", Password: "local-secret"}
}

func TestRepositoryGet(t *testing.T) {
	repo, _ := testRepository(t)
	ctx := context.Background()
	want := sampleConnection("conn-1")
	if err := repo.Save(ctx, want); err != nil { t.Fatal(err) }
	got, err := repo.Get(ctx, want.ID)
	if err != nil { t.Fatal(err) }
	if got.ID != want.ID || got.Name != want.Name || got.Password != want.Password { t.Fatalf("读取结果不正确: %+v", got) }
	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, ErrConnectionNotFound) { t.Fatalf("期望 ErrConnectionNotFound，得到 %v", err) }
	if _, err := repo.Get(ctx, " "); !errors.Is(err, connection.ErrMissingID) { t.Fatalf("期望缺少 ID 错误，得到 %v", err) }
}

func TestRepositorySaveListAndUpdate(t *testing.T) {
	repo, path := testRepository(t)
	ctx := context.Background()
	item := sampleConnection("conn-1")
	if err := repo.Save(ctx, item); err != nil { t.Fatal(err) }
	item.Name = "Updated"
	if err := repo.Save(ctx, item); err != nil { t.Fatal(err) }
	items, err := repo.List(ctx)
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].Name != "Updated" || items[0].Password != "local-secret" { t.Fatalf("读取结果不正确: %+v", items) }
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0o600 { t.Fatalf("配置文件权限=%o，期望 600", info.Mode().Perm()) }
}

func TestRepositoryDelete(t *testing.T) {
	repo, _ := testRepository(t)
	ctx := context.Background()
	if err := repo.Save(ctx, sampleConnection("conn-1")); err != nil { t.Fatal(err) }
	if err := repo.Delete(ctx, "conn-1"); err != nil { t.Fatal(err) }
	items, err := repo.List(ctx)
	if err != nil { t.Fatal(err) }
	if len(items) != 0 { t.Fatalf("删除后仍有连接: %+v", items) }
	if err := repo.Delete(ctx, "missing"); !errors.Is(err, os.ErrNotExist) { t.Fatalf("期望 os.ErrNotExist，得到 %v", err) }
}

func TestRepositoryRejectsUnsupportedVersion(t *testing.T) {
	repo, path := testRepository(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, []byte(`{"version":99,"connections":[]}`), 0o600); err != nil { t.Fatal(err) }
	if _, err := repo.List(context.Background()); !errors.Is(err, ErrInvalidFile) { t.Fatalf("期望 ErrInvalidFile，得到 %v", err) }
}

func TestRepositoryRejectsDuplicateIDs(t *testing.T) {
	repo, path := testRepository(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { t.Fatal(err) }
	raw := `{"version":1,"connections":[{"id":"same","name":"First","host":"127.0.0.1","port":9000,"protocol":"native"},{"id":"same","name":"Second","host":"127.0.0.1","port":9000,"protocol":"native"}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil { t.Fatal(err) }
	if _, err := repo.List(context.Background()); !errors.Is(err, ErrDuplicateID) { t.Fatalf("期望 ErrDuplicateID，得到 %v", err) }
}

func TestImportLegacyJSON(t *testing.T) {
	repo, _ := testRepository(t)
	raw := []byte(`{"folders":[],"connections":[{"id":"old-1","name":"Legacy","host":"db.local","port":9000,"user":"default","password":"secret"}]}`)
	count, err := repo.ImportLegacyJSON(context.Background(), raw)
	if err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("导入数量=%d", count) }
	items, err := repo.List(context.Background())
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].Protocol != connection.ProtocolNative || items[0].Password != "secret" { t.Fatalf("迁移结果不正确: %+v", items) }
}

func TestImportLegacyJSONIsAtomicOnInvalidEntry(t *testing.T) {
	repo, _ := testRepository(t)
	raw := []byte(`{"connections":[{"id":"ok","name":"Good","host":"db","port":9000},{"id":"bad","name":"","host":"db","port":9000}]}`)
	if _, err := repo.ImportLegacyJSON(context.Background(), raw); err == nil { t.Fatal("期望无效条目导致导入失败") }
	items, err := repo.List(context.Background())
	if err != nil { t.Fatal(err) }
	if len(items) != 0 { t.Fatalf("校验失败不应写入部分连接: %+v", items) }
}

func TestRandomIDGenerator(t *testing.T) {
	g := RandomIDGenerator{}
	first, err := g.NewID()
	if err != nil { t.Fatal(err) }
	second, err := g.NewID()
	if err != nil { t.Fatal(err) }
	if len(first) != 32 || first == second { t.Fatalf("随机 ID 不符合预期: %q %q", first, second) }
}
