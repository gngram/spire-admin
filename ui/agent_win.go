package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/gngram/spidar/servers"
)

type agentRowWidget struct {
	widget.BaseWidget
	spiffeIDTxt *canvas.Text
	infoBtn     *clickableStack
	evictBtn    *clickableStack
	banBtn      *clickableStack
	container   *fyne.Container
	spiffeID    string
}

func newAgentRowWidget() *agentRowWidget {
	r := &agentRowWidget{
		spiffeIDTxt: canvas.NewText("", clrText),
	}
	r.spiffeIDTxt.Alignment = fyne.TextAlignLeading

	makeBtn := func(icon fyne.Resource) *clickableStack {
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
		}
		btn.onHoverOut = func() {
			bg.FillColor = clrBg
			bg.Refresh()
		}
		return btn
	}

	r.infoBtn = makeBtn(theme.InfoIcon())
	r.evictBtn = makeBtn(theme.DeleteIcon())
	r.banBtn = makeBtn(theme.MediaStopIcon())

	actionGroup := container.NewHBox(r.infoBtn, r.evictBtn, r.banBtn)

	content := container.NewBorder(nil, nil, nil, actionGroup, r.spiffeIDTxt)
	rowBg := canvas.NewRectangle(clrCard)
	r.container = container.NewStack(rowBg, content)
	r.ExtendBaseWidget(r)
	return r
}

func (r *agentRowWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.container)
}

// CustomLabel wraps a standard label so we can override its minimum height
type CustomLabel struct {
	widget.Label
	customHeight float32
}

func newCustomLabel(text string, bold bool, height float32) *CustomLabel {
	l := &CustomLabel{customHeight: height}
	l.Text = text
	l.Alignment = fyne.TextAlignLeading
	if bold {
		l.TextStyle.Bold = true
	}
	l.ExtendBaseWidget(l)
	return l
}

// MinSize overrides the default padding size enforced by Fyne
func (c *CustomLabel) MinSize() fyne.Size {
	min := c.Label.MinSize()
	return fyne.NewSize(min.Width, c.customHeight)
}

func showAgentInfo(details string, window fyne.Window) {
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

	if len(pairs) == 0 {
		dialog.ShowInformation("Properties", "No details available", window)
		return
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

	// 1. Define a smooth, light gray color for the background (Hex: #EBEBEB)
	bgRect := canvas.NewRectangle(clrBg)

	// 2. Put the background and the text grid into a MaxLayout container
	// MaxLayout stacks items on top of each other, filling the available area
	backgroundContainer := container.New(layout.NewMaxLayout(), bgRect, grid)

	// Wrap our customized panel inside the scroller
	scroller := container.NewVScroll(backgroundContainer)

	d := dialog.NewCustom("Agent Details", "Close", scroller, window)
	d.Resize(fyne.NewSize(600, 275))
	d.Show()
}

func buildAgentsContent(spireServer *servers.SpireServer, window fyne.Window) fyne.CanvasObject {
	title := canvas.NewText("Agents", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Manage SPIRE agents.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	var list *widget.List

	refreshData := func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := spireServer.ListAgents(ctx, true)
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if list != nil {
				fyne.Do(func() {
					list.Refresh()
				})
			}
		}()
	}

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		refreshData()
	})

	purgeBtn := widget.NewButtonWithIcon("Purge Expired", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Purge Expired Agents", "Are you sure you want to purge all expired agents?", func(ok bool) {
			if ok {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					err := spireServer.PurgeExpiredAgents(ctx)
					if err != nil {
						dialog.ShowError(err, window)
					} else {
						dialog.ShowInformation("Success", "Expired agents purged successfully.", window)
					}
					refreshData()
				}()
			}
		}, window)
	})

	topBar := container.NewBorder(nil, nil, titleBlock, container.NewHBox(refreshBtn, purgeBtn))

	list = widget.NewList(
		func() int { return len(spireServer.Agents) },
		func() fyne.CanvasObject {
			return newAgentRowWidget()
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*agentRowWidget)
			agent := spireServer.Agents[id]
			spiffeID := agent.SPIFFEID
			row.spiffeID = spiffeID
			row.spiffeIDTxt.Text = spiffeID
			row.spiffeIDTxt.Refresh()

			row.infoBtn.onTap = func() {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					info, err := spireServer.GetAgentInfo(ctx, spiffeID)
					if err != nil {
						fyne.Do(func() { dialog.ShowError(err, window) })
					} else {
						fyne.Do(func() { showAgentInfo(info, window) })
					}
				}()
			}

			row.evictBtn.onTap = func() {
				dialog.ShowConfirm("Evict Agent", fmt.Sprintf("Are you sure you want to evict %s?", spiffeID), func(ok bool) {
					if ok {
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer cancel()
							if err := spireServer.EvictAgent(ctx, spiffeID); err != nil {
								fyne.Do(func() { dialog.ShowError(err, window) })
							}
							refreshData()
						}()
					}
				}, window)
			}

			row.banBtn.onTap = func() {
				dialog.ShowConfirm("Ban Agent", fmt.Sprintf("Are you sure you want to ban %s?", spiffeID), func(ok bool) {
					if ok {
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
							defer cancel()
							if err := spireServer.BanAgent(ctx, spiffeID); err != nil {
								fyne.Do(func() { dialog.ShowError(err, window) })
							}
							refreshData()
						}()
					}
				}, window)
			}
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		list.Unselect(id)
	}

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
