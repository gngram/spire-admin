package main

import (
	"flag"

	"github.com/gngram/spire_admin/ui"
)

func main() {
	parentSocket := flag.String("socket", "", "Path to the parent agent socket")
	width := flag.Int("width", 1280, "Initial window width")
	height := flag.Int("height", 720, "Initial window height")
	flag.Parse()
	if *parentSocket == "" {
		flag.Usage()
		return
	}

	ui.OpenDashboard(*parentSocket, uint16(*width), uint16(*height))
}
