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
	"github.com/gngram/spire_admin/servers"
)

func ShowServerWindow(spireServer *servers.SpireServer, name string, width uint16, height uint16, allServers func() []*servers.SpireServer) {
	title := "Server Details"
	if spireServer != nil {
		title = "Server: " + name
	}
	aw := fyne.CurrentApp().NewWindow(title)
	aw.Resize(fyne.NewSize(float32(width), float32(height)))
	aw.SetFixedSize(true)
	aw.CenterOnScreen()

	if spireServer == nil {
		dialog.ShowError(errors.New("server is nil"), aw)
		aw.Show()
		return
	}

	agentPane := buildAgentsContent(spireServer, aw)
	entriesPane := buildEntriesContent(spireServer, aw)
	bundlePane := buildBundlesContent(spireServer, aw)
	federationPane := buildFederationContent(spireServer, aw, allServers)
	localAuthorityPane := buildLocalAuthorityContent(spireServer, aw)
	upstreamAuthorityPane := buildUpstreamAuthorityContent(spireServer, aw)

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
		makeNavItem(theme.FolderIcon(), "Trust Bundles", bundlePane, 1),
		makeNavItem(theme.StorageIcon(), "Federations", federationPane, 2),
		makeNavItem(theme.DocumentIcon(), "Registered Entries", entriesPane, 3),
		makeNavItem(theme.DocumentIcon(), "Local Authority: x509", localAuthorityPane, 4),
		makeNavItem(theme.DocumentIcon(), "Upstream Authority", upstreamAuthorityPane, 5),
	)

	sidebar := container.NewStack(bg, container.NewBorder(nav, nil, nil, nil))

	split := container.NewHSplit(sidebar, container.NewPadded(contentContainer))
	split.SetOffset(0.22)

	aw.SetContent(split)
	aw.Show()
}
