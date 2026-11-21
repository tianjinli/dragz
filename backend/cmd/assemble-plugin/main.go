// Command assemble-plugin installs an independently delivered plugin zip into
// the host monorepo at build time (plugins/<id>, plugins/all, page-map, locales).
//
//	go run ./cmd/assemble-plugin -zip plugin-demo-v0.1.0.zip [-force]
package main

import (
	"flag"
	"fmt"
	"os"

	"dragz-admin/backend/pluginapi"
)

func main() {
	zipPath := flag.String("zip", "", "plugin delivery zip (required)")
	force := flag.Bool("force", false, "overwrite existing plugin sources and frontend pages")
	flag.Parse()

	if err := run(*zipPath, *force); err != nil {
		fmt.Fprintf(os.Stderr, "assemble-plugin: %v\n", err)
		os.Exit(1)
	}
}

func run(zipPath string, force bool) error {
	host, err := pluginapi.ResolveHostPaths("")
	if err != nil {
		return err
	}
	res, err := pluginapi.Assemble(pluginapi.AssembleOptions{
		Host:    host,
		ZipPath: zipPath,
		Force:   force,
	})
	if err != nil {
		return err
	}
	fmt.Printf("assemble-plugin: installed %s@%s into %s\n", res.Manifest.ID, res.Manifest.Version, res.PluginDir)
	fmt.Println("assemble-plugin: next — bash scripts/check-imports.sh && go build ./... && go run ./cmd/gen-api")
	fmt.Println("assemble-plugin: then enable the plugin in 插件中心 (install/upgrade requires a rebuild)")
	return nil
}
