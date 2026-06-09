package ui

import (
	"context"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/gngram/spire_admin/servers"
)

func buildUpstreamAuthorityContent(spireServer *servers.SpireServer, window fyne.Window) fyne.CanvasObject {
	makeLightBtn := func(label string, action func()) fyne.CanvasObject {
		btnBg := canvas.NewRectangle(clrBg)
		btnBg.CornerRadius = 6

		btnTxt := canvas.NewText(label, clrText)
		btnTxt.TextSize = 13
		btnTxt.TextStyle = fyne.TextStyle{Bold: true}

		btn := newClickableStack(container.NewStack(
			container.New(layout.NewGridWrapLayout(fyne.NewSize(100, 36)), btnBg),
			container.NewCenter(btnTxt),
		), action)

		btn.onHoverIn = func() {
			btnBg.FillColor = clrBorder
			btnBg.Refresh()
		}
		btn.onHoverOut = func() {
			btnBg.FillColor = clrBg
			btnBg.Refresh()
		}
		return btn
	}

	title := canvas.NewText("Upstream Authority", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Manage upstream X.509 authority trust.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	skidEntry := widget.NewEntry()
	skidEntry.SetPlaceHolder("Subject Key ID (Hex)")

	revokeBtn := makeLightBtn("Revoke", func() {
		if skidEntry.Text == "" {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := spireServer.RevokeUpstreamX509Authority(ctx, skidEntry.Text)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
			} else {
				fyne.Do(func() { dialog.ShowInformation("Success", "Upstream authority revoked", window) })
			}
		}()
	})

	taintBtn := makeLightBtn("Taint", func() {
		if skidEntry.Text == "" {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := spireServer.TaintUpstreamX509Authority(ctx, skidEntry.Text)
			if err != nil {
				fyne.Do(func() { dialog.ShowError(err, window) })
			} else {
				fyne.Do(func() { dialog.ShowInformation("Success", "Upstream authority tainted", window) })
			}
		}()
	})

	bg := canvas.NewRectangle(clrCard)
	bg.CornerRadius = 8
	form := container.NewVBox(
		widget.NewLabel("Enter Upstream Subject Key ID:"),
		skidEntry,
		container.NewHBox(revokeBtn, taintBtn),
	)

	card := container.NewStack(bg, container.NewPadded(form))
	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(0, 16))

	return container.NewPadded(
		container.NewBorder(
			container.NewVBox(titleBlock, widget.NewSeparator(), gap),
			nil, nil, nil,
			container.NewPadded(card),
		),
	)
}
