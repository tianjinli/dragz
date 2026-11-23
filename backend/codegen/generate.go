package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"
)

// columnList is "id, title, ..., created_at, updated_at".
func columnList(spec Spec) string {
	names := []string{"id"}
	for _, field := range spec.Fields {
		names = append(names, field.Name)
	}
	names = append(names, "created_at", "updated_at")
	return strings.Join(names, ", ")
}

func insertList(spec Spec) string {
	names := make([]string, 0, len(spec.Fields))
	for _, field := range spec.Fields {
		names = append(names, field.Name)
	}
	return strings.Join(names, ", ")
}

func placeholders(count int) string {
	parts := make([]string, 0, count)
	for i := 1; i <= count; i++ {
		parts = append(parts, fmt.Sprintf("$%d", i))
	}
	return strings.Join(parts, ", ")
}

// keywordClause builds the searchable predicate. A module with no searchable
// text field gets TRUE so the query keeps the same argument count.
func keywordClause(spec Spec) string {
	var parts []string
	for _, field := range spec.Fields {
		if field.Searchable && (field.Type == "string" || field.Type == "text") {
			parts = append(parts, fmt.Sprintf("%s ILIKE '%%' || $1 || '%%'", field.Name))
		}
	}
	if len(parts) == 0 {
		return "TRUE"
	}
	return "($1 = '' OR " + strings.Join(parts, " OR ") + ")"
}

func setClause(spec Spec) string {
	parts := make([]string, 0, len(spec.Fields)+1)
	for i, field := range spec.Fields {
		parts = append(parts, fmt.Sprintf("%s = $%d", field.Name, i+2))
	}
	parts = append(parts, "updated_at = NOW()")
	return strings.Join(parts, ",\n  ")
}

// statements is one SQL statement per store method, shared by the queries file
// and the generated repository so the two can never drift.
type statements struct {
	List   string
	Count  string
	Get    string
	Create string
	Update string
	Delete string
}

func buildStatements(spec Spec) statements {
	table := spec.Table
	where := "deleted_at IS NULL\n  AND tenant_id = app_current_tenant_id()\n  AND " + keywordClause(spec)
	return statements{
		List: fmt.Sprintf(`SELECT %s
FROM %s
WHERE %s
ORDER BY id DESC
LIMIT $2 OFFSET $3`, columnList(spec), table, where),
		Count: fmt.Sprintf(`SELECT COUNT(*)
FROM %s
WHERE %s`, table, where),
		Get: fmt.Sprintf(`SELECT %s
FROM %s
WHERE id = $1 AND tenant_id = app_current_tenant_id() AND deleted_at IS NULL`, columnList(spec), table),
		Create: fmt.Sprintf(`INSERT INTO %s (%s)
VALUES (%s)
RETURNING id`, table, insertList(spec), placeholders(len(spec.Fields))),
		Update: fmt.Sprintf(`UPDATE %s
SET %s
WHERE id = $1 AND tenant_id = app_current_tenant_id() AND deleted_at IS NULL`, table, setClause(spec)),
		Delete: fmt.Sprintf(`UPDATE %s
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND tenant_id = app_current_tenant_id() AND deleted_at IS NULL`, table),
	}
}

// Generate renders every file for spec. version is the migration prefix.
func Generate(spec Spec, version int) ([]Output, error) {
	stmts := buildStatements(spec)
	files := []struct {
		path string
		body func() ([]byte, error)
	}{
		{fmt.Sprintf("migrations/%03d_%s.sql", version, spec.Table), func() ([]byte, error) {
			return renderMigration(spec)
		}},
		{fmt.Sprintf("internal/store/%s.go", spec.Module), func() ([]byte, error) {
			return renderStore(spec, stmts)
		}},
		{fmt.Sprintf("internal/httpapi/%s.go", spec.Module), func() ([]byte, error) {
			return renderHandlers(spec)
		}},
	}
	out := make([]Output, 0, len(files))
	for _, file := range files {
		body, err := file.body()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.path, err)
		}
		out = append(out, Output{Path: file.path, Content: body})
	}
	return out, nil
}

func renderMigration(spec Spec) ([]byte, error) {
	var columns strings.Builder
	for _, field := range spec.Fields {
		nullability := "NOT NULL"
		if field.Nullable {
			nullability = "NULL"
		}
		fmt.Fprintf(&columns, "  %s %s %s%s,\n", field.Name, field.sqlType(), nullability, field.sqlDefaultSpec())
	}
	return execute("migration", migrationTemplate, map[string]any{
		"Spec":    spec,
		"Columns": columns.String(),
	})
}

