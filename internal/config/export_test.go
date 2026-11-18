// 本文件为 export.go 的 5 种格式渲染函数补单元测试。
// 重点覆盖：脱敏开关、注册表元信息、特殊字符转义、以及「导出→再解析」的闭环一致性。
package config

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
)

// sampleData 是测试用固定样本：两个分组 + 三条连接，覆盖根级连接、TLS、压缩、native 协议。
func sampleData() DataFile {
	return DataFile{
		Folders: []Folder{
			{ID: "f1", Name: "生产"},
			{ID: "f2", Name: "测试"},
		},
		Connections: []Connection{
			{
				ID: "c1", FolderID: "f1", Name: "prod-http", Host: "10.0.0.1", Port: 8123,
				Protocol: "http", User: "default", Password: "secret", Database: "analytics",
				Secure: false, UseCompress: true,
			},
			{
				ID: "c2", FolderID: "f2", Name: "test-native", Host: "127.0.0.1", Port: 9000,
				Protocol: "native", User: "tester", Password: "p@ss word!", Database: "default",
			},
			{
				ID: "c3", FolderID: "", Name: "root-tls", Host: "ch.example.com", Port: 8443,
				Protocol: "http", User: "u", Password: "", Database: "",
				Secure: true,
			},
		},
	}
}

