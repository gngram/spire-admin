package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (a *SpireAdminApp) buildSettingsContent() fyne.CanvasObject {
	title := canvas.NewText("Settings", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Application settings and preferences.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	topBar := container.NewBorder(nil, nil, titleBlock, nil)

	thickLine := canvas.NewRectangle(clrBorder)
	thickLine.SetMinSize(fyne.NewSize(0, 1))

	gapBeforeTheme := canvas.NewRectangle(color.Transparent)
	gapBeforeTheme.SetMinSize(fyne.NewSize(0, 16))

	themeSelect := widget.NewSelect([]string{"Purple", "Green", "Blue", "Gray"}, func(selected string) {
		if a.CurrentTheme == selected {
			return // Prevent infinite loop when SetSelected is called programmatically
		}
		a.CurrentTheme = selected
		SetThemeColors(selected)
		a.fyneApp.Settings().SetTheme(&spireTheme{})
		if a.RefreshUI != nil {
			a.RefreshUI()
		}
	})
	themeSelect.SetSelected(a.CurrentTheme)

	themeLabel := canvas.NewText("Select Theme:", clrText)
	themeLabel.TextSize = 14
	themeLabel.TextStyle = fyne.TextStyle{Bold: true}

	bg := canvas.NewRectangle(clrCard)
	bg.CornerRadius = 8
	bg.StrokeColor = clrBorder
	bg.StrokeWidth = 1

	card := container.NewStack(bg, container.NewPadded(themeSelect))

	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(0, 8))

	themeBlock := container.NewVBox(
		themeLabel,
		gap,
		card,
	)

	return container.NewPadded(
		container.NewBorder(
			container.NewVBox(topBar, thickLine, gapBeforeTheme),
			nil, nil, nil,
			container.NewPadded(container.NewVBox(themeBlock)),
		),
	)
}
