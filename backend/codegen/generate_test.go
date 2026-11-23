package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleSpec() Spec {
	return Spec{
		Module:     "article",
		Title:      "文章",
		Route:      "/api/cms/articles",
		Permission: "cms:article",
		MenuCode:   "cms_article",
		Fields: []Field{
			{Name: "title", Type: "string", Label: "标题", Required: true, Searchable: true, MaxLength: 200},
			{Name: "summary", Type: "text", Label: "摘要", Nullable: true},
			{Name: "views", Type: "int", Label: "浏览量"},
			{Name: "published", Type: "bool", Label: "已发布"},
			{Name: "published_at", Type: "time", Label: "发布时间", Nullable: true},
		},
	}
}

func TestGenerateKeepsTenantBoundary(t *testing.T) {
	spec := sampleSpec()
	if err := spec.Normalize(); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	files, err := Generate(spec, 20)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	byPath := map[string]string{}
	for _, file := range files {
		byPath[file.Path] = string(file.Content)
	}
	migration, ok := byPath["migrations/020_article.sql"]
	if !ok {
		t.Fatalf("migration not generated: %v", keys(byPath))
	}
	for _, marker := range []string{
		"tenant_id BIGINT NOT NULL",
		"ENABLE ROW LEVEL SECURITY",
		"FORCE ROW LEVEL SECURITY",
		"app_fill_tenant_id",
		"CREATE POLICY tenant_isolation",
	} {
		if !strings.Contains(migration, marker) {
			t.Errorf("migration is missing %q", marker)
		}
	}
	store := byPath["internal/store/article.go"]
	for _, marker := range []string{
		"tenant_id = app_current_tenant_id()",
		"func NewArticleRepository",
		"func scanArticle",
	} {
		if !strings.Contains(store, marker) {
			t.Errorf("store is missing %q", marker)
		}
	}
	handlers := byPath["internal/httpapi/article.go"]
	for _, marker := range []string{
		`permission + ":query"`,
		"func ArticleRoutes()",
		"func createArticleHandler",
	} {
		if !strings.Contains(handlers, marker) {
			t.Errorf("handlers is missing %q", marker)
		}
	}
}

func TestNormalizeRejectsManagedColumns(t *testing.T) {
	spec := sampleSpec()
	spec.Fields = append(spec.Fields, Field{Name: "tenant_id", Type: "int"})
	if err := spec.Normalize(); err == nil {
		t.Fatal("tenant_id should be rejected as a declared field")
	}
}

func TestNormalizeRejectsUnknownType(t *testing.T) {
	spec := sampleSpec()
	spec.Fields[0].Type = "money"
	if err := spec.Normalize(); err == nil {
		t.Fatal("unknown type should be rejected")
	}
}

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "article.yaml")
	body := `module: article
title: 文章
route: /api/cms/articles
permission: cms:article
fields:
  - name: title
    type: string
    required: true
    searchable: true
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if spec.Table != "article" || spec.Fields[0].MaxLength != 255 {
		t.Fatalf("defaults not applied: %+v", spec)
	}
}

func TestNextMigrationVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001_init.sql", "019_tenant.sql", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	version, err := NextMigrationVersion(dir)
	if err != nil {
		t.Fatal(err)
	}
	if version != 20 {
		t.Fatalf("version = %d, want 20", version)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}
