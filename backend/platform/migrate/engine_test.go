package migrate

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type appliedCall struct {
	pluginID string
	version  string
	checksum string
	script   string
}

type fakeStore struct {
	applied map[string]map[string]string // pluginID → version → checksum
	calls   []appliedCall
	failOn  string // version that makes ApplyMigration fail
}

func newFakeStore() *fakeStore {
	return &fakeStore{applied: map[string]map[string]string{}}
}

func (f *fakeStore) AppliedMigrations(_ context.Context, pluginID string) (map[string]string, error) {
	out := map[string]string{}
	for version, checksum := range f.applied[pluginID] {
		out[version] = checksum
	}
	return out, nil
}

func (f *fakeStore) ApplyMigration(_ context.Context, pluginID, version, _, checksum, script string) error {
	f.calls = append(f.calls, appliedCall{pluginID: pluginID, version: version, checksum: checksum, script: script})
	if version == f.failOn {
		return errors.New("boom")
	}
	if f.applied[pluginID] == nil {
		f.applied[pluginID] = map[string]string{}
	}
	f.applied[pluginID][version] = checksum
	return nil
}

func file(version, checksum string) File {
	return File{Version: version, Name: "m" + version, SQL: "SELECT '" + version + "'", Checksum: checksum}
}

func TestRunAppliesInVersionOrder(t *testing.T) {
	store := newFakeStore()
	engine := &Engine{Store: store}

	// Deliberately unsorted input: the engine owns the ordering.
	err := engine.Run(context.Background(), Source{
		PluginID: "demo",
		Files:    []File{file("003", "c3"), file("001", "c1"), file("002", "c2")},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got []string
	for _, call := range store.calls {
		got = append(got, call.version)
	}
	if strings.Join(got, ",") != "001,002,003" {
		t.Fatalf("applied order = %v, want 001,002,003", got)
	}
}

func TestRunSkipsAppliedFiles(t *testing.T) {
	store := newFakeStore()
	store.applied["demo"] = map[string]string{"001": "c1"}
	engine := &Engine{Store: store}

	if err := engine.Run(context.Background(), Source{
		PluginID: "demo",
		Files:    []File{file("001", "c1"), file("002", "c2")},
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(store.calls) != 1 || store.calls[0].version != "002" {
		t.Fatalf("calls = %+v, want only 002", store.calls)
	}
}

// An edited, already-applied file must fail the run instead of being silently
// re-applied or ignored.
func TestRunFailsOnChecksumMismatch(t *testing.T) {
	store := newFakeStore()
	store.applied["demo"] = map[string]string{"001": "original"}
	engine := &Engine{Store: store}

	err := engine.Run(context.Background(), Source{
		PluginID: "demo",
		Files:    []File{file("001", "edited"), file("002", "c2")},
	})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v, want ErrChecksumMismatch", err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("calls = %+v, want none: a mismatch must stop the run before applying anything", store.calls)
	}
}

// A failing file stops the run; later files are not attempted and the earlier
// ones stay applied, so the next run resumes from the failure.
func TestRunStopsAtFirstFailure(t *testing.T) {
	store := newFakeStore()
	store.failOn = "002"
	engine := &Engine{Store: store}

	err := engine.Run(context.Background(), Source{
		PluginID: "demo",
		Files:    []File{file("001", "c1"), file("002", "c2"), file("003", "c3")},
	})
	if err == nil {
		t.Fatal("Run: want an error")
	}
	if len(store.calls) != 2 {
		t.Fatalf("calls = %+v, want 001 and 002 only", store.calls)
	}
	if _, ok := store.applied["demo"]["001"]; !ok {
		t.Fatal("001 should remain applied after a later failure")
	}
}

func TestRunRejectsDuplicateVersions(t *testing.T) {
	store := newFakeStore()
	engine := &Engine{Store: store}

	err := engine.Run(context.Background(), Source{
		PluginID: "demo",
		Files:    []File{file("001", "c1"), file("001", "c1b")},
	})
	if err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("err = %v, want a duplicate-version error", err)
	}
}

func TestPendingCountsUnappliedFiles(t *testing.T) {
	store := newFakeStore()
	store.applied["demo"] = map[string]string{"001": "c1"}
	engine := &Engine{Store: store}

	pending, err := engine.Pending(context.Background(), Source{
		PluginID: "demo",
		Files:    []File{file("001", "c1"), file("002", "c2"), file("003", "c3")},
	})
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if pending != 2 {
		t.Fatalf("pending = %d, want 2", pending)
	}
}
