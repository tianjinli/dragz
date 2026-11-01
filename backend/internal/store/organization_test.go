package store

import "testing"

func TestFormatDepartmentAncestorPath(t *testing.T) {
	names := map[uint64]string{
		1: "总部",
		2: "研发中心",
		3: "销售中心",
	}

	tests := []struct {
		ancestors string
		want      string
	}{
		{ancestors: "0", want: ""},
		{ancestors: "0,1", want: "总部"},
		{ancestors: "0,1,2", want: "总部 / 研发中心"},
		{ancestors: "0,1,3", want: "总部 / 销售中心"},
	}

	for _, tt := range tests {
		if got := formatDepartmentAncestorPath(tt.ancestors, names); got != tt.want {
			t.Fatalf("formatDepartmentAncestorPath(%q) = %q, want %q", tt.ancestors, got, tt.want)
		}
	}
}
