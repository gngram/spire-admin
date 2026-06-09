package ui

import (
	"context"
	"fmt"
	"image/color"
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

func buildFederationContent(spireServer *servers.SpireServer, window fyne.Window) fyne.CanvasObject {
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
					details := fmt.Sprintf("Trust Domain: %s\nBundle Endpoint: %s\nSPIFFE ID: %s",
						rel.TrustDomain, rel.BundleEndpointUrl, rel.BundleEndpointProfile)
					fyne.Do(func() { dialog.ShowInformation("Federation Info", details, window) })
				}()
			}

			row.updateBtn.onTap = func() {
				showFederationDialog(spireServer, window, refreshData, &types.FederationRelationship{
					TrustDomain:       f.TrustDomain,
					BundleEndpointUrl: f.Address,
				})
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
		showFederationDialog(spireServer, window, refreshData, nil)
	})

	topBar := container.NewBorder(nil, nil, titleBlock, container.NewHBox(newBtn))

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

func showFederationDialog(spireServer *servers.SpireServer, window fyne.Window, refreshData func(), existing *types.FederationRelationship) {
	title := "New Federation"
	if existing != nil {
		title = "Update Federation"
	}

	domainEntry := widget.NewEntry()
	if existing != nil {
		domainEntry.SetText(existing.TrustDomain)
		domainEntry.Disable()
	}
	urlEntry := widget.NewEntry()
	if existing != nil {
		urlEntry.SetText(existing.BundleEndpointUrl)
	}

	items := []*widget.FormItem{
		widget.NewFormItem("Trust Domain", domainEntry),
		widget.NewFormItem("Endpoint URL", urlEntry),
	}

	dialog.ShowForm(title, "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			rel := &types.FederationRelationship{TrustDomain: domainEntry.Text, BundleEndpointUrl: urlEntry.Text}
			var err error
			if existing == nil {
				_, err = spireServer.CreateFederationRelationship(ctx, rel)
			} else {
				_, err = spireServer.UpdateFederationRelationship(ctx, rel, nil)
			}
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
			}
			refreshData()
		}()
	}, window)
}
