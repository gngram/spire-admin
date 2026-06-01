package ui

import (
	"context"
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/gngram/spire_admin/servers"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

type fixedWidthLabel struct {
	widget.Label
	width float32
}

func (l *fixedWidthLabel) MinSize() fyne.Size {
	min := l.Label.MinSize()
	return fyne.NewSize(fyne.Max(min.Width, l.width), min.Height)
}

func leftAlignedFormItem(text string, input fyne.CanvasObject) *widget.FormItem {
	lbl := &fixedWidthLabel{width: 220}
	lbl.Text = text
	lbl.Alignment = fyne.TextAlignLeading
	lbl.ExtendBaseWidget(lbl)
	return widget.NewFormItem("", container.NewBorder(nil, nil, lbl, nil, input))
}

type entryRowWidget struct {
	widget.BaseWidget
	spiffeIDLbl *widget.Label
	container   *fyne.Container
	entryID     string
	onTapped    func(pe *fyne.PointEvent, entryID string)
}

func newEntryRowWidget() *entryRowWidget {
	r := &entryRowWidget{
		spiffeIDLbl: widget.NewLabel(""),
	}
	r.spiffeIDLbl.Truncation = fyne.TextTruncateEllipsis

	r.container = container.NewBorder(nil, nil, nil, nil, r.spiffeIDLbl)
	r.ExtendBaseWidget(r)
	return r
}

func (r *entryRowWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.container)
}

func (r *entryRowWidget) Tapped(pe *fyne.PointEvent) {
	if r.onTapped != nil && r.entryID != "" {
		r.onTapped(pe, r.entryID)
	}
}

func buildEntriesContent(spireServer *servers.SpireServer, window fyne.Window) fyne.CanvasObject {
	title := canvas.NewText("Entries", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Manage registered entries.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	var workloads, agents, downstreams []servers.Entry

	filterEntries := func() {
		workloads = nil
		agents = nil
		downstreams = nil
		for _, e := range spireServer.Entries {
			if e.Original != nil && e.Original.Downstream {
				downstreams = append(downstreams, e)
				continue
			}

			isAgent := false
			for _, a := range spireServer.Agents {
				if e.Parent == a.SPIFFEID {
					isAgent = true
					break
				}
			}

			if isAgent || strings.Contains(e.Parent, "/spire/server") || strings.Contains(e.SPIFFEID, "/spire/agent/") {
				agents = append(agents, e)
			} else {
				workloads = append(workloads, e)
			}
		}
	}
	filterEntries()

	var wlList, agList, dsList *widget.List

	refreshData := func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := spireServer.ListEntries(ctx, true)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
				return
			}
			filterEntries()
			if wlList != nil {
				fyne.Do(func() {
					wlList.Refresh()
					agList.Refresh()
					dsList.Refresh()
				})
			}
		}()
	}

	createList := func(entries *[]servers.Entry) *widget.List {
		l := widget.NewList(
			func() int { return len(*entries) },
			func() fyne.CanvasObject {
				row := newEntryRowWidget()
				row.onTapped = func(pe *fyne.PointEvent, entryID string) {
					var p *widget.PopUp

					btnInfo := widget.NewButton("Info", func() {
						p.Hide()
						showEntryInfo(spireServer, entryID, window)
					})
					btnUpdate := widget.NewButton("Update", func() {
						p.Hide()
						showUpdateEntryDialog(spireServer, entryID, window, refreshData)
					})
					btnDelete := widget.NewButton("Delete", func() {
						p.Hide()
						dialog.ShowConfirm("Delete Entry", fmt.Sprintf("Are you sure you want to delete entry %s?", entryID), func(ok bool) {
							if ok {
								go func() {
									ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
									defer cancel()
									if err := spireServer.DeleteEntry(ctx, entryID); err != nil {
										fyne.Do(func() { dialog.ShowError(err, window) })
									}
									refreshData()
								}()
							}
						}, window)
					})

					p = widget.NewPopUp(container.NewVBox(btnInfo, btnUpdate, btnDelete), window.Canvas())
					p.ShowAtPosition(pe.AbsolutePosition)
				}
				return row
			},
			func(id widget.ListItemID, o fyne.CanvasObject) {
				row := o.(*entryRowWidget)
				e := (*entries)[id]
				row.entryID = e.ID
				row.spiffeIDLbl.SetText(e.SPIFFEID)
			},
		)
		return l
	}

	wlList = createList(&workloads)
	agList = createList(&agents)
	dsList = createList(&downstreams)

	tabs := container.NewAppTabs(
		container.NewTabItem("Workloads", wlList),
		container.NewTabItem("Agents", agList),
		container.NewTabItem("Downstream Servers", dsList),
	)

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		refreshData()
	})

	newBtn := widget.NewButtonWithIcon("New", theme.ContentAddIcon(), func() {
		showCreateEntryDialog(spireServer, window, refreshData)
	})

	topBar := container.NewBorder(nil, nil, titleBlock, container.NewHBox(refreshBtn, newBtn))

	bg := canvas.NewRectangle(clrCard)
	bg.CornerRadius = 8
	bg.StrokeColor = clrBorder
	bg.StrokeWidth = 1

	card := container.NewStack(bg, container.NewPadded(tabs))

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

