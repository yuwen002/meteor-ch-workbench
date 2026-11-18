// 本文件为 import.go 的解析函数与合并策略补单元测试。
// 重点覆盖 roadmap 中列出的边界：DBeaver 结构、CSV 缺列、DSN 非法行、
// 以及合并模式下的同名分组复用、ID 冲突重分配、folder_id 重映射。
package config

import (
	"strings"
	"testing"
)

// ---- 格式注册与自动识别 ----

func TestImportFormatsRegistry(t *testing.T) {
	formats := ImportFormats()
	if len(formats) != len(importRegistry) {
		t.Fatalf("want %d formats, got %d", len(importRegistry), len(formats))
	}
	ids := map[string]bool{}
	for _, f := range formats {
		if f.ID == "" || f.Name == "" || f.Extension == "" {
			t.Fatalf("format has empty meta field: %+v", f)
		}
		ids[f.ID] = true
	}
	for _, want := range []string{"json", "yaml", "csv", "dsn"} {
		if !ids[want] {
			t.Fatalf("missing expected import format: %s", want)
		}
	}
}

func TestParseByFormatExplicit(t *testing.T) {
	raw := "clickhouse://default:secret@h1:8123/db1\n"
	df, format, err := parseByFormat(raw, "dsn")
	if err != nil {
		t.Fatalf("parseByFormat: %v", err)
	}
	if format != "dsn" {
		t.Fatalf("format = %q, want dsn", format)
	}
	if len(df.Connections) != 1 || df.Connections[0].Host != "h1" {
		t.Fatalf("unexpected parse result: %+v", df.Connections)
	}
}

func TestParseByFormatUnsupported(t *testing.T) {
	if _, _, err := parseByFormat("anything", "xml"); err == nil {
		t.Fatal("expected unsupported format error")
	}
}

func TestParseByFormatAutoDetectPrefersDsn(t *testing.T) {
	// CSV 解析器对任意单行文本都会成功（只要表头齐全），因此自动识别依赖注册顺序。
	raw := "# comment\nclickhouse://u@h1:8123/db\n\nclickhouses://h2:8443\n"
	df, format, err := parseByFormat(raw, "")
	if err != nil {
		t.Fatalf("auto detect: %v", err)
	}
	if format != "dsn" {
		t.Fatalf("format = %q, want dsn", format)
	}
	if len(df.Connections) != 2 {
		t.Fatalf("want 2 conns, got %d", len(df.Connections))
	}
	if !df.Connections[1].Secure {
		t.Fatalf("clickhouses scheme should set Secure")
	}
}

func TestParseByFormatAutoDetectNoData(t *testing.T) {
	if _, _, err := parseByFormat("   \n  \n", ""); err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
	if _, _, err := parseByFormat("random non config text", ""); err == nil {
		t.Fatal("expected error for unrecognizable text")
	}
}

// ---- JSON / DBeaver ----

