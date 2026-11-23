package wailsadapter

import "testing"

func TestIsDialogCancelled(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "空错误", err: nil, want: false},
		{name: "用户取消", err: errString("dialog cancelled"), want: true},
		{name: "大小写不敏感", err: errString("Dialog CANCELLED by user"), want: true},
		{name: "其他错误", err: errString("permission denied"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDialogCancelled(tt.err); got != tt.want {
				t.Fatalf("isDialogCancelled(%v) = %v，期望 %v", tt.err, got, tt.want)
			}
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }
