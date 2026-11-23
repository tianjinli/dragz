// Package codegen renders a tenant-aware CRUD module from a small spec.
//
// It exists because the tenant boundary is the part every hand-written module
// gets wrong: a new table without tenant_id, a query without a member check, a
// migration without FORCE ROW LEVEL SECURITY. The generator makes the safe
// shape the easy one and the omissions impossible to forget.
//
// The output is ordinary code, not a framework: a migration, sqlc queries, a
// repository and HTTP handlers in the same style as the rest of the repository.
package codegen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Field is one column of the generated table.
type Field struct {
	// Name is the snake_case column name.
	Name string `yaml:"name" json:"name"`
	// Type is one of string, text, int, bool, time, decimal, json.
	Type string `yaml:"type" json:"type"`
	// Label is the human name used in the generated comments and the menu.
	Label string `yaml:"label" json:"label"`
	// Required rejects an empty value on create.
	Required bool `yaml:"required" json:"required"`
	// Nullable maps to a SQL NULL and a Go pointer.
	Nullable bool `yaml:"nullable" json:"nullable"`
	// Searchable adds the field to the keyword filter of the list query.
	Searchable bool `yaml:"searchable" json:"searchable"`
	// MaxLength is the VARCHAR length for string fields. 0 means 255.
	MaxLength int `yaml:"maxLength" json:"maxLength"`
}

// Spec is one module to generate.
type Spec struct {
	// Module is the lower_snake base for file names and the Go type prefix.
	Module string `yaml:"module" json:"module"`
	// Table is the SQL table name. Defaults to Module.
	Table string `yaml:"table" json:"table"`
	// Title is the Chinese display name.
	Title string `yaml:"title" json:"title"`
	// Route is the API prefix, for example /api/cms/articles.
	Route string `yaml:"route" json:"route"`
	// Permission is the permission prefix, for example cms:article.
	Permission string `yaml:"permission" json:"permission"`
	// MenuCode is the page the permission buttons hang under.
	MenuCode string `yaml:"menuCode" json:"menuCode"`
	// Fields are the business columns. id/tenant_id/timestamps are added.
	Fields []Field `yaml:"fields" json:"fields"`
}