func TestExportFormatsRegistry(t *testing.T) {
	formats := ExportFormats()
	if len(formats) != len(exportRegistry) {
		t.Fatalf("want %d formats, got %d", len(exportRegistry), len(formats))
	}
	ids := map[string]bool{}
	for _, f := range formats {
		if f.ID == "" || f.Name == "" || f.Extension == "" || f.Mime == "" {
			t.Fatalf("format has empty meta field: %+v", f)
		}
		if ids[f.ID] {
			t.Fatalf("duplicate format id: %s", f.ID)
		}
		ids[f.ID] = true
	}
	for _, want := range []string{"json", "yaml", "csv", "dsn", "dbeaver"} {
		if !ids[want] {
			t.Fatalf("missing expected format: %s", want)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	out, err := renderJSON(sampleData(), false)
	if err != nil {
		t.Fatalf("renderJSON: %v", err)
	}
	var df DataFile
	if err := json.Unmarshal([]byte(out), &df); err != nil {
		t.Fatalf("output is not valid json: %v", err)
	}
	if len(df.Connections) != 3 || len(df.Folders) != 2 {
		t.Fatalf("roundtrip lost data: %d conns, %d folders", len(df.Connections), len(df.Folders))
	}
	// 脱敏：所有密码应为空
	for _, c := range df.Connections {
		if c.Password != "" {
			t.Fatalf("password should be stripped, got %q on %s", c.Password, c.Name)
		}
	}
}

func TestRenderJSONWithPassword(t *testing.T) {
	out, err := renderJSON(sampleData(), true)
	if err != nil {
		t.Fatalf("renderJSON: %v", err)
	}
	if !strings.Contains(out, "secret") || !strings.Contains(out, `p@ss word!`) {
		t.Fatalf("includePassword=true should keep passwords, got:\n%s", out)
	}
}

func TestRenderJSONDoesNotMutateSource(t *testing.T) {
	data := sampleData()
	if _, err := renderJSON(data, false); err != nil {
		t.Fatalf("renderJSON: %v", err)
	}
	// sanitize 必须是拷贝语义，否则脱敏导出会把内存里的真实密码清掉
	if data.Connections[0].Password != "secret" {
		t.Fatalf("sanitize mutated source data: %q", data.Connections[0].Password)
	}
}

func TestRenderYAMLStructure(t *testing.T) {
	out, err := renderYAML(sampleData(), false)
	if err != nil {
		t.Fatalf("renderYAML: %v", err)
	}
	if !strings.HasPrefix(out, "# Meteor CH Workbench configuration") {
		t.Fatalf("missing yaml header:\n%s", out)
	}
	for _, want := range []string{"folders:", "connections:", "folder_id:", "use_compress:", "secure:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("yaml missing key %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("password should be stripped:\n%s", out)
	}
	if !strings.Contains(out, "compress: true") && !strings.Contains(out, "use_compress: true") {
		t.Fatalf("expected use_compress: true in yaml:\n%s", out)
	}
}

func TestRenderYAMLEmptyLists(t *testing.T) {
	out, err := renderYAML(DataFile{}, false)
	if err != nil {
		t.Fatalf("renderYAML: %v", err)
	}
	if !strings.Contains(out, "folders:\n  []") {
		t.Fatalf("empty folders should render as []:\n%s", out)
	}
	if !strings.Contains(out, "connections:\n  []") {
		t.Fatalf("empty connections should render as []:\n%s", out)
	}
}

func TestRenderYAMLRoundtrip(t *testing.T) {
	data := sampleData()
	out, err := renderYAML(data, true)
	if err != nil {
		t.Fatalf("renderYAML: %v", err)
	}
	parsed, err := parseYAML(out)
	if err != nil {
		t.Fatalf("parseYAML: %v", err)
	}
	normalizeData(&parsed)
	if len(parsed.Connections) != len(data.Connections) {
		t.Fatalf("want %d conns, got %d", len(data.Connections), len(parsed.Connections))
	}
	if len(parsed.Folders) != len(data.Folders) {
		t.Fatalf("want %d folders, got %d", len(data.Folders), len(parsed.Folders))
	}
	// 逐字段核对第一条连接，确保 yamlQuote 的转义没有破坏内容
	got, want := parsed.Connections[0], data.Connections[0]
	if got.Host != want.Host || got.Port != want.Port || got.Protocol != want.Protocol {
		t.Fatalf("conn mismatch: got %+v want %+v", got, want)
	}
	if got.Password != want.Password {
		t.Fatalf("password lost in roundtrip: got %q want %q", got.Password, want.Password)
	}
	if got.FolderID != want.FolderID {
		t.Fatalf("folder_id lost: got %q want %q", got.FolderID, want.FolderID)
	}
	if got.UseCompress != want.UseCompress {
		t.Fatalf("use_compress lost: got %v want %v", got.UseCompress, want.UseCompress)
	}
	// 第二条含特殊字符的密码必须原样还原
	if parsed.Connections[1].Password != "p@ss word!" {
		t.Fatalf("special-char password corrupted: %q", parsed.Connections[1].Password)
	}
}

func TestYAMLQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", `""`},
		{"plain", "plain"},
		{"has space", `"has space"`},
		{"a:b", `"a:b"`},
		{`say "hi"`, `"say \"hi\""`},
		{"line1\nline2", `"line1\nline2"`},
		// 反斜杠不在特殊字符集合内，因此原样输出、不加引号（YAML 中 plain scalar 合法）
		{"back\\slash", `back\slash`},
	}
	for _, tc := range cases {
		if got := yamlQuote(tc.in); got != tc.want {
			t.Fatalf("yamlQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRenderCSV(t *testing.T) {
	out, err := renderCSV(sampleData(), false)
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid csv: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("want 1 header + 3 rows, got %d records", len(records))
	}
	wantHeader := []string{
		"folder", "name", "protocol", "host", "port", "user", "password",
		"database", "secure", "use_compress",
	}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Fatalf("header[%d] = %q, want %q", i, records[0][i], h)
		}
	}
	// 第一行应展示分组名而非 ID
	if records[1][0] != "生产" {
		t.Fatalf("folder name not rendered: %q", records[1][0])
	}
	// 根级连接的 folder 列为空
	if records[3][0] != "" {
		t.Fatalf("root connection folder should be empty, got %q", records[3][0])
	}
	// 脱敏后 password 列为空
	if records[1][6] != "" {
		t.Fatalf("password should be empty, got %q", records[1][6])
	}
}

func TestRenderCSVWithPasswordAndQuoting(t *testing.T) {
	out, err := renderCSV(sampleData(), true)
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	records, _ := csv.NewReader(strings.NewReader(out)).ReadAll()
	if records[1][6] != "secret" {
		t.Fatalf("password missing: %q", records[1][6])
	}
	// 含逗号/空格的密码必须被正确转义后再还原
	if records[2][6] != "p@ss word!" {
		t.Fatalf("csv quoting broken: %q", records[2][6])
	}
}

