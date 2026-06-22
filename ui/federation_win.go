package ui

import (
	"context"
	"fmt"
	"image/color"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/gngram/spire_admin/servers"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

type federationRowWidget struct {
	widget.BaseWidget
	trustDomainLbl *widget.Label
	infoBtn        *clickableStack
	updateBtn      *clickableStack
	refreshBtn     *clickableStack
	deleteBtn      *clickableStack
	container      *fyne.Container
	trustDomain    string
}

func newFederationRowWidget() *federationRowWidget {
	r := &federationRowWidget{
		trustDomainLbl: widget.NewLabel(""),
	}
	r.trustDomainLbl.Truncation = fyne.TextTruncateEllipsis

	tooltipTxt := canvas.NewText("", clrMuted)
	tooltipTxt.TextSize = 12
	tooltipTxt.Alignment = fyne.TextAlignTrailing
	tooltipTxt.TextStyle = fyne.TextStyle{Italic: true}
	tooltipWrapper := container.New(layout.NewGridWrapLayout(fyne.NewSize(45, 32)), container.NewCenter(tooltipTxt))

	makeBtn := func(icon fyne.Resource, tooltip string) *clickableStack {
		bg := canvas.NewRectangle(clrBg)
		bg.CornerRadius = 6
		ic := widget.NewIcon(icon)
		btn := newClickableStack(container.NewStack(
			container.New(layout.NewGridWrapLayout(fyne.NewSize(32, 32)), bg),
			container.NewCenter(ic),
		), nil)
		btn.onHoverIn = func() {
			bg.FillColor = clrBorder
			bg.Refresh()
			tooltipTxt.Text = tooltip
			tooltipTxt.Refresh()
		}
		btn.onHoverOut = func() {
			bg.FillColor = clrBg
			bg.Refresh()
			tooltipTxt.Text = ""
			tooltipTxt.Refresh()
		}
		return btn
	}

	r.infoBtn = makeBtn(theme.InfoIcon(), "Info")
	r.updateBtn = makeBtn(theme.DocumentCreateIcon(), "Update")
	r.refreshBtn = makeBtn(theme.ViewRefreshIcon(), "Refresh")
	r.deleteBtn = makeBtn(theme.DeleteIcon(), "Delete")

	actionGroup := container.NewHBox(tooltipWrapper, r.infoBtn, r.updateBtn, r.refreshBtn, r.deleteBtn)
	r.container = container.NewBorder(nil, nil, nil, actionGroup, r.trustDomainLbl)
	r.ExtendBaseWidget(r)
	return r
}

func (r *federationRowWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.container)
}

func buildFederationContent(spireServer *servers.SpireServer, window fyne.Window, allServers func() []*servers.SpireServer) fyne.CanvasObject {
	title := canvas.NewText("Federation Relationships", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Manage dynamic federation with foreign trust domains.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	var list *widget.List

	refreshData := func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := spireServer.ListFederatedServers(ctx, true)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
				return
			}
			if list != nil {
				fyne.Do(func() { list.Refresh() })
			}
		}()
	}

	list = widget.NewList(
		func() int { return len(spireServer.FederatedServers) },
		func() fyne.CanvasObject { return newFederationRowWidget() },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*federationRowWidget)
			f := spireServer.FederatedServers[id]
			row.trustDomain = f.TrustDomain
			row.trustDomainLbl.SetText(fmt.Sprintf("%s (%s)", f.TrustDomain, f.Address))

			row.infoBtn.onTap = func() {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					rel, err := spireServer.GetFederationRelationship(ctx, row.trustDomain)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(err, window) })
						return
					}
					
					profileType := "Unknown"
					spiffeID := "N/A"
					if rel.GetHttpsSpiffe() != nil {
						profileType = "HTTPS SPIFFE"
						spiffeID = rel.GetHttpsSpiffe().EndpointSpiffeId
					} else if rel.GetHttpsWeb() != nil {
						profileType = "HTTPS Web"
					}

					details := fmt.Sprintf("Trust Domain: %s\nBundle Endpoint: %s\nProfile Type: %s\nSPIFFE ID: %s",
						rel.TrustDomain, rel.BundleEndpointUrl, profileType, spiffeID)
					fyne.Do(func() { dialog.ShowInformation("Federation Info", details, window) })
				}()
			}

			row.updateBtn.onTap = func() {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					rel, err := spireServer.GetFederationRelationship(ctx, row.trustDomain)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(err, window) })
						return
					}
					fyne.Do(func() {
						showFederationDialog(spireServer, window, refreshData, rel)
					})
				}()
			}

			row.refreshBtn.onTap = func() {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := spireServer.RefreshFederationBundle(ctx, row.trustDomain); err != nil {
						fyne.Do(func() { dialog.ShowError(err, window) })
					} else {
						fyne.Do(func() { dialog.ShowInformation("Success", "Bundle refreshed", window) })
					}
				}()
			}

			row.deleteBtn.onTap = func() {
				dialog.ShowConfirm("Delete Federation", fmt.Sprintf("Delete relationship with %s?", row.trustDomain), func(ok bool) {
					if ok {
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer cancel()
							if err := spireServer.DeleteFederationRelationship(ctx, row.trustDomain); err != nil {
								fyne.Do(func() { dialog.ShowError(err, window) })
							}
							refreshData()
						}()
					}
				}, window)
			}
		},
	)

	newBtn := widget.NewButtonWithIcon("New", theme.ContentAddIcon(), func() {
		showNewFederationDialog(spireServer, window, refreshData, allServers)
	})

	topBar := container.NewBorder(nil, nil, titleBlock, newBtn)

	bg := canvas.NewRectangle(clrCard)
	bg.CornerRadius = 8
	bg.StrokeColor = clrBorder
	bg.StrokeWidth = 1

	card := container.NewStack(bg, container.NewPadded(list))
	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(0, 16))

	return container.NewPadded(
		container.NewBorder(
			container.NewVBox(topBar, widget.NewSeparator(), gap),
			nil, nil, nil,
			container.NewPadded(card),
		),
	)
}