func parseSPIFFEID(id string) (*types.SPIFFEID, error) {
	if !strings.HasPrefix(id, "spiffe://") {
		return nil, fmt.Errorf("invalid SPIFFE ID format")
	}
	trimmed := strings.TrimPrefix(id, "spiffe://")
	parts := strings.SplitN(trimmed, "/", 2)
	td := parts[0]
	path := ""
	if len(parts) > 1 {
		path = "/" + parts[1]
	}
	return &types.SPIFFEID{TrustDomain: td, Path: path}, nil
}

func showEntryInfo(spireServer *servers.SpireServer, entryID string, window fyne.Window) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		entry, err := spireServer.GetEntry(ctx, entryID)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, window) })
			return
		}

		fyne.Do(func() {
			spiffeID := "unknown"
			if entry.SpiffeId != nil {
				spiffeID = fmt.Sprintf("spiffe://%s%s", entry.SpiffeId.TrustDomain, entry.SpiffeId.Path)
			}
			parentID := "unknown"
			if entry.ParentId != nil {
				parentID = fmt.Sprintf("spiffe://%s%s", entry.ParentId.TrustDomain, entry.ParentId.Path)
			}

			var selectors []string
			for _, s := range entry.Selectors {
				selectors = append(selectors, fmt.Sprintf("%s:%s", s.Type, s.Value))
			}

			details := fmt.Sprintf("ID : %s\nSPIFFE ID : %s\nParent ID : %s\nSelectors : %s\nDNS Names : %s\nTTL : %d\nDownstream : %t\nAdmin : %t\nHint : %s",
				entry.Id, spiffeID, parentID, strings.Join(selectors, ", "), strings.Join(entry.DnsNames, ", "), entry.X509SvidTtl, entry.Downstream, entry.Admin, entry.Hint)

			lines := strings.Split(details, "\n")
			type pair struct{ key, val string }
			var pairs []pair
			maxKeyLen := 0

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) < 2 {
					continue
				}
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])

				if len(k) > maxKeyLen {
					maxKeyLen = len(k)
				}
				pairs = append(pairs, pair{k, v})
			}

			var finalLines []string
			finalLines = append(finalLines, "\n")
			for _, p := range pairs {
				paddedKey := fmt.Sprintf("%-*s", maxKeyLen, p.key)
				finalLines = append(finalLines, fmt.Sprintf(" %s : %s", paddedKey, p.val))
			}

			fullText := strings.Join(finalLines, "\n")
			grid := widget.NewTextGrid()
			grid.SetText(fullText)

			darkGray := color.NRGBA{R: 100, G: 100, B: 100, A: 255}
			darkGrayStyle := &widget.CustomTextGridStyle{
				FGColor: darkGray,
			}

			for rowIdx, lineText := range finalLines {
				for colIdx := range lineText {
					grid.SetStyleRange(rowIdx, colIdx, rowIdx, colIdx+1, darkGrayStyle)
				}
			}

			bgRect := canvas.NewRectangle(clrBg)
			backgroundContainer := container.NewStack(bgRect, grid)
			scroller := container.NewVScroll(backgroundContainer)

			d := dialog.NewCustom("Entry Details", "Close", scroller, window)
			d.Resize(fyne.NewSize(600, 300))
			d.Show()
		})
	}()
}

