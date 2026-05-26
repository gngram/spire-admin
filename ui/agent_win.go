package ui

import (
	"context"
	"fmt"
	"image/color"
	"reflect"
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
	spiffeIDLbl  *widget.Label
	statusLbl    *widget.Label
	check        *widget.Check
	container    *fyne.Container
}

func newAgentRowWidget() *agentRowWidget {
	r := &agentRowWidget{
		spiffeIDLbl:  widget.NewLabel(""),
		statusLbl:    widget.NewLabel(""),
		check:        widget.NewCheck("", nil),
	}
	r.spiffeIDLbl.Truncation = fyne.TextTruncateEllipsis

	r.container = container.NewBorder(nil, nil, r.check, r.statusLbl, r.spiffeIDLbl)
	r.ExtendBaseWidget(r)
	return r
}

func (r *agentRowWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.container)
}

func showAgentDetails(details interface{}, window fyne.Window) {
	v := reflect.ValueOf(details)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		dialog.ShowInformation("Agent Details", fmt.Sprintf("%+v", details), window)
		return
	}

	content := container.NewVBox()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		if !t.Field(i).IsExported() {
			continue
		}

		fieldName := t.Field(i).Name
		fieldValue := fmt.Sprintf("%v", v.Field(i).Interface())

		label := widget.NewLabel(fieldName)
		label.TextStyle.Bold = true

		value := widget.NewLabel(fieldValue)
		value.Wrapping = fyne.TextWrapWord

		content.Add(container.New(layout.NewFormLayout(), label, value))
		if i < v.NumField()-1 {
			content.Add(widget.NewSeparator())
		}
	}

	scroller := container.NewVScroll(content)
	d := dialog.NewCustom("Agent Details", "Close", scroller, window)
	d.Resize(fyne.NewSize(500, 400))
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
	selectedAgents := make(map[string]bool)

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
		func() fyne.CanvasObject { return newAgentRowWidget() },
		func(id widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*agentRowWidget)
			agent := spireServer.Agents[id]
			spiffeID := agent.SPIFFEID
			row.spiffeIDLbl.SetText(spiffeID)

			// To avoid triggering OnChanged when setting to empty
			row.check.OnChanged = nil
			row.check.SetChecked(selectedAgents[spiffeID])
			row.check.OnChanged = func(checked bool) {
				selectedAgents[spiffeID] = checked
			}
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		spiffeID := spireServer.Agents[id].SPIFFEID
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			details, err := spireServer.AgentDetails(ctx, spiffeID)
			if err != nil {
				dialog.ShowError(err, window)
			} else {
				showAgentDetails(details, window)
			}
		}()
		list.Unselect(id)
	}

	banBtn := widget.NewButton("Ban Selected", func() {
		var toBan []string
		for id, selected := range selectedAgents {
			if selected {
				toBan = append(toBan, id)
			}
		}
		if len(toBan) == 0 {
			dialog.ShowInformation("No Agents Selected", "Please select one or more agents to ban.", window)
			return
		}
		msg := fmt.Sprintf("Are you sure you want to ban %d agent(s)?", len(toBan))
		dialog.ShowConfirm("Ban Agents", msg, func(ok bool) {
			if !ok {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				for _, agentID := range toBan {
					_ = spireServer.BanAgent(ctx, agentID)
					delete(selectedAgents, agentID)
				}
				refreshData()
			}()
		}, window)
	})

	evictBtn := widget.NewButton("Evict Selected", func() {
		var toEvict []string
		for id, selected := range selectedAgents {
			if selected {
				toEvict = append(toEvict, id)
			}
		}
		if len(toEvict) == 0 {
			dialog.ShowInformation("No Agents Selected", "Please select one or more agents to evict.", window)
			return
		}
		msg := fmt.Sprintf("Are you sure you want to evict %d agent(s)?", len(toEvict))
		dialog.ShowConfirm("Evict Agents", msg, func(ok bool) {
			if !ok {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				for _, agentID := range toEvict {
					_ = spireServer.EvictAgent(ctx, agentID)
					delete(selectedAgents, agentID)
				}
				refreshData()
			}()
		}, window)
	})

	actionButtons := container.NewHBox(layout.NewSpacer(), evictBtn, banBtn)

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
			actionButtons, nil, nil,
			container.NewPadded(card),
		),
	)
}
