package app

import (
	//	"fmt"

	//	"fyne.io/fyne/v2/app"
	//"github.com/gngram/spire_admin/config"
	"github.com/gngram/spire_admin/ui"
)

func Run(parentSocket string) {
	/*
		cfg := &config.AppConfig{
			ParentSocket: parentSocket,
		}

		myApp := app.NewWithID("com.github.gngram.spire_admin")
		myApp.Settings().SetTheme(ui.NewPurpleTheme())
		fmt.Println("RRRRRRRRRRRRRRRRRRRRRRRRRRRRRRRR")
		fmt.Println(cfg.ParentSocket)
		ui.ShowMain(myApp, cfg)
		myApp.Run()
	*/
	ui.OpenDashboard(parentSocket)
}