var (
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	routePattern      = regexp.MustCompile(`^/[a-z0-9/_-]+$`)
	permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*:[a-z][a-z0-9_]*$`)
)

// Load reads a YAML or JSON spec and validates it.
func Load(path string) (Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	var spec Spec
	if strings.HasSuffix(strings.ToLower(path), ".json") {
		if err := yaml.Unmarshal(raw, &spec); err != nil {
			return Spec{}, fmt.Errorf("parse spec: %w", err)
		}
	} else if err := yaml.Unmarshal(raw, &spec); err != nil {
		return Spec{}, fmt.Errorf("parse spec: %w", err)
	}
	if err := spec.Normalize(); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

// Normalize fills defaults and rejects anything that would generate broken
// code. Every error names the field, because a generator that silently emits
// invalid SQL is worse than one that refuses.
func (s *Spec) Normalize() error {
	s.Module = strings.ToLower(strings.TrimSpace(s.Module))
	s.Table = strings.ToLower(strings.TrimSpace(s.Table))
	if s.Table == "" {
		s.Table = s.Module
	}
	s.Title = strings.TrimSpace(s.Title)
	s.Route = strings.TrimRight(strings.TrimSpace(s.Route), "/")
	s.Permission = strings.ToLower(strings.TrimSpace(s.Permission))
	s.MenuCode = strings.TrimSpace(s.MenuCode)
	if !identifierPattern.MatchString(s.Module) {
		return fmt.Errorf("module %q must be lower_snake_case", s.Module)
	}
	if !identifierPattern.MatchString(s.Table) {
		return fmt.Errorf("table %q must be lower_snake_case", s.Table)
	}
	if s.Title == "" {
		return errors.New("title is required")
	}
	if !routePattern.MatchString(s.Route) {
		return fmt.Errorf("route %q must look like /api/cms/articles", s.Route)
	}
	if !permissionPattern.MatchString(s.Permission) {
		return fmt.Errorf("permission %q must look like cms:article", s.Permission)
	}
	if s.MenuCode == "" {
		s.MenuCode = s.Module
	}
	if len(s.Fields) == 0 {
		return errors.New("at least one field is required")
	}
	seen := map[string]bool{}
	for i := range s.Fields {
		field := &s.Fields[i]
		field.Name = strings.ToLower(strings.TrimSpace(field.Name))
		field.Type = strings.ToLower(strings.TrimSpace(field.Type))
		field.Label = strings.TrimSpace(field.Label)
		if !identifierPattern.MatchString(field.Name) {
			return fmt.Errorf("field %q must be lower_snake_case", field.Name)
		}
		if seen[field.Name] {
			return fmt.Errorf("field %q is declared twice", field.Name)
		}
		seen[field.Name] = true
		if reservedColumns[field.Name] {
			return fmt.Errorf("field %q is managed by the generator", field.Name)
		}
		if _, ok := fieldTypes[field.Type]; !ok {
			return fmt.Errorf("field %q has unknown type %q", field.Name, field.Type)
		}
		if field.Label == "" {
			field.Label = field.Name
		}
		if field.Type == "string" && field.MaxLength <= 0 {
			field.MaxLength = 255
		}
		if field.Type == "string" && field.MaxLength > 4000 {
			return fmt.Errorf("field %q maxLength is too large for VARCHAR", field.Name)
		}
		if field.Nullable && field.Required {
			return fmt.Errorf("field %q cannot be required and nullable", field.Name)
		}
	}
	return nil
}

// reservedColumns are added by the generator and may not be declared.
var reservedColumns = map[string]bool{
	"id": true, "tenant_id": true, "created_at": true, "updated_at": true, "deleted_at": true,
}

// NextMigrationVersion returns the next free NNN prefix in dir.
func NextMigrationVersion(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	max := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			continue
		}
		value, err := strconv.Atoi(prefix)
		if err != nil {
			continue
		}
		if value > max {
			max = value
		}
	}
	if max == 0 {
		return 1, nil
	}
	return max + 1, nil
}

// Output is one generated file. Path is relative to the repository backend
// directory.
type Output struct {
	Path    string
	Content []byte
}

// Write renders the spec and writes every file under root. Existing files are
// never overwritten: regenerating a module is a manual decision.
func Write(root string, spec Spec) ([]Output, error) {
	version, err := NextMigrationVersion(filepath.Join(root, "migrations"))
	if err != nil {
		return nil, err
	}
	files, err := Generate(spec, version)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		target := filepath.Join(root, file.Path)
		if _, statErr := os.Stat(target); statErr == nil {
			return nil, fmt.Errorf("%s already exists; remove it first", file.Path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, file.Content, 0o644); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// GoType is the field's type in a read struct: nullable columns are pointers.
func (f Field) GoType() string {
	base := fieldTypes[f.Type].Go
	if f.Nullable {
		return "*" + base
	}
	return base
}

// CreateType is the field's type in a write struct: a nullable column stays a
// pointer so "absent" and "zero" remain distinguishable.
func (f Field) CreateType() string {
	base := fieldTypes[f.Type].Go
	if f.Nullable {
		return "*" + base
	}
	return base
}

// PointerType is the request-decoding type. Every incoming field is optional at
// the wire level; validation decides which ones are required for a create.
func (f Field) PointerType() string {
	return "*" + fieldTypes[f.Type].Go
}

// BaseType is the non-pointer Go type.
func (f Field) BaseType() string {
	return fieldTypes[f.Type].Go
}

// sqlType returns the column type of a field.
func (f Field) sqlType() string {
	switch f.Type {
	case "string":
		return fmt.Sprintf("VARCHAR(%d)", f.MaxLength)
	case "text":
		return "TEXT"
	case "int":
		return "BIGINT"
	case "bool":
		return "BOOLEAN"
	case "time":
		return "TIMESTAMPTZ"
	case "decimal":
		return "NUMERIC(18, 2)"
	case "json":
		return "JSONB"
	default:
		return "TEXT"
	}
}

// zeroValue is the Go literal used when a non-nullable field is absent.
func (f Field) zeroValue() string {
	switch f.Type {
	case "int", "decimal":
		return "0"
	case "bool":
		return "false"
	case "time":
		return "time.Time{}"
	case "json":
		return "nil"
	default:
		return `""`
	}
}

type fieldType struct {
	Go string
}

var fieldTypes = map[string]fieldType{
	"string":  {Go: "string"},
	"text":    {Go: "string"},
	"int":     {Go: "int64"},
	"bool":    {Go: "bool"},
	"time":    {Go: "time.Time"},
	"decimal": {Go: "float64"},
	"json":    {Go: "json.RawMessage"},
}

// Camel converts snake_case to UpperCamelCase.
func Camel(value string) string {
	parts := strings.Split(value, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

// LowerCamel converts snake_case to lowerCamelCase.
func LowerCamel(value string) string {
	camel := Camel(value)
	if camel == "" {
		return camel
	}
	return strings.ToLower(camel[:1]) + camel[1:]
}