func showNewFederationDialog(spireServer *servers.SpireServer, window fyne.Window, refreshData func(), allServers func() []*servers.SpireServer) {
	// Create the Internal Tab Content
	eligibleServers := []*servers.SpireServer{}
	eligibleNames := []string{}
	currentDomain := spireServer.Domain

	alreadyFederated := make(map[string]bool)
	for _, f := range spireServer.FederatedServers {
		alreadyFederated[f.TrustDomain] = true
	}

	if allServers != nil {
		for _, srv := range allServers() {
			if srv.Domain == "" || srv.Domain == "Unknown" || srv.Domain == currentDomain {
				continue
			}
			if alreadyFederated[srv.Domain] {
				continue
			}
			eligibleServers = append(eligibleServers, srv)
			eligibleNames = append(eligibleNames, fmt.Sprintf("%s (%s)", srv.Nickname, srv.Domain))
		}
	}

	var internalContent fyne.CanvasObject
	var srvSelect *widget.Select
	if len(eligibleServers) == 0 {
		internalContent = container.NewPadded(widget.NewLabel("No other eligible internal servers found."))
	} else {
		srvSelect = widget.NewSelect(eligibleNames, nil)
		srvSelect.SetSelected(eligibleNames[0])
		internalForm := widget.NewForm(
			widget.NewFormItem("Select Server", srvSelect),
		)
		internalContent = container.NewPadded(internalForm)
	}

	// Create the External Tab Content
	domainEntry := widget.NewEntry()
	domainEntry.SetPlaceHolder("example.org")
	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://endpoint-url:port")
	profileSelect := widget.NewSelect([]string{"HTTPS SPIFFE", "HTTPS Web"}, nil)
	spiffeIDEntry := widget.NewEntry()
	spiffeIDEntry.SetPlaceHolder("spiffe://example.org/spire/server")

	profileSelect.OnChanged = func(val string) {
		if val == "HTTPS SPIFFE" {
			spiffeIDEntry.Enable()
		} else {
			spiffeIDEntry.Disable()
		}
	}
	profileSelect.SetSelected("HTTPS SPIFFE")

	externalForm := widget.NewForm(
		widget.NewFormItem("Trust Domain", domainEntry),
		widget.NewFormItem("Endpoint URL", urlEntry),
		widget.NewFormItem("Profile Type", profileSelect),
		widget.NewFormItem("Endpoint SPIFFE ID", spiffeIDEntry),
	)
	externalContent := container.NewPadded(externalForm)

	// Create AppTabs
	tabs := container.NewAppTabs(
		container.NewTabItem("Internal Server", internalContent),
		container.NewTabItem("External Domain", externalContent),
	)

	// Wrap in a padded container to ensure gap from the border
	dialogContent := container.NewPadded(tabs)

	d := dialog.NewCustomConfirm("New Federation", "Save", "Cancel", dialogContent, func(ok bool) {
		if !ok {
			return
		}
		// Based on selected tab, execute internal or external federation
		if tabs.SelectedIndex() == 0 {
			// Internal
			if len(eligibleServers) == 0 {
				dialog.ShowError(fmt.Errorf("No internal server selected"), window)
				return
			}
			selectedIdx := -1
			for idx, name := range eligibleNames {
				if name == srvSelect.Selected {
					selectedIdx = idx
					break
				}
			}
			if selectedIdx == -1 {
				return
			}
			targetSrv := eligibleServers[selectedIdx]

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()

				// 1. Pull bundle from target server
				bundle, err := targetSrv.GetBundle(ctx, "")
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("failed to pull bundle: %v", err), window) })
					return
				}

				// 2. Push bundle to current server
				_, err = spireServer.SetFederatedBundle(ctx, bundle)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("failed to push bundle: %v", err), window) })
					return
				}

				// 3. Create federation relationship
				portInt, _ := strconv.Atoi(targetSrv.Port)
				bundlePort := portInt + 364
				endpointURL := fmt.Sprintf("https://%s:%d", targetSrv.Address, bundlePort)
				spiffeID := fmt.Sprintf("spiffe://%s/spire/server", targetSrv.Domain)

				rel := &types.FederationRelationship{
					TrustDomain:       targetSrv.Domain,
					BundleEndpointUrl: endpointURL,
					BundleEndpointProfile: &types.FederationRelationship_HttpsSpiffe{
						HttpsSpiffe: &types.HTTPSSPIFFEProfile{
							EndpointSpiffeId: spiffeID,
						},
					},
				}

				_, err = spireServer.CreateFederationRelationship(ctx, rel)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("failed to create federation relationship: %v", err), window) })
					return
				}

				fyne.Do(func() {
					dialog.ShowInformation("Success", fmt.Sprintf("Successfully federated with %s!", targetSrv.Nickname), window)
					refreshData()
				})
			}()
		} else {
			// External
			if domainEntry.Text == "" || urlEntry.Text == "" {
				dialog.ShowError(fmt.Errorf("Trust Domain and Endpoint URL are required"), window)
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				rel := &types.FederationRelationship{
					TrustDomain:       domainEntry.Text,
					BundleEndpointUrl: urlEntry.Text,
				}

				if profileSelect.Selected == "HTTPS SPIFFE" {
					rel.BundleEndpointProfile = &types.FederationRelationship_HttpsSpiffe{
						HttpsSpiffe: &types.HTTPSSPIFFEProfile{
							EndpointSpiffeId: spiffeIDEntry.Text,
						},
					}
				} else {
					rel.BundleEndpointProfile = &types.FederationRelationship_HttpsWeb{
						HttpsWeb: &types.HTTPSWebProfile{},
					}
				}

				_, err := spireServer.CreateFederationRelationship(ctx, rel)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, window) })
				}
				refreshData()
			}()
		}
	}, window)

	d.Resize(fyne.NewSize(600, 450))
	d.Show()
}

