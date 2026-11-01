// Command gen-api regenerates api/openapi/openapi.json from the compiled route
// table, which is the only source of truth for the documented API. Run
// `go run ./cmd/gen-api` after any route change and commit the result.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"dragz-admin/backend/internal/kernel"
	"dragz-admin/backend/plugins/all"
)

func main() {
	out := flag.String("o", "api/openapi/openapi.json", "output file path")
	flag.Parse()

	registry, err := kernel.Build(all.All())
	if err != nil {
		log.Fatalf("build registry: %v", err)
	}

	doc := map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "dragz admin api",
			"version":     "generated",
			"description": "GENERATED FILE — do not edit. Source of truth: backend plugin registry (go run ./cmd/gen-api).",
		},
		"paths": buildPaths(registry),
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	fmt.Printf("gen-api: wrote %s (%d paths)\n", *out, len(doc["paths"].(map[string]any)))
}

func buildPaths(registry *kernel.Registry) map[string]any {
	paths := map[string]any{}
	for _, bound := range registry.Routes() {
		route := bound.Route
		item, _ := paths[route.Path].(map[string]any)
		if item == nil {
			item = map[string]any{}
			paths[route.Path] = item
		}
		method := strings.ToLower(route.Method)
		operation := map[string]any{
			"operationId": operationID(route.Method, route.Path),
			"tags":        []string{bound.PluginID},
			"responses": map[string]any{
				"200": map[string]any{"description": "OK"},
			},
		}
		if !route.Public {
			operation["security"] = []map[string]any{{"bearerAuth": []string{}}}
		}
		item[method] = operation
	}
	return paths
}

func operationID(method, path string) string {
	id := strings.ToLower(method)
	for _, segment := range strings.Split(strings.Trim(path, "/"), "/") {
		segment = strings.Trim(segment, "{}")
		segment = strings.ReplaceAll(segment, ":", "_")
		if segment == "" {
			continue
		}
		id += "_" + segment
	}
	return id
}
