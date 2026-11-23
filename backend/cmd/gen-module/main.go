// Command gen-module renders a tenant-aware CRUD module from a YAML spec.
//
//	gen-module -spec article.yaml            # write files under the current dir
//	gen-module -spec article.yaml -dry-run   # print what would be written
//
// The spec is validated before anything is written; a module that would produce
// invalid SQL or Go is refused rather than half-generated.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"dragz-admin/backend/codegen"
)

func main() {
	specPath := flag.String("spec", "", "path to the module spec (.yaml or .json)")
	root := flag.String("root", ".", "backend directory the files are written under")
	dryRun := flag.Bool("dry-run", false, "print the generated files instead of writing them")
	flag.Parse()
	if *specPath == "" {
		fmt.Fprintln(os.Stderr, "gen-module: -spec is required")
		os.Exit(2)
	}
	spec, err := codegen.Load(*specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-module: %v\n", err)
		os.Exit(1)
	}
	if *dryRun {
		version, err := codegen.NextMigrationVersion(filepath.Join(*root, "migrations"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "gen-module: %v\n", err)
			os.Exit(1)
		}
		files, err := codegen.Generate(spec, version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gen-module: %v\n", err)
			os.Exit(1)
		}
		for _, file := range files {
			fmt.Printf("==> %s\n%s\n", file.Path, file.Content)
		}
		return
	}
	files, err := codegen.Write(*root, spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-module: %v\n", err)
		os.Exit(1)
	}
	for _, file := range files {
		fmt.Printf("wrote %s\n", filepath.Join(*root, file.Path))
	}
	fmt.Println("next: run sqlc generate, then append <Type>Routes() to internal/httpapi/routes.go")
}
