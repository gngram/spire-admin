package ui

import (
	"errors"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/gngram/spidar/servers"
)

func ShowServerWindow(spireServer *servers.SpireServer) {
	title := "Server Details"
	if spireServer != nil {
		title = "Server: " + spireServer.Address
	}
	aw := fyne.CurrentApp().NewWindow(title)
	aw.Resize(fyne.NewSize(1020, 700))

	if spireServer == nil {
		dialog.ShowError(errors.New("server is nil"), aw)
		aw.Show()
		return
	}

	agentPane := buildAgentsContent(spireServer, aw)
	entriesPane := buildEntriesContent(spireServer, aw)

	bundleList := widget.NewList(
		func() int { return len(spireServer.Bundles) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(spireServer.Bundles[id].TrustDomain)
		},
	)

	fedList := widget.NewList(
		func() int { return len(spireServer.FederatedServers) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			fed := spireServer.FederatedServers[id]
			o.(*widget.Label).SetText(fed.TrustDomain + " (" + fed.Address + ")")
		},
	)

	contentContainer := container.NewStack(agentPane)

	bg := canvas.NewRectangle(clrSidebar)

	logoTxt := canvas.NewText(spireServer.Address, color.White)
	logoTxt.TextSize = 15
	logoTxt.TextStyle = fyne.TextStyle{Bold: true}
	logoIcon := widget.NewIcon(theme.ComputerIcon())
	logoRow := container.NewPadded(container.NewHBox(logoIcon, logoTxt))

	var navItems []*widget.Button

	makeNavItem := func(icon fyne.Resource, label string, content fyne.CanvasObject, index int) fyne.CanvasObject {
		btn := widget.NewButtonWithIcon(label, icon, func() {
			for i, b := range navItems {
				if i == index {
					b.Importance = widget.HighImportance
				} else {
					b.Importance = widget.LowImportance
				}
				b.Refresh()
			}
			contentContainer.Objects = []fyne.CanvasObject{content}
			contentContainer.Refresh()
		})
		btn.Alignment = widget.ButtonAlignLeading
		if index == 0 {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		navItems = append(navItems, btn)
		return container.NewPadded(btn)
	}

	nav := container.NewVBox(
		logoRow,
		widget.NewSeparator(),
		makeNavItem(theme.AccountIcon(), "Registered Agents", agentPane, 0),
		makeNavItem(theme.FolderIcon(), "Trust Bundles", bundleList, 1),
		makeNavItem(theme.StorageIcon(), "Dynamic Federation", fedList, 2),
		makeNavItem(theme.DocumentIcon(), "Registered Entries", entriesPane, 3),
	)

	sidebar := container.NewStack(bg, container.NewBorder(nav, nil, nil, nil))

	split := container.NewHSplit(sidebar, container.NewPadded(contentContainer))
	split.SetOffset(0.22)

	aw.SetContent(split)
	aw.Show()
}
