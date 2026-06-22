package ui

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/gngram/spire_admin/servers"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

type bundleRowWidget struct {
	widget.BaseWidget
	trustDomainLbl *widget.Label
	infoBtn        *clickableStack
	updateBtn      *clickableStack
	deleteBtn      *clickableStack
	container      *fyne.Container
	trustDomain    string
}

func newBundleRowWidget() *bundleRowWidget {
	r := &bundleRowWidget{
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
	r.deleteBtn = makeBtn(theme.DeleteIcon(), "Delete")

	actionGroup := container.NewHBox(tooltipWrapper, r.infoBtn, r.updateBtn, r.deleteBtn)
	r.container = container.NewBorder(nil, nil, nil, actionGroup, r.trustDomainLbl)
	r.ExtendBaseWidget(r)
	return r
}

func (r *bundleRowWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.container)
}

func buildBundlesContent(spireServer *servers.SpireServer, window fyne.Window) fyne.CanvasObject {
	title := canvas.NewText("Federated Bundles", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Manage federated trust bundles.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	var list *widget.List

	refreshData := func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := spireServer.ListFederatedBundles(ctx, true)
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
		func() int { return len(spireServer.Bundles) },
		func() fyne.CanvasObject { return newBundleRowWidget() },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*bundleRowWidget)
			b := spireServer.Bundles[id]
			row.trustDomain = b.TrustDomain
			row.trustDomainLbl.SetText(b.TrustDomain)

			row.infoBtn.onTap = func() {
				showBundleDetails(spireServer, row.trustDomain, window)
			}
			row.updateBtn.onTap = func() {
				showSetBundleDialog(spireServer, window, refreshData, b)
			}
			row.deleteBtn.onTap = func() {
				dialog.ShowConfirm("Delete Bundle", fmt.Sprintf("Delete federated bundle for %s?", row.trustDomain), func(ok bool) {
					if ok {
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer cancel()
							err := spireServer.DeleteFederatedBundle(ctx, row.trustDomain, bundlev1.BatchDeleteFederatedBundleRequest_RESTRICT)
							if err != nil {
								fyne.Do(func() { dialog.ShowError(err, window) })
							}
							refreshData()
						}()
					}
				}, window)
			}
		},
	)

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), refreshData)
	newBtn := widget.NewButtonWithIcon("New", theme.ContentAddIcon(), func() {
		showSetBundleDialog(spireServer, window, refreshData, nil)
	})

	topBar := container.NewBorder(nil, nil, titleBlock, container.NewHBox(refreshBtn, newBtn))

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

func showBundleDetails(s *servers.SpireServer, td string, w fyne.Window) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		bundle, err := s.GetBundle(ctx, td)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, w) })
			return
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Trust Domain  : %s\n", bundle.TrustDomain)
		fmt.Fprintf(&b, "Sequence      : %d\n", bundle.SequenceNumber)
		fmt.Fprintf(&b, "Refresh Hint  : %d seconds\n", bundle.RefreshHint)

		fmt.Fprintf(&b, "\nX.509 Authorities (%d):\n", len(bundle.X509Authorities))
		for i, auth := range bundle.X509Authorities {
			cert, err := x509.ParseCertificate(auth.Asn1)
			if err != nil {
				fmt.Fprintf(&b, "  [%d] (Error parsing cert: %v)\n", i, err)
				continue
			}
			fmt.Fprintf(&b, "  [%d] Subject: %s\n", i, cert.Subject)
			fmt.Fprintf(&b, "      SKID   : %X\n", cert.SubjectKeyId)
			fmt.Fprintf(&b, "      Expires: %s\n", cert.NotAfter.Format(time.RFC3339))
		}

		fmt.Fprintf(&b, "\nJWT Authorities (%d):\n", len(bundle.JwtAuthorities))
		for i, auth := range bundle.JwtAuthorities {
			fmt.Fprintf(&b, "  [%d] Key ID: %s\n", i, auth.KeyId)
			expires := "N/A"
			if auth.ExpiresAt > 0 {
				expires = time.Unix(auth.ExpiresAt, 0).Format(time.RFC3339)
			}
			fmt.Fprintf(&b, "      Expires: %s\n", expires)
		}

		fyne.Do(func() {
			entry := widget.NewMultiLineEntry()
			entry.SetText(b.String())
			entry.Disable()
			content := container.NewStack(canvas.NewRectangle(clrBg), container.NewPadded(entry))
			d := dialog.NewCustom("Bundle Details: "+td, "Close", content, w)
			d.Resize(fyne.NewSize(750, 480))
			d.Show()
		})
	}()
}

