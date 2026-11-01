package store

import "testing"

func TestDataScopeAllowsDepartment(t *testing.T) {
	tests := []struct {
		name         string
		scope        DataScope
		departmentID uint64
		want         bool
	}{
		{name: "all", scope: DataScope{All: true}, departmentID: 99, want: true},
		{name: "custom match", scope: DataScope{DepartmentIDs: []uint64{2, 4}}, departmentID: 4, want: true},
		{name: "custom miss", scope: DataScope{DepartmentIDs: []uint64{2, 4}}, departmentID: 5, want: false},
		{name: "empty", scope: DataScope{}, departmentID: 1, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.scope.AllowsDepartment(test.departmentID); got != test.want {
				t.Fatalf("AllowsDepartment(%d) = %v, want %v", test.departmentID, got, test.want)
			}
		})
	}
}
