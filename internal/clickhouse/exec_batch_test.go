package clickhouse

import (
	"context"
	"errors"
	"testing"
)

func TestExecBatchWith(t *testing.T) {
	tests := []struct {
		name            string
		statements      []string
		continueOnError bool
		failAt          map[int]string
		calls           []string
		wantOk          int
		wantFailed      int
		wantSkipped     int
		wantErrAt       map[int]string
	}{
		{
			name:            "空列表：全部计数为 0",
			statements:      nil,
			continueOnError: false,
			calls:           []string{},
			wantOk:          0,
			wantFailed:      0,
			wantSkipped:     0,
		},
		{
			name:            "全部成功",
			statements:      []string{"A", "B", "C"},
			continueOnError: false,
			calls:           []string{"A", "B", "C"},
			wantOk:          3,
			wantFailed:      0,
			wantSkipped:     0,
		},
		{
			name:            "单条成功",
			statements:      []string{"A"},
			continueOnError: false,
			calls:           []string{"A"},
			wantOk:          1,
		},
		{
			name:            "中间失败即中断：后续标记 skipped 且不再执行",
			statements:      []string{"A", "B", "C", "D"},
			continueOnError: false,
			failAt:          map[int]string{1: "boom"},
			calls:           []string{"A", "B"},
			wantOk:          1,
			wantFailed:      1,
			wantSkipped:     2,
			wantErrAt:       map[int]string{1: "boom"},
		},
		{
			name:            "continueOnError：继续执行并收集全部失败",
			statements:      []string{"A", "B", "C", "D"},
			continueOnError: true,
			failAt:          map[int]string{1: "boom1", 2: "boom2"},
			calls:           []string{"A", "B", "C", "D"},
			wantOk:          2,
			wantFailed:      2,
			wantSkipped:     0,
			wantErrAt:       map[int]string{1: "boom1", 2: "boom2"},
		},
		{
			name:            "首条失败即中断：全部后续 skipped",
			statements:      []string{"A", "B", "C"},
			continueOnError: false,
			failAt:          map[int]string{0: "first"},
			calls:           []string{"A"},
			wantOk:          0,
			wantFailed:      1,
			wantSkipped:     2,
			wantErrAt:       map[int]string{0: "first"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			exec := func(_ context.Context, sqlText string) (int64, error) {
				calls = append(calls, sqlText)
				for i, s := range tc.statements {
					if s != sqlText {
						continue
					}
					if msg, ok := tc.failAt[i]; ok {
						return 0, errors.New(msg)
					}
					break
				}
				return 1, nil
			}

			s := &Service{}
			got, err := s.execBatchWith(tc.statements, tc.continueOnError, exec)
			if err != nil {
				t.Fatalf("execBatchWith 返回意外 error: %v", err)
			}
			if got.Ok != tc.wantOk {
				t.Errorf("Ok = %d, 期望 %d", got.Ok, tc.wantOk)
			}
			if got.Failed != tc.wantFailed {
				t.Errorf("Failed = %d, 期望 %d", got.Failed, tc.wantFailed)
			}
			if got.Skipped != tc.wantSkipped {
				t.Errorf("Skipped = %d, 期望 %d", got.Skipped, tc.wantSkipped)
			}
			if len(calls) != len(tc.calls) {
				t.Errorf("实际执行 %v, 期望 %v", calls, tc.calls)
			}
			for i := range tc.calls {
				if i < len(calls) && calls[i] != tc.calls[i] {
					t.Errorf("第 %d 条执行 %q, 期望 %q", i, calls[i], tc.calls[i])
				}
			}
			if len(got.Statements) != len(tc.statements) {
				t.Errorf("Statements 长度 = %d, 期望 %d", len(got.Statements), len(tc.statements))
			}
			for i, st := range got.Statements {
				if st.Index != i {
					t.Errorf("第 %d 条 Index = %d, 期望 %d", i, st.Index, i)
				}
				if i < len(tc.statements) && st.SQL != tc.statements[i] {
					t.Errorf("第 %d 条 SQL = %q, 期望 %q", i, st.SQL, tc.statements[i])
				}
				if want, ok := tc.wantErrAt[i]; ok {
					if st.Error != want {
						t.Errorf("第 %d 条 Error = %q, 期望 %q", i, st.Error, want)
					}
				} else if st.Error != "" {
					t.Errorf("第 %d 条不应有 Error, 实际 %q", i, st.Error)
				}
			}
		})
	}
}

func TestExecBatchWithNilInput(t *testing.T) {
	s := &Service{}
	exec := func(_ context.Context, _ string) (int64, error) { return 0, nil }
	got, err := s.execBatchWith(nil, false, exec)
	if err != nil {
		t.Fatalf("返回意外 error: %v", err)
	}
	if got.Statements == nil {
		t.Error("Statements 应为空切片而非 nil，避免前端序列化为 null")
	}
	if got.ElapsedMs < 0 {
		t.Errorf("ElapsedMs 不应为负: %v", got.ElapsedMs)
	}
}