func showFederationDialog(spireServer *servers.SpireServer, window fyne.Window, refreshData func(), existing *types.FederationRelationship) {
	title := "Update Federation"

	domainEntry := widget.NewEntry()
	domainEntry.SetText(existing.TrustDomain)
	domainEntry.Disable()

	urlEntry := widget.NewEntry()
	urlEntry.SetText(existing.BundleEndpointUrl)

	profileSelect := widget.NewSelect([]string{"HTTPS SPIFFE", "HTTPS Web"}, nil)
	spiffeIDEntry := widget.NewEntry()
	spiffeIDEntry.SetPlaceHolder("spiffe://example.org/spire/server")

	profileSelect.OnChanged = func(val string) {
		if val == "HTTPS SPIFFE" {
			spiffeIDEntry.Enable()
		} else {
			spiffeIDEntry.Disable()
		}
	}

	if existing.GetHttpsSpiffe() != nil {
		profileSelect.SetSelected("HTTPS SPIFFE")
		spiffeIDEntry.SetText(existing.GetHttpsSpiffe().EndpointSpiffeId)
	} else {
		profileSelect.SetSelected("HTTPS Web")
		spiffeIDEntry.Disable()
	}

	items := []*widget.FormItem{
		widget.NewFormItem("Trust Domain", domainEntry),
		widget.NewFormItem("Endpoint URL", urlEntry),
		widget.NewFormItem("Profile Type", profileSelect),
		widget.NewFormItem("Endpoint SPIFFE ID", spiffeIDEntry),
	}

	form := widget.NewForm(items...)
	dialogContent := container.NewPadded(form)

	d := dialog.NewCustomConfirm(title, "Save", "Cancel", dialogContent, func(ok bool) {
		if !ok {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			rel := &types.FederationRelationship{
				TrustDomain:       domainEntry.Text,
				BundleEndpointUrl: urlEntry.Text,
			}

			if profileSelect.Selected == "HTTPS SPIFFE" {
				rel.BundleEndpointProfile = &types.FederationRelationship_HttpsSpiffe{
					HttpsSpiffe: &types.HTTPSSPIFFEProfile{
						EndpointSpiffeId: spiffeIDEntry.Text,
					},
				}
			} else {
				rel.BundleEndpointProfile = &types.FederationRelationship_HttpsWeb{
					HttpsWeb: &types.HTTPSWebProfile{},
				}
			}

			_, err := spireServer.UpdateFederationRelationship(ctx, rel, nil)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
			}
			refreshData()
		}()
	}, window)

	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}