func showSetBundleDialog(spireServer *servers.SpireServer, window fyne.Window, refreshData func(), existing *types.Bundle) {
	title := "Set Federated Bundle"
	if existing != nil {
		title = "Update Federated Bundle"
	}

	var domainWidget fyne.CanvasObject
	domainEntry := widget.NewEntry()

	if existing != nil {
		domainWidget = widget.NewLabel(existing.TrustDomain)
	} else {
		domainEntry.SetPlaceHolder("example.org")
		domainWidget = domainEntry
	}

	pathEntry := widget.NewEntry()
	pathEntry.SetPlaceHolder("/path/to/bundle.pem")

	browseBtn := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if reader == nil {
				return
			}
			pathEntry.SetText(reader.URI().Path())
			reader.Close()
		}, window)
		fd.Show()
	})

	spiffeEntry := widget.NewEntry()
	spiffeEntry.SetPlaceHolder("spiffe://example.org")
	spiffeEntry.Hide()

	fileContainer := container.NewBorder(nil, nil, nil, browseBtn, pathEntry)

	sourceSelect := widget.NewSelect([]string{"From File", "From SPIFFE ID"}, func(s string) {
		if s == "From File" {
			fileContainer.Show()
			spiffeEntry.Hide()
		} else {
			fileContainer.Hide()
			spiffeEntry.Show()
		}
	})
	sourceSelect.SetSelected("From File")

	items := []*widget.FormItem{
		leftAlignedFormItem("Trust Domain:", domainWidget),
		leftAlignedFormItem("Source Type:", sourceSelect),
		leftAlignedFormItem("Bundle Source:", container.NewStack(fileContainer, spiffeEntry)),
	}

	d := dialog.NewForm(title, "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			domain := ""
			if existing != nil {
				domain = existing.TrustDomain
			} else {
				domain = domainEntry.Text
			}

			if domain == "" {
				fyne.Do(func() { dialog.ShowError(fmt.Errorf("trust domain is required"), window) })
				return
			}

			var authorities []*types.X509Certificate

			if sourceSelect.Selected == "From File" {
				filePath := pathEntry.Text
				if filePath == "" {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("bundle file is required"), window) })
					return
				}

				pemBytes, err := os.ReadFile(filePath)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, window) })
					return
				}

				rest := pemBytes
				for {
					block, remainder := pem.Decode(rest)
					if block == nil {
						break
					}
					if block.Type == "CERTIFICATE" {
						authorities = append(authorities, &types.X509Certificate{Asn1: block.Bytes})
					}
					rest = remainder
				}
			} else {
				spiffeID := spiffeEntry.Text
				if spiffeID == "" {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("SPIFFE ID is required"), window) })
					return
				}

				// Extract trust domain if a full SPIFFE ID was provided
				targetTD := spiffeID
				if strings.HasPrefix(spiffeID, "spiffe://") {
					parts := strings.Split(strings.TrimPrefix(spiffeID, "spiffe://"), "/")
					targetTD = parts[0]
				}

				bundle, err := spireServer.GetBundle(ctx, targetTD)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("failed to fetch bundle for %s: %v", targetTD, err), window) })
					return
				}
				if bundle == nil || len(bundle.X509Authorities) == 0 {
					fyne.Do(func() { dialog.ShowError(fmt.Errorf("no authorities found for trust domain %s", targetTD), window) })
					return
				}
				authorities = bundle.X509Authorities
			}

			if len(authorities) == 0 {
				fyne.Do(func() { dialog.ShowError(fmt.Errorf("at least one valid X.509 certificate is required"), window) })
				return
			}

			bundle := &types.Bundle{
				TrustDomain:     domain,
				X509Authorities: authorities,
			}

			_, err := spireServer.SetFederatedBundle(ctx, bundle)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
			} else {
				refreshData()
			}
		}()
	}, window)

	d.Resize(fyne.NewSize(780, 560))
	d.Show()
}
