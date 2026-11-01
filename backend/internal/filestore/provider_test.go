package filestore

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLocalProviderLifecycle(t *testing.T) {
	provider, err := New(Config{Type: "local", BasePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Test(context.Background()); err != nil {
		t.Fatal(err)
	}
	content := "dragz admin attachment"
	if err := provider.Put(context.Background(), "2026/08/test.txt", strings.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	reader, err := provider.Open(context.Background(), "2026/08/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	value, err := io.ReadAll(reader)
	if err != nil || string(value) != content {
		t.Fatalf("unexpected content %q: %v", value, err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := provider.Delete(context.Background(), "2026/08/test.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestLocalProviderRejectsTraversal(t *testing.T) {
	provider, err := New(Config{Type: "local", BasePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Put(context.Background(), "../../escape.txt", strings.NewReader("x"), 1, "text/plain"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestLocalProviderDeleteExpiredUnderPrefix(t *testing.T) {
	root := t.TempDir()
	provider, err := New(Config{Type: "local", BasePath: root})
	if err != nil {
		t.Fatal(err)
	}
	key := StorageTestPrefix + "favicon.svg"
	if err := provider.Put(context.Background(), key, bytes.NewReader([]byte("svg")), 3, "image/svg+xml"); err != nil {
		t.Fatal(err)
	}
	path, err := provider.(*localProvider).resolve(key)
	if err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	deleted, err := provider.DeleteExpiredUnderPrefix(context.Background(), StorageTestPrefix, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted file, got %d", deleted)
	}
	_, err = provider.Open(context.Background(), key)
	if err == nil {
		t.Fatal("expected expired test file to be removed")
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	if _, err := New(Config{Type: "local"}); err == nil {
		t.Fatal("expected local storage without base path to be rejected")
	}
	if _, err := New(Config{Type: "ftp", BasePath: t.TempDir()}); err == nil {
		t.Fatal("expected unsupported storage type to be rejected")
	}
}

func TestLocalProviderMissingObjectsAreSafe(t *testing.T) {
	provider, err := New(Config{Type: "local", BasePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := provider.Open(ctx, "2026/08/missing.txt"); err == nil {
		t.Fatal("expected opening a missing object to fail")
	}
	if err := provider.Delete(ctx, "2026/08/missing.txt"); err != nil {
		t.Fatalf("deleting a missing object should be a no-op, got %v", err)
	}
	deleted, err := provider.DeleteExpiredUnderPrefix(ctx, StorageTestPrefix, time.Now())
	if err != nil {
		t.Fatalf("cleanup of a missing prefix should succeed, got %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted for missing prefix, got %d", deleted)
	}
}

func TestLocalProviderResolveRejectsEscapes(t *testing.T) {
	provider := &localProvider{root: t.TempDir()}
	for _, key := range []string{"../escape.txt", "a/../../escape.txt"} {
		if _, err := provider.resolve(key); err == nil {
			t.Fatalf("expected key %q to be rejected", key)
		}
	}
}
