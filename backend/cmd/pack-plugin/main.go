// Command pack-plugin builds an independently deliverable plugin zip from a
// plugin already present in the host tree (backend/plugins/<id> + admin pages).
//
//	go run ./cmd/pack-plugin -id example [-o dist/plugin-example-v0.1.0.zip]
package main

import (
	"flag"
	"fmt"
	"os"

	"dragz-admin/backend/pluginapi"
)

func main() {
	id := flag.String("id", "", "plugin id (required)")
	out := flag.String("o", "", "output zip path (default: ./plugin-<id>-v<ver>.zip)")
	flag.Parse()

	if err := run(*id, *out); err != nil {
		fmt.Fprintf(os.Stderr, "pack-plugin: %v\n", err)
		os.Exit(1)
	}
}

func run(id, out string) error {
	host, err := pluginapi.ResolveHostPaths("")
	if err != nil {
		return err
	}
	path, err := pluginapi.Pack(pluginapi.PackOptions{
		Host:      host,
		PluginID:  id,
		OutputZip: out,
	})
	if err != nil {
		return err
	}
	fmt.Printf("pack-plugin: wrote %s\n", path)
	fmt.Println("pack-plugin: install with go run ./cmd/assemble-plugin -zip <path>")
	return nil
}
