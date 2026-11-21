package pluginapi

import "testing"

func TestSatisfiesEngineRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		engine string
		expr   string
		ok     bool
	}{
		{"1.0.0", ">=1.0.0 <2.0.0", true},
		{"1.9.9", ">=1.0.0 <2.0.0", true},
		{"2.0.0", ">=1.0.0 <2.0.0", false},
		{"0.9.0", ">=1.0.0 <2.0.0", false},
		{"1.0.0", "1.0.0", true},
		{"1.0.1", "=1.0.0", false},
		{"1.0.0", "", true}, // default range
	}
	for _, tc := range cases {
		ok, err := SatisfiesEngineRange(tc.engine, tc.expr)
		if err != nil {
			t.Fatalf("%s vs %q: %v", tc.engine, tc.expr, err)
		}
		if ok != tc.ok {
			t.Fatalf("%s vs %q: got %v want %v", tc.engine, tc.expr, ok, tc.ok)
		}
	}
}

func TestManifestValidateRejectsBuiltin(t *testing.T) {
	t.Parallel()
	m := Manifest{
		ID: "example", Name: "Example", Version: "0.1.0",
		EngineRange: DefaultEngineRange,
		Components:  []string{"/example/index"},
		Builtin:     true,
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for builtin package")
	}
}
