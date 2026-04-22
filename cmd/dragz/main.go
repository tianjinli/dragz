package main

import (
	"context"
	"log"

	"github.com/tianjinli/dragz/cmd"
	"github.com/tianjinli/dragz/pkg/appkit"
)

func main() {
	log.Println("****** Dragz version", appkit.Version, "******")

	app, cleanup, err := cmd.InitContainer()
	if err != nil {
		log.Panicf("%+v\n", err)
	}

	defer cleanup()
	err = app.Run(context.Background())
	if err != nil {
		log.Panicf("%+v\n", err)
	}
}