func TestParseJSONNativeStructure(t *testing.T) {
	raw := `{"folders":[{"id":"f1","name":"A"}],"connections":[{"id":"c1","folder_id":"f1","name":"n","host":"h","port":9000,"protocol":"native"}]}`
	df, err := parseJSONOrDBeaver(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(df.Connections) != 1 || df.Connections[0].FolderID != "f1" {
		t.Fatalf("unexpected: %+v", df)
	}
}

func TestParseJSONInvalid(t *testing.T) {
	if _, err := parseJSONOrDBeaver("{not json"); err == nil {
		t.Fatal("expected invalid json error")
	}
}

func TestParseJSONMissingConnectionsField(t *testing.T) {
	if _, err := parseJSONOrDBeaver(`{"folders":[]}`); err == nil {
		t.Fatal("expected missing connections field error")
	}
}

func TestParseJSONEmptyConnections(t *testing.T) {
	// 有 connections 字段但内容为空，本工具结构解析出的 Host 为空，会落到 DBeaver 分支
	if _, err := parseJSONOrDBeaver(`{"connections":[]}`); err == nil {
		t.Fatal("expected no usable connections error")
	}
}

func TestParseDBeaverFromURLOnly(t *testing.T) {
	// Host/Port/Database 全部留空，只靠 jdbc url 推导——这是 DBeaver 实际导出的常见形态。
	// 注意 port 用数字、host 用 url，同时覆盖「jdbc: 前缀」与「port 类型宽松」两个兼容点。
	raw := `{"connections":[{"name":"from-url","url":"jdbc:clickhouse://db.example.com:9443/warehouse","port":9443,"folder":"Prod","user":"u","password":"p","properties":{"ssl":"true","compress":"1"}}]}`
	df, err := parseJSONOrDBeaver(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(df.Connections) != 1 {
		t.Fatalf("want 1 conn, got %d", len(df.Connections))
	}
	c := df.Connections[0]
	if c.Host != "db.example.com" || c.Port != 9443 || c.Database != "warehouse" {
		t.Fatalf("url parts not parsed: %+v", c)
	}
	if !c.Secure || !c.UseCompress {
		t.Fatalf("properties not applied: %+v", c)
	}
	if len(df.Folders) != 1 || df.Folders[0].Name != "Prod" || c.FolderID != df.Folders[0].ID {
		t.Fatalf("folder not rebuilt: %+v", df.Folders)
	}
}

func TestParseDBeaverPortAsString(t *testing.T) {
	// 部分 DBeaver 版本把 port 导出为字符串，必须同样能解析
	raw := `{"connections":[{"name":"a","host":"h1","port":"8123"}]}`
	df, err := parseJSONOrDBeaver(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if df.Connections[0].Port != 8123 {
		t.Fatalf("string port not parsed, got %d", df.Connections[0].Port)
	}
}

func TestParseDBeaverNullPort(t *testing.T) {
	raw := `{"connections":[{"name":"a","host":"h1","port":null}]}`
	df, err := parseJSONOrDBeaver(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 端口为空时保留 0，交由 Normalize 按端口推断协议
	if df.Connections[0].Port != 0 {
		t.Fatalf("null port should be 0, got %d", df.Connections[0].Port)
	}
}

func TestParseDBeaverFolderReuse(t *testing.T) {
	// 两条连接属于同名 folder，应只创建一个文件夹
	raw := `{"connections":[
		{"name":"a","host":"h1","port":"8123","folder":"Same"},
		{"name":"b","host":"h2","port":"8123","folder":"Same"}
	]}`
	df, err := parseJSONOrDBeaver(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(df.Folders) != 1 {
		t.Fatalf("want 1 folder, got %d", len(df.Folders))
	}
	if df.Connections[0].FolderID != df.Connections[1].FolderID {
		t.Fatalf("both conns should share folder id")
	}
}

func TestParseDBeaverSkipsHostlessEntries(t *testing.T) {
	// 既无 host 也无可用 url 的条目应被跳过，而不是产生空主机连接
	raw := `{"connections":[
		{"name":"ghost"},
		{"name":"real","host":"h1","port":"8123"}
	]}`
	df, err := parseJSONOrDBeaver(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(df.Connections) != 1 || df.Connections[0].Host != "h1" {
		t.Fatalf("hostless entry not skipped: %+v", df.Connections)
	}
}

func TestParseDBeaverNameFallsBackToHost(t *testing.T) {
	raw := `{"connections":[{"host":"h1","port":"8123"}]}`
	df, err := parseJSONOrDBeaver(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if df.Connections[0].Name != "h1" {
		t.Fatalf("name should fall back to host, got %q", df.Connections[0].Name)
	}
}

func TestParseDBeaverAllHostlessErrors(t *testing.T) {
	if _, err := parseJSONOrDBeaver(`{"connections":[{"name":"ghost"}]}`); err == nil {
		t.Fatal("expected no usable dbeaver connections error")
	}
}

// ---- YAML ----

func TestParseYAMLEdgeCases(t *testing.T) {
	raw := `# comment line
folders:
  - id: f1
    name: "A B"
connections:
  - id: c1
    folder_id: f1
    name: 'quoted'
    host: h1
    port: not-a-number
    secure: yes
    use_compress: 1
`
	df, err := parseYAML(raw)
	if err != nil {
		t.Fatalf("parseYAML: %v", err)
	}
	if len(df.Folders) != 1 || df.Folders[0].Name != "A B" {
		t.Fatalf("folder: %+v", df.Folders)
	}
	c := df.Connections[0]
	if c.Name != "quoted" {
		t.Fatalf("single-quote not stripped: %q", c.Name)
	}
	// 非法端口应保留为 0，后续 Normalize 会按端口推断协议
	if c.Port != 0 {
		t.Fatalf("invalid port should be 0, got %d", c.Port)
	}
	if !c.Secure || !c.UseCompress {
		t.Fatalf("bool parsing failed: secure=%v compress=%v", c.Secure, c.UseCompress)
	}
}

func TestParseYAMLNotConfig(t *testing.T) {
	if _, err := parseYAML("just some text\nwith: colons"); err == nil {
		t.Fatal("expected not a yaml config error")
	}
}

func TestParseYAMLEmptySections(t *testing.T) {
	if _, err := parseYAML("folders:\nconnections:\n"); err == nil {
		t.Fatal("expected yaml has no data error")
	}
}

func TestParseYAMLScalar(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"a\nb"`, "a\nb"},
		{`"say \"hi\""`, `say "hi"`},
		{`'it''s'`, "it's"},
		{"plain", "plain"},
		{``, ``},
		{`"unterminated`, `"unterminated`},
	}
	for _, tc := range cases {
		if got := parseYAMLScalar(tc.in); got != tc.want {
			t.Fatalf("parseYAMLScalar(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ---- CSV ----

func TestParseCSVMissingRequiredColumns(t *testing.T) {
	for _, missing := range []string{"name", "host", "port"} {
		header := []string{"folder", "name", "host", "port", "user"}
		kept := make([]string, 0, len(header))
		for _, h := range header {
			if h != missing {
				kept = append(kept, h)
			}
		}
		raw := strings.Join(kept, ",") + "\n" + strings.Join(make([]string, len(kept)), ",") + "\n"
		if _, err := parseCSV(raw); err == nil {
			t.Fatalf("expected missing column error for %q", missing)
		} else if !strings.Contains(err.Error(), missing) {
			t.Fatalf("error should mention %q, got %v", missing, err)
		}
	}
}

func TestParseCSVHeaderCaseAndWhitespace(t *testing.T) {
	raw := " Folder , NAME , Host , PORT \nProd,n1,h1,8123\n"
	df, err := parseCSV(raw)
	if err != nil {
		t.Fatalf("parseCSV: %v", err)
	}
	if len(df.Connections) != 1 || df.Connections[0].Name != "n1" {
		t.Fatalf("header normalization failed: %+v", df.Connections)
	}
}

func TestParseCSVShortRowsAndBlankLines(t *testing.T) {
	// 行字段数少于表头时不应 panic，缺失列按空值处理
	raw := "folder,name,host,port,user,password,database,secure,use_compress\n" +
		"Prod,only-name,h1,8123\n" +
		"\n" +
		",,,,,,,,\n" +
		"Prod,n2,h2,9000,admin,,\n"
	df, err := parseCSV(raw)
	if err != nil {
		t.Fatalf("parseCSV: %v", err)
	}
	if len(df.Connections) != 2 {
		t.Fatalf("want 2 usable conns, got %d", len(df.Connections))
	}
	if df.Connections[0].Database != "" || df.Connections[0].User != "" {
		t.Fatalf("missing columns should be empty: %+v", df.Connections[0])
	}
}

func TestParseCSVNameFallsBackToHost(t *testing.T) {
	raw := "name,host,port\n,h1,8123\n"
	df, err := parseCSV(raw)
	if err != nil {
		t.Fatalf("parseCSV: %v", err)
	}
	if df.Connections[0].Name != "h1" {
		t.Fatalf("name should fall back to host, got %q", df.Connections[0].Name)
	}
}

func TestParseCSVInvalid(t *testing.T) {
	// 引号未闭合是 csv 包的硬错误
	if _, err := parseCSV("name,host,port\n\"broken,,\n"); err == nil {
		t.Fatal("expected invalid csv error")
	}
}

func TestParseCSVNoUsableRows(t *testing.T) {
	raw := "name,host,port\n"
	if _, err := parseCSV(raw); err == nil {
		t.Fatal("expected csv has no usable connections error")
	}
}

// ---- DSN ----

func TestParseDSNInvalidLine(t *testing.T) {
	// 非 clickhouse 前缀的行是硬错误，不会静默跳过
	if _, err := parseDSN("clickhouse://h1:8123\nhttp://h2:8123\n"); err == nil {
		t.Fatal("expected error for non-clickhouse line")
	} else if !strings.Contains(err.Error(), "http://h2:8123") {
		t.Fatalf("error should quote the offending line, got %v", err)
	}
}

func TestParseDSNEmpty(t *testing.T) {
	if _, err := parseDSN("\n\n# only comments\n"); err == nil {
		t.Fatal("expected dsn has no entries error")
	}
}

func TestParseDSNDefaultPortAndNoPassword(t *testing.T) {
	df, err := parseDSN("clickhouse://user@h1/db\n")
	if err != nil {
		t.Fatalf("parseDSN: %v", err)
	}
	c := df.Connections[0]
	if c.Port != 8123 {
		t.Fatalf("missing port should default to 8123, got %d", c.Port)
	}
	if c.User != "user" || c.Password != "" {
		t.Fatalf("user without password: %+v", c)
	}
	if c.Database != "db" {
		t.Fatalf("database = %q", c.Database)
	}
}

func TestParseDSNEncodedCredentials(t *testing.T) {
	df, err := parseDSN("clickhouse://my%20user:p%40ss%20word@h1:8123\n")
	if err != nil {
		t.Fatalf("parseDSN: %v", err)
	}
	c := df.Connections[0]
	if c.User != "my user" || c.Password != "p@ss word" {
		t.Fatalf("url decoding failed: user=%q pass=%q", c.User, c.Password)
	}
}

func TestParseBool(t *testing.T) {
	for _, s := range []string{"true", "TRUE", "1", "yes", "on", "y", "t", " true "} {
		if !parseBool(s) {
			t.Fatalf("parseBool(%q) should be true", s)
		}
	}
	for _, s := range []string{"false", "0", "no", "off", "", "anything"} {
		if parseBool(s) {
			t.Fatalf("parseBool(%q) should be false", s)
		}
	}
}

// ---- normalizeData ----

func TestNormalizeData(t *testing.T) {
	df := DataFile{
		Folders: []Folder{{Name: "no-id"}},
		Connections: []Connection{
			{Name: "missing-id", Host: "h1", Port: 9000},                   // 缺 ID，端口应推断出 native
			{Name: "dangling", Host: "h2", FolderID: "nonexistent-folder"}, // 悬空 folder_id
		},
	}
	normalizeData(&df)
	if df.Folders[0].ID == "" {
		t.Fatal("folder id not generated")
	}
	if df.Connections[0].ID == "" {
		t.Fatal("connection id not generated")
	}
	if df.Connections[0].Protocol != "native" {
		t.Fatalf("protocol should be inferred as native, got %q", df.Connections[0].Protocol)
	}
	if df.Connections[1].FolderID != "" {
		t.Fatalf("dangling folder_id should be cleared, got %q", df.Connections[1].FolderID)
	}
}

func TestNormalizeDataNilSlices(t *testing.T) {
	df := DataFile{}
	normalizeData(&df)
	if df.Folders == nil || df.Connections == nil {
		t.Fatal("nil slices should be normalized to empty slices")
	}
}

// ---- 合并模式 ----

func TestMergeDataReusesSameNameFolder(t *testing.T) {
	cur := DataFile{
		Folders:     []Folder{{ID: "cur-f", Name: "Prod"}},
		Connections: []Connection{{ID: "cur-c", FolderID: "cur-f", Name: "existing", Host: "h0"}},
	}
	incoming := DataFile{
		Folders:     []Folder{{ID: "in-f", Name: "Prod"}},
		Connections: []Connection{{ID: "in-c", FolderID: "in-f", Name: "new", Host: "h1"}},
	}
	out := mergeData(cur, incoming)
	if len(out.Folders) != 1 {
		t.Fatalf("same-name folder should be reused, got %d folders", len(out.Folders))
	}
	if len(out.Connections) != 2 {
		t.Fatalf("want 2 conns, got %d", len(out.Connections))
	}
	// 新连接应映射到已有的 cur-f
	if out.Connections[1].FolderID != "cur-f" {
		t.Fatalf("folder_id not remapped: got %q want cur-f", out.Connections[1].FolderID)
	}
}

func TestMergeDataResolvesFolderIDConflict(t *testing.T) {
	// 同名但 ID 冲突：复用已有文件夹，不给新 ID
	cur := DataFile{Folders: []Folder{{ID: "same-id", Name: "A"}}}
	incoming := DataFile{
		Folders:     []Folder{{ID: "same-id", Name: "A"}},
		Connections: []Connection{{Name: "c", Host: "h1", FolderID: "same-id"}},
	}
	out := mergeData(cur, incoming)
	if len(out.Folders) != 1 || out.Folders[0].ID != "same-id" {
		t.Fatalf("folder should be reused: %+v", out.Folders)
	}
	if out.Connections[0].FolderID != "same-id" {
		t.Fatalf("folder_id = %q", out.Connections[0].FolderID)
	}
}

func TestMergeDataRegeneratesConflictingFolderID(t *testing.T) {
	// ID 冲突但名称不同：必须重分配 ID，否则两个文件夹共用同一 ID
	cur := DataFile{Folders: []Folder{{ID: "dup", Name: "Old"}}}
	incoming := DataFile{
		Folders:     []Folder{{ID: "dup", Name: "Brand New"}},
		Connections: []Connection{{Name: "c", Host: "h1", FolderID: "dup"}},
	}
	out := mergeData(cur, incoming)
	if len(out.Folders) != 2 {
		t.Fatalf("want 2 folders, got %d", len(out.Folders))
	}
	newFolder := out.Folders[1]
	if newFolder.ID == "dup" {
		t.Fatal("conflicting folder id should be regenerated")
	}
	if newFolder.Name != "Brand New" {
		t.Fatalf("folder name = %q", newFolder.Name)
	}
	// 连接应指向重新分配后的新 ID
	if out.Connections[0].FolderID != newFolder.ID {
		t.Fatalf("folder_id not remapped: got %q want %q", out.Connections[0].FolderID, newFolder.ID)
	}
}

func TestMergeDataRegeneratesConflictingConnID(t *testing.T) {
	cur := DataFile{Connections: []Connection{{ID: "dup", Name: "old", Host: "h0"}}}
	incoming := DataFile{Connections: []Connection{{ID: "dup", Name: "new", Host: "h1"}}}
	out := mergeData(cur, incoming)
	if len(out.Connections) != 2 {
		t.Fatalf("want 2 conns, got %d", len(out.Connections))
	}
	if out.Connections[1].ID == "dup" {
		t.Fatal("conflicting connection id should be regenerated")
	}
	if out.Connections[0].ID != "dup" {
		t.Fatal("existing connection id should be preserved")
	}
}

func TestMergeDataGeneratesMissingIDs(t *testing.T) {
	incoming := DataFile{
		Folders:     []Folder{{Name: "no-id"}},
		Connections: []Connection{{Host: "h1", FolderID: ""}},
	}
	out := mergeData(DataFile{}, incoming)
	if out.Folders[0].ID == "" || out.Connections[0].ID == "" {
		t.Fatal("missing ids should be generated during merge")
	}
}

func TestMergeDataDropsUnmappedFolderID(t *testing.T) {
	// 指向不存在文件夹的连接：合并阶段保持原值，由后续 normalizeData 清理为根级。
	// 这里同时验证「不存在的 folder_id 不会被替换成随机文件夹 ID」。
	out := mergeData(DataFile{}, DataFile{
		Connections: []Connection{{Name: "c", Host: "h1", FolderID: "ghost"}},
	})
	if len(out.Connections) != 1 {
		t.Fatalf("want 1 conn, got %d", len(out.Connections))
	}
	if len(out.Folders) != 0 {
		t.Fatalf("mergeData should not invent a folder, got %d", len(out.Folders))
	}
	normalizeData(&out)
	if out.Connections[0].FolderID != "" {
		t.Fatalf("dangling folder_id should be cleared by normalizeData, got %q", out.Connections[0].FolderID)
	}
}

func TestMergeDataDoesNotMutateInputs(t *testing.T) {
	cur := DataFile{
		Folders:     []Folder{{ID: "f", Name: "A"}},
		Connections: []Connection{{ID: "c", Host: "h0"}},
	}
	incoming := DataFile{
		Folders:     []Folder{{ID: "f", Name: "B"}},
		Connections: []Connection{{ID: "c", Host: "h1"}},
	}
	out := mergeData(cur, incoming)
	if len(cur.Folders) != 1 || len(cur.Connections) != 1 {
		t.Fatal("mergeData mutated cur slices")
	}
	if len(incoming.Folders) != 1 || len(incoming.Connections) != 1 {
		t.Fatal("mergeData mutated incoming slices")
	}
	if len(out.Folders) != 2 || len(out.Connections) != 2 {
		t.Fatalf("out should hold the union: %d folders, %d conns", len(out.Folders), len(out.Connections))
	}
}

// ---- Manager 层集成 ----

// newTestManager 构造一个使用临时目录的 Manager，避免污染真实配置文件。
func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return &Manager{
		path: dir + "/connections.json",
		dir:  dir,
		data: DataFile{Folders: []Folder{}, Connections: []Connection{}},
	}
}

func TestImportDataWithOptionsReplace(t *testing.T) {
	m := newTestManager(t)
	m.data = DataFile{
		Folders:     []Folder{{ID: "f1", Name: "Old"}},
		Connections: []Connection{{ID: "c1", FolderID: "f1", Name: "old", Host: "h0"}},
	}

	res, err := m.ImportDataWithOptions("clickhouse://u@h1:8123/db1\nclickhouse://u@h2:8123/db2\n", ImportOptions{
		Format: "dsn", Mode: "replace",
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Mode != "replace" || res.Format != "dsn" {
		t.Fatalf("unexpected result meta: %+v", res)
	}
	if res.ImportedConns != 2 || res.TotalConns != 2 {
		t.Fatalf("replace should discard previous data, got %+v", res)
	}
	if res.TotalFolders != 0 {
		t.Fatalf("replace should drop old folders, got %d", res.TotalFolders)
	}
}

func TestImportDataWithOptionsMerge(t *testing.T) {
	m := newTestManager(t)
	m.data = DataFile{
		Folders:     []Folder{{ID: "f-old", Name: "Prod"}},
		Connections: []Connection{{ID: "c-old", FolderID: "f-old", Name: "existing", Host: "h0", Port: 8123}},
	}

	csvRaw := "folder,name,host,port,user,password,database,secure,use_compress\n" +
		"Prod,brand-new,h1,8123,admin,secret,db,false,true\n"
	res, err := m.ImportDataWithOptions(csvRaw, ImportOptions{Format: "csv", Mode: "merge"})
	if err != nil {
		t.Fatalf("merge import: %v", err)
	}
	if res.Mode != "merge" || res.Format != "csv" {
		t.Fatalf("unexpected meta: %+v", res)
	}
	if res.TotalConns != 2 {
		t.Fatalf("merge should keep existing + add new, got %d conns", res.TotalConns)
	}
	if res.TotalFolders != 1 {
		t.Fatalf("same-name folder should be reused, got %d folders", res.TotalFolders)
	}
	got := m.data.Connections[1]
	if got.FolderID != "f-old" {
		t.Fatalf("new conn should attach to the reused folder, got %q", got.FolderID)
	}
	if !got.UseCompress || got.Password != "secret" {
		t.Fatalf("fields lost in merge: %+v", got)
	}
}

func TestImportDataWithOptionsUnknownModeFallback(t *testing.T) {
	m := newTestManager(t)
	res, err := m.ImportDataWithOptions("clickhouse://h1:8123", ImportOptions{Format: "dsn", Mode: "whatever"})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Mode != "replace" {
		t.Fatalf("unknown mode should fall back to replace, got %q", res.Mode)
	}
}

func TestImportDataWithOptionsEmptyInput(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.ImportDataWithOptions("  \n\t ", ImportOptions{}); err == nil {
		t.Fatal("expected empty config content error")
	}
}

func TestImportDataWithOptionsAutoDetect(t *testing.T) {
	m := newTestManager(t)
	raw := "folders:\n  - id: f1\n    name: Prod\nconnections:\n  - id: c1\n    folder_id: f1\n    name: n\n    host: h1\n    port: 8123\n"
	res, err := m.ImportDataWithOptions(raw, ImportOptions{})
	if err != nil {
		t.Fatalf("auto detect import: %v", err)
	}
	if res.Format != "yaml" {
		t.Fatalf("format = %q, want yaml", res.Format)
	}
	if res.ImportedConns != 1 || res.ImportedFolders != 1 {
		t.Fatalf("unexpected counts: %+v", res)
	}
}

func TestImportDataWithOptionsPersists(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.ImportDataWithOptions("clickhouse://u@h1:8123/db", ImportOptions{Format: "dsn", Mode: "replace"}); err != nil {
		t.Fatalf("import: %v", err)
	}
	loaded, err := m.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded.Connections) != 1 || loaded.Connections[0].Host != "h1" {
		t.Fatalf("imported data not persisted: %+v", loaded.Connections)
	}
}
