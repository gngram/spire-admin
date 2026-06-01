package main

import (
	"flag"

	"github.com/gngram/spire_admin/app"
)

func main() {
	parentSocket := flag.String("socket", "", "Path to the parent agent socket")
	flag.Parse()

	app.Run(*parentSocket)
}