func showCreateEntryDialog(spireServer *servers.SpireServer, window fyne.Window, refreshData func()) {
	domain := spireServer.Domain
	for _, e := range spireServer.Entries {
		if e.Original != nil && e.Original.SpiffeId != nil && e.Original.SpiffeId.TrustDomain != "" {
			domain = e.Original.SpiffeId.TrustDomain
			break
		}
	}

	spiffeIDEntry := widget.NewEntry()
	spiffeIDEntry.SetPlaceHolder("spiffe://" + domain + "/new-workload")

	parentIDEntry := widget.NewEntry()
	parentIDEntry.SetPlaceHolder("spiffe://" + domain + "/spire/agent/xyz")

	selectorsEntry := widget.NewEntry()
	selectorsEntry.SetPlaceHolder("unix:uid:1000")

	items := []*widget.FormItem{
		leftAlignedFormItem("SPIFFE ID:", spiffeIDEntry),
		leftAlignedFormItem("Parent ID:", parentIDEntry),
		leftAlignedFormItem("Selectors:", selectorsEntry),
	}

	d := dialog.NewForm("New", "Create", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}

		spiffeVal := spiffeIDEntry.Text
		if spiffeVal == "" {
			spiffeVal = spiffeIDEntry.PlaceHolder
		}
		spiffeID, err := parseSPIFFEID(spiffeVal)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}

		parentVal := parentIDEntry.Text
		if parentVal == "" {
			parentVal = parentIDEntry.PlaceHolder
		}
		parentID, err := parseSPIFFEID(parentVal)
		if err != nil {
			dialog.ShowError(err, window)
			return
		}

		selectorsVal := selectorsEntry.Text
		if selectorsVal == "" {
			selectorsVal = selectorsEntry.PlaceHolder
		}
		var selectors []*types.Selector
		for _, s := range strings.Split(selectorsVal, ",") {
			parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
			if len(parts) == 2 {
				selectors = append(selectors, &types.Selector{Type: strings.TrimSpace(parts[0]), Value: strings.TrimSpace(parts[1])})
			}
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			entry := &types.Entry{
				SpiffeId:  spiffeID,
				ParentId:  parentID,
				Selectors: selectors,
			}

			_, err := spireServer.CreateEntry(ctx, entry)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
			} else {
				refreshData()
			}
		}()
	}, window)
	d.Resize(fyne.NewSize(700, 350))
	d.Show()
}

func showUpdateEntryDialog(spireServer *servers.SpireServer, entryID string, window fyne.Window, refreshData func()) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		entry, err := spireServer.GetEntry(ctx, entryID)
		if err != nil {
			fyne.Do(func() { dialog.ShowError(err, window) })
			return
		}

		fyne.Do(func() {
			dnsEntry := widget.NewEntry()
			dnsEntry.SetPlaceHolder(strings.Join(entry.DnsNames, ","))

			hintEntry := widget.NewEntry()
			hintEntry.SetPlaceHolder(entry.Hint)

			ttlEntry := widget.NewEntry()
			ttlEntry.SetPlaceHolder(fmt.Sprintf("%d", entry.X509SvidTtl))

			fedEntry := widget.NewEntry()
			fedEntry.SetPlaceHolder(strings.Join(entry.FederatesWith, ","))

			downstreamCheck := widget.NewCheck("", nil)
			downstreamCheck.SetChecked(entry.Downstream)

			adminCheck := widget.NewCheck("", nil)
			adminCheck.SetChecked(entry.Admin)

			items := []*widget.FormItem{
				leftAlignedFormItem("DNS Names:", dnsEntry),
				leftAlignedFormItem("Hint:", hintEntry),
				leftAlignedFormItem("TTL:", ttlEntry),
				leftAlignedFormItem("Federates With:", fedEntry),
				leftAlignedFormItem("Downstream:", downstreamCheck),
				leftAlignedFormItem("Admin:", adminCheck),
			}

			d := dialog.NewForm("Update Entry", "Save", "Cancel", items, func(ok bool) {
				if !ok {
					return
				}

				dnsVal := dnsEntry.Text
				if dnsVal == "" {
					dnsVal = dnsEntry.PlaceHolder
				}
				var dnsNames []string
				for _, d := range strings.Split(dnsVal, ",") {
					dt := strings.TrimSpace(d)
					if dt != "" {
						dnsNames = append(dnsNames, dt)
					}
				}

				fedVal := fedEntry.Text
				if fedVal == "" {
					fedVal = fedEntry.PlaceHolder
				}
				var fedWith []string
				for _, f := range strings.Split(fedVal, ",") {
					ft := strings.TrimSpace(f)
					if ft != "" {
						fedWith = append(fedWith, ft)
					}
				}

				ttlVal := ttlEntry.Text
				if ttlVal == "" {
					ttlVal = ttlEntry.PlaceHolder
				}
				ttl, _ := strconv.Atoi(ttlVal)

				hintVal := hintEntry.Text
				if hintVal == "" {
					hintVal = hintEntry.PlaceHolder
				}

				entry.DnsNames = dnsNames
				entry.Hint = hintVal
				entry.X509SvidTtl = int32(ttl)
				entry.JwtSvidTtl = int32(ttl)
				entry.FederatesWith = fedWith
				entry.Downstream = downstreamCheck.Checked
				entry.Admin = adminCheck.Checked

				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, err := spireServer.UpdateEntry(ctx, entry)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(err, window) })
					} else {
						refreshData()
					}
				}()
			}, window)
			d.Resize(fyne.NewSize(700, 450))
			d.Show()
		})
	}()
}
