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

	"github.com/gngram/spire_admin/servers"
	localauthorityv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/localauthority/v1"
)

func buildLocalAuthorityContent(spireServer *servers.SpireServer, window fyne.Window) fyne.CanvasObject {
	title := canvas.NewText("Local X.509 Authorities", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Manage local signing authorities and rotation.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	activeContainer := container.NewVBox()
	preparedContainer := container.NewVBox()
	oldContainer := container.NewVBox()

	wrapSection := func(c *fyne.Container) fyne.CanvasObject {
		bg := canvas.NewRectangle(clrBg)
		bg.CornerRadius = 4
		return container.NewStack(bg, container.NewPadded(c))
	}

	mainContent := container.NewVBox(
		wrapSection(activeContainer),
		wrapSection(preparedContainer),
		wrapSection(oldContainer),
	)

	var refreshData func()
	refreshData = func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := spireServer.ShowLocalX509Authorities(ctx)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
				return
			}

			fyne.Do(func() {
				updateAuthorityTab(spireServer, window, activeContainer, resp.Active, "Active", refreshData)
				updateAuthorityTab(spireServer, window, preparedContainer, resp.Prepared, "Prepared", refreshData)
				updateAuthorityTab(spireServer, window, oldContainer, resp.Old, "Old", refreshData)
			})
		}()
	}

	newBtn := widget.NewButtonWithIcon("New", theme.ContentAddIcon(), func() {
		dialog.ShowConfirm("Rotate Authority", "Prepare and activate a new X.509 authority?", func(ok bool) {
			if !ok {
				return
			}
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				_, err := spireServer.PrepareLocalX509Authority(ctx)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, window) })
					return
				}
				refreshData()
			}()
		}, window)
	})

	topBar := container.NewBorder(nil, nil, titleBlock, container.NewHBox(newBtn))
	bg := canvas.NewRectangle(clrCard)
	bg.CornerRadius = 8
	card := container.NewStack(bg, container.NewPadded(container.NewVScroll(mainContent)))

	refreshData()

	return container.NewPadded(
		container.NewBorder(
			container.NewVBox(topBar, widget.NewSeparator(), canvas.NewRectangle(color.Transparent)),
			nil, nil, nil,
			card,
		),
	)
}

func showAuthorityInfo(label string, state *localauthorityv1.AuthorityState, w fyne.Window) {
	info := fmt.Sprintf("Authority ID         : %s\nExpires at           : %s\nUpstream authority ID: No upstream authority",
		state.AuthorityId, time.Unix(state.ExpiresAt, 0).UTC().Format("2006-01-02 15:04:05 -0700 MST"))

	entry := widget.NewMultiLineEntry()
	entry.SetText(info)
	entry.Disable()

	bgRect := canvas.NewRectangle(clrBg)
	backgroundContainer := container.NewStack(bgRect, container.NewPadded(entry))

	d := dialog.NewCustom(label+" Authority Details", "Close", backgroundContainer, w)
	d.Resize(fyne.NewSize(650, 260))
	d.Show()
}

func updateAuthorityTab(s *servers.SpireServer, w fyne.Window, cont *fyne.Container, state *localauthorityv1.AuthorityState, label string, refresh func()) {
	cont.Objects = nil
	header := widget.NewLabel(label + " Authority")
	header.TextStyle.Bold = true
	cont.Add(header)

	if state == nil {
		cont.Add(widget.NewLabel(fmt.Sprintf("No %s X.509 authority found", strings.ToLower(label))))
		cont.Refresh()
		return
	}

	idLbl := widget.NewLabel(state.AuthorityId)
	idLbl.TextStyle.Monospace = true

	tooltipTxt := canvas.NewText("", clrMuted)
	tooltipTxt.TextSize = 12
	tooltipTxt.TextStyle = fyne.TextStyle{Italic: true}
	tooltipWrapper := container.New(layout.NewGridWrapLayout(fyne.NewSize(60, 32)), container.NewCenter(tooltipTxt))

	makeBtn := func(icon fyne.Resource, tooltip string, action func()) *clickableStack {
		bg := canvas.NewRectangle(color.Transparent)
		bg.CornerRadius = 6
		ic := widget.NewIcon(icon)

		btn := newClickableStack(container.NewStack(
			container.New(layout.NewGridWrapLayout(fyne.NewSize(32, 32)), bg),
			container.NewCenter(ic),
		), action)

		btn.onHoverIn = func() {
			bg.FillColor = clrBorder
			bg.Refresh()
			tooltipTxt.Text = tooltip
			tooltipTxt.Refresh()
		}
		btn.onHoverOut = func() {
			bg.FillColor = color.Transparent
			bg.Refresh()
			tooltipTxt.Text = ""
			tooltipTxt.Refresh()
		}
		return btn
	}

	actions := container.NewHBox(tooltipWrapper)

	// Info button (Always)
	actions.Add(makeBtn(theme.InfoIcon(), "Info", func() {
		showAuthorityInfo(label, state, w)
	}))

	// Activate button (Prepared only)
	if label == "Prepared" {
		actions.Add(makeBtn(theme.ConfirmIcon(), "Activate", func() {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := s.ActivateLocalX509Authority(ctx, state.AuthorityId)
				if err != nil {
					fyne.Do(func() { dialog.ShowError(err, w) })
				}
				refresh()
			}()
		}))
	}

	// Show Delete (Taint + Revoke) button on Old tab
	if label == "Old" {
		actions.Add(makeBtn(theme.DeleteIcon(), "Delete", func() {
			dialog.ShowConfirm("Delete Authority", fmt.Sprintf("Taint and Revoke %s authority %s?", strings.ToLower(label), state.AuthorityId), func(ok bool) {
				if ok {
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
						defer cancel()
						// First Taint
						if _, err := s.TaintLocalX509Authority(ctx, state.AuthorityId); err != nil {
							fyne.Do(func() { dialog.ShowError(err, w) })
							return
						}
						// Then Revoke
						if _, err := s.RevokeLocalX509Authority(ctx, state.AuthorityId); err != nil {
							fyne.Do(func() { dialog.ShowError(err, w) })
							return
						}
						// Immediately update UI to show it is removed while we wait for refresh
						fyne.Do(func() {
							updateAuthorityTab(s, w, cont, nil, label, refresh)
						})
						time.Sleep(time.Millisecond * 500) // Brief delay for server state propagation
						refresh()
					}()
				}
			}, w)
		}))
	}

	cont.Add(container.NewBorder(nil, nil, nil, actions, idLbl))
	cont.Refresh()
}