func TestRenderCSVRoundtrip(t *testing.T) {
	out, err := renderCSV(sampleData(), true)
	if err != nil {
		t.Fatalf("renderCSV: %v", err)
	}
	parsed, err := parseCSV(out)
	if err != nil {
		t.Fatalf("parseCSV: %v", err)
	}
	normalizeData(&parsed)
	if len(parsed.Connections) != 3 {
		t.Fatalf("want 3 conns, got %d", len(parsed.Connections))
	}
	if parsed.Connections[0].Host != "10.0.0.1" || parsed.Connections[0].Port != 8123 {
		t.Fatalf("host/port lost: %+v", parsed.Connections[0])
	}
	if parsed.Connections[0].UseCompress != true {
		t.Fatalf("use_compress lost: %+v", parsed.Connections[0])
	}
	// 分组应被重建为同名文件夹
	if len(parsed.Folders) != 2 {
		t.Fatalf("want 2 folders rebuilt, got %d", len(parsed.Folders))
	}
}

func TestRenderDSN(t *testing.T) {
	out, err := renderDSN(sampleData(), true)
	if err != nil {
		t.Fatalf("renderDSN: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 dsn lines, got %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "clickhouse://") {
		t.Fatalf("http conn should use clickhouse:// scheme: %q", lines[0])
	}
	if !strings.HasPrefix(lines[2], "clickhouses://") {
		t.Fatalf("secure conn should use clickhouses:// scheme: %q", lines[2])
	}
	if !strings.Contains(lines[0], "/analytics") {
		t.Fatalf("database path missing: %q", lines[0])
	}
	if !strings.Contains(lines[0], "compress=true") {
		t.Fatalf("compress flag missing: %q", lines[0])
	}
	if !strings.Contains(lines[1], "protocol=native") {
		t.Fatalf("native protocol flag missing: %q", lines[1])
	}
}

func TestRenderDSNPasswordEscaping(t *testing.T) {
	out, err := renderDSN(sampleData(), true)
	if err != nil {
		t.Fatalf("renderDSN: %v", err)
	}
	// 原始密码 "p@ss word!" 必须经过 URL 转义后才能出现在 URI 中
	if strings.Contains(out, "p@ss word!") {
		t.Fatalf("password should be url-escaped:\n%s", out)
	}
	if !strings.Contains(out, "p%40ss") {
		t.Fatalf("expected url-escaped password in output:\n%s", out)
	}
}

func TestRenderDSNStripPassword(t *testing.T) {
	out, err := renderDSN(sampleData(), false)
	if err != nil {
		t.Fatalf("renderDSN: %v", err)
	}
	if strings.Contains(out, "p%40ss") || strings.Contains(out, "secret") {
		t.Fatalf("password should be stripped:\n%s", out)
	}
	// 用户名仍应保留
	if !strings.Contains(out, "default@") {
		t.Fatalf("user should remain when password stripped:\n%s", out)
	}
	// 无用户无密码的连接不应出现多余的 @
	noCred, err := renderDSN(DataFile{Connections: []Connection{
		{Name: "anon", Host: "anon.example.com", Port: 8123, Database: "d"},
	}}, false)
	if err != nil {
		t.Fatalf("renderDSN: %v", err)
	}
	if strings.Contains(noCred, "@") {
		t.Fatalf("userless conn should have no @ segment: %q", strings.TrimSpace(noCred))
	}
}

func TestRenderDSNRoundtrip(t *testing.T) {
	out, err := renderDSN(sampleData(), true)
	if err != nil {
		t.Fatalf("renderDSN: %v", err)
	}
	parsed, err := parseDSN(out)
	if err != nil {
		t.Fatalf("parseDSN: %v", err)
	}
	normalizeData(&parsed)
	if len(parsed.Connections) != 3 {
		t.Fatalf("want 3 conns, got %d", len(parsed.Connections))
	}
	got := parsed.Connections[0]
	if got.Host != "10.0.0.1" || got.Port != 8123 || got.Database != "analytics" {
		t.Fatalf("dsn roundtrip mismatch: %+v", got)
	}
	if got.User != "default" || got.Password != "secret" {
		t.Fatalf("credentials lost in dsn roundtrip: %+v", got)
	}
	if got.UseCompress != true {
		t.Fatalf("compress lost in dsn roundtrip: %+v", got)
	}
	if parsed.Connections[1].Protocol != "native" {
		t.Fatalf("native protocol lost: %+v", parsed.Connections[1])
	}
	if parsed.Connections[2].Secure != true {
		t.Fatalf("secure lost: %+v", parsed.Connections[2])
	}
}

func TestRenderDBeaver(t *testing.T) {
	out, err := renderDBeaver(sampleData(), true)
	if err != nil {
		t.Fatalf("renderDBeaver: %v", err)
	}
	var root struct {
		Version     string           `json:"version"`
		Connections []map[string]any `json:"connections"`
		Folders     []Folder         `json:"folders"`
	}
	if err := json.Unmarshal([]byte(out), &root); err != nil {
		t.Fatalf("output is not valid json: %v", err)
	}
	if root.Version != "1.0" {
		t.Fatalf("version = %q, want 1.0", root.Version)
	}
	if len(root.Connections) != 3 {
		t.Fatalf("want 3 connections, got %d", len(root.Connections))
	}
	// JDBC URL 与端口推断
	if got := root.Connections[0]["url"]; got != "jdbc:clickhouse://10.0.0.1:8123/analytics" {
		t.Fatalf("jdbc url = %v", got)
	}
	if got := root.Connections[1]["port"]; got != "9000" {
		t.Fatalf("native port = %v, want 9000", got)
	}
	// properties 反映压缩与 TLS
	props0, _ := root.Connections[0]["properties"].(map[string]any)
	if props0 == nil || props0["compress"] != "1" {
		t.Fatalf("compress property missing: %v", props0)
	}
	props2, _ := root.Connections[2]["properties"].(map[string]any)
	if props2 == nil || props2["ssl"] != "true" {
		t.Fatalf("ssl property missing: %v", props2)
	}
	// 分组名映射
	if got := root.Connections[0]["folder"]; got != "生产" {
		t.Fatalf("folder name = %v, want 生产", got)
	}
	// 根级连接应省略 folder 字段
	if _, ok := root.Connections[2]["folder"]; ok {
		t.Fatalf("root connection should omit folder field")
	}
}

func TestRenderDBeaverStripPassword(t *testing.T) {
	out, err := renderDBeaver(sampleData(), false)
	if err != nil {
		t.Fatalf("renderDBeaver: %v", err)
	}
	if strings.Contains(out, "secret") || strings.Contains(out, "p@ss word!") {
		t.Fatalf("password should be stripped:\n%s", out)
	}
}

func TestRenderDBeaverDefaultPorts(t *testing.T) {
	// Port 为 0 时按协议回落到 8123 / 9000
	data := DataFile{Connections: []Connection{
		{Name: "a", Host: "h1", Protocol: "http", Database: "d"},
		{Name: "b", Host: "h2", Protocol: "native", Database: "d"},
	}}
	out, err := renderDBeaver(data, false)
	if err != nil {
		t.Fatalf("renderDBeaver: %v", err)
	}
	if !strings.Contains(out, "jdbc:clickhouse://h1:8123/d") {
		t.Fatalf("http default port should be 8123:\n%s", out)
	}
	if !strings.Contains(out, "jdbc:clickhouse://h2:9000/d") {
		t.Fatalf("native default port should be 9000:\n%s", out)
	}
}

func TestRenderDBeaverRoundtrip(t *testing.T) {
	out, err := renderDBeaver(sampleData(), true)
	if err != nil {
		t.Fatalf("renderDBeaver: %v", err)
	}
	parsed, err := parseJSONOrDBeaver(out)
	if err != nil {
		t.Fatalf("parse back: %v", err)
	}
	normalizeData(&parsed)
	if len(parsed.Connections) != 3 {
		t.Fatalf("want 3 conns, got %d", len(parsed.Connections))
	}
	if len(parsed.Folders) != 2 {
		t.Fatalf("want 2 folders, got %d", len(parsed.Folders))
	}
	got := parsed.Connections[1]
	if got.Host != "127.0.0.1" || got.Port != 9000 || got.Protocol != "native" {
		t.Fatalf("dbeaver roundtrip mismatch: %+v", got)
	}
	if got.Password != "p@ss word!" {
		t.Fatalf("password lost: %q", got.Password)
	}
}

// indexOfHeader 返回表头中指定列的索引，找不到返回 -1。
func indexOfHeader(header []string, name string) int {
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(h), name) {
			return i
		}
	}
	return -1
}