func renderStore(spec Spec, stmts statements) ([]byte, error) {
	var (
		structFields strings.Builder
		createFields strings.Builder
		scanArgs     strings.Builder
		createArgs   strings.Builder
		updateArgs   strings.Builder
		applyInput   strings.Builder
	)
	for _, field := range spec.Fields {
		fmt.Fprintf(&structFields, "\t%s %s `json:\"%s\"`\n", Camel(field.Name), field.GoType(), LowerCamel(field.Name))
		fmt.Fprintf(&createFields, "\t%s %s `json:\"%s\"`\n", Camel(field.Name), field.CreateType(), LowerCamel(field.Name))
		fmt.Fprintf(&scanArgs, "\t\t&item.%s,\n", Camel(field.Name))
		fmt.Fprintf(&createArgs, "\t\tinput.%s,\n", Camel(field.Name))
		fmt.Fprintf(&updateArgs, "\t\tcurrent.%s,\n", Camel(field.Name))
		fmt.Fprintf(&applyInput, "\tif input.%s != nil {\n\t\tcurrent.%s = *input.%s\n\t}\n",
			Camel(field.Name), Camel(field.Name), Camel(field.Name))
	}
	source, err := execute("store", storeTemplate, map[string]any{
		"Spec":         spec,
		"Type":         Camel(spec.Module),
		"Repo":         Camel(spec.Module) + "Repository",
		"StructFields": structFields.String(),
		"CreateFields": createFields.String(),
		"ScanArgs":     scanArgs.String(),
		"CreateArgs":   createArgs.String(),
		"UpdateArgs":   updateArgs.String(),
		"ApplyInput":   applyInput.String(),
		"SQL":          stmts,
	})
	if err != nil {
		return nil, err
	}
	return formatGo(source)
}

func renderHandlers(spec Spec) ([]byte, error) {
	var (
		fields   strings.Builder
		validate strings.Builder
		toCreate strings.Builder
		toUpdate strings.Builder
	)
	for _, field := range spec.Fields {
		fmt.Fprintf(&fields, "\t%s %s `json:\"%s\"`\n", Camel(field.Name), field.CreateType(), LowerCamel(field.Name))
		fmt.Fprintf(&toUpdate, "\t\t%s: req.%s,\n", Camel(field.Name), Camel(field.Name))
		if field.Required {
			fmt.Fprintf(&validate, "\tif req.%s == nil {\n\t\tfail(w, r, http.StatusBadRequest, apierrors.CodeValidation, \"errors.validation\")\n\t\treturn store.%sCreate{}, false\n\t}\n", Camel(field.Name), Camel(spec.Module))
			if field.Type == "string" || field.Type == "text" {
				fmt.Fprintf(&validate, "\tif strings.TrimSpace(*req.%s) == \"\" {\n\t\tfail(w, r, http.StatusBadRequest, apierrors.CodeValidation, \"errors.validation\")\n\t\treturn store.%sCreate{}, false\n\t}\n", Camel(field.Name), Camel(spec.Module))
			}
			fmt.Fprintf(&toCreate, "\t\t%s: *req.%s,\n", Camel(field.Name), Camel(field.Name))
			continue
		}
		fmt.Fprintf(&toCreate, "\t\t%s: req.%s,\n", Camel(field.Name), Camel(field.Name))
	}
	source, err := execute("handlers", handlersTemplate, map[string]any{
		"Spec":     spec,
		"Type":     Camel(spec.Module),
		"Fields":   fields.String(),
		"Validate": validate.String(),
		"ToCreate": toCreate.String(),
		"ToUpdate": toUpdate.String(),
	})
	if err != nil {
		return nil, err
	}
	return formatGo(source)
}

func execute(name, body string, data map[string]any) ([]byte, error) {
	tpl, err := template.New(name).Funcs(template.FuncMap{
		"Camel":      Camel,
		"LowerCamel": LowerCamel,
		"Lower":      strings.ToLower,
	}).Parse(body)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// formatGo runs the generated source through gofmt so a template mistake is a
// generator error instead of a compile error in the operator's tree.
func formatGo(source []byte) ([]byte, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return nil, fmt.Errorf("generated Go does not parse: %w", err)
	}
	return formatted, nil
}

// sqlDefaultSpec is the column default for a NOT NULL business column, so a row
// written before the column existed keeps working.
func (f Field) sqlDefaultSpec() string {
	switch f.Type {
	case "int", "decimal":
		return " DEFAULT 0"
	case "bool":
		return " DEFAULT FALSE"
	case "time":
		return " DEFAULT NOW()"
	case "json":
		return " DEFAULT '{}'::jsonb"
	default:
		return " DEFAULT ''"
	}
}
