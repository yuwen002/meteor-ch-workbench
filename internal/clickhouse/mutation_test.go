package clickhouse

import "testing"

func TestEnsureMutationSync(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "ALTER UPDATE 追加 mutations_sync=2",
			in:   "ALTER TABLE db.t UPDATE col = 'x' WHERE id = 1",
			want: "ALTER TABLE db.t UPDATE col = 'x' WHERE id = 1 SETTINGS mutations_sync = 2",
		},
		{
			name: "ALTER UPDATE 末尾带分号时先去掉分号再追加",
			in:   "ALTER TABLE db.t UPDATE col = 'x' WHERE id = 1;",
			want: "ALTER TABLE db.t UPDATE col = 'x' WHERE id = 1 SETTINGS mutations_sync = 2",
		},
		{
			name: "ALTER DELETE 同样追加",
			in:   "ALTER TABLE db.t DELETE WHERE id = 1",
			want: "ALTER TABLE db.t DELETE WHERE id = 1 SETTINGS mutations_sync = 2",
		},
		{
			name: "独立 DELETE FROM 追加",
			in:   "DELETE FROM db.t WHERE id = 1",
			want: "DELETE FROM db.t WHERE id = 1 SETTINGS mutations_sync = 2",
		},
		{
			name: "小写语句同样识别",
			in:   "alter table db.t update col = 1 where id = 1",
			want: "alter table db.t update col = 1 where id = 1 SETTINGS mutations_sync = 2",
		},
		{
			name: "前置行注释不影响识别",
			in:   "-- 改一行\nALTER TABLE db.t UPDATE col = 1 WHERE id = 1",
			want: "-- 改一行\nALTER TABLE db.t UPDATE col = 1 WHERE id = 1 SETTINGS mutations_sync = 2",
		},
		{
			name: "前置块注释不影响识别",
			in:   "/* note */ ALTER TABLE db.t UPDATE col = 1 WHERE id = 1",
			want: "/* note */ ALTER TABLE db.t UPDATE col = 1 WHERE id = 1 SETTINGS mutations_sync = 2",
		},
		{
			name: "已显式指定 mutations_sync 时原样返回",
			in:   "ALTER TABLE db.t UPDATE col = 1 WHERE id = 1 SETTINGS mutations_sync = 0",
			want: "ALTER TABLE db.t UPDATE col = 1 WHERE id = 1 SETTINGS mutations_sync = 0",
		},
		{
			name: "SELECT 不追加",
			in:   "SELECT * FROM db.t",
			want: "SELECT * FROM db.t",
		},
		{
			name: "INSERT 不追加",
			in:   "INSERT INTO db.t VALUES (1)",
			want: "INSERT INTO db.t VALUES (1)",
		},
		{
			name: "CREATE 不追加",
			in:   "CREATE TABLE db.t (id Int32) ENGINE = MergeTree ORDER BY id",
			want: "CREATE TABLE db.t (id Int32) ENGINE = MergeTree ORDER BY id",
		},
		{
			name: "ALTER 其他类型（ADD COLUMN）不追加",
			in:   "ALTER TABLE db.t ADD COLUMN c Int32",
			want: "ALTER TABLE db.t ADD COLUMN c Int32",
		},
		{
			name: "空语句原样返回",
			in:   "",
			want: "",
		},
		{
			name: "只有注释的语句原样返回",
			in:   "-- 只是注释",
			want: "-- 只是注释",
		},
		{
			name: "字段名含 update 字样但非 mutation 不误判",
			in:   "SELECT update_time FROM db.t",
			want: "SELECT update_time FROM db.t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ensureMutationSync(tt.in); got != tt.want {
				t.Errorf("ensureMutationSync(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
