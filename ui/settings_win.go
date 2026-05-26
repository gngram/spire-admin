package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)


func SetThemeColors(name string) {
	switch name {
	case "Green":
		clrPurple = color.NRGBA{R: 56, G: 124, B: 61, A: 255}
		clrSidebar = color.NRGBA{R: 45, G: 100, B: 50, A: 255}
		clrAccent = color.NRGBA{R: 80, G: 180, B: 90, A: 255}
		clrBg = color.NRGBA{R: 234, G: 248, B: 234, A: 255}
		clrCard = color.NRGBA{R: 246, G: 255, B: 246, A: 255}
		clrBorder = color.NRGBA{R: 205, G: 235, B: 205, A: 255}
		clrText = color.NRGBA{R: 20, G: 60, B: 20, A: 255}
		clrMuted = color.NRGBA{R: 100, G: 160, B: 100, A: 255}
	case "Blue":
		clrPurple = color.NRGBA{R: 61, G: 86, B: 124, A: 255}
		clrSidebar = color.NRGBA{R: 52, G: 72, B: 110, A: 255}
		clrAccent = color.NRGBA{R: 100, G: 130, B: 200, A: 255}
		clrBg = color.NRGBA{R: 234, G: 237, B: 248, A: 255}
		clrCard = color.NRGBA{R: 246, G: 248, B: 255, A: 255}
		clrBorder = color.NRGBA{R: 205, G: 210, B: 235, A: 255}
		clrText = color.NRGBA{R: 20, G: 30, B: 60, A: 255}
		clrMuted = color.NRGBA{R: 100, G: 120, B: 160, A: 255}
	case "Gray":
		clrPurple = color.NRGBA{R: 80, G: 80, B: 80, A: 255}
		clrSidebar = color.NRGBA{R: 60, G: 60, B: 60, A: 255}
		clrAccent = color.NRGBA{R: 130, G: 130, B: 130, A: 255}
		clrBg = color.NRGBA{R: 240, G: 240, B: 240, A: 255}
		clrCard = color.NRGBA{R: 250, G: 250, B: 250, A: 255}
		clrBorder = color.NRGBA{R: 215, G: 215, B: 215, A: 255}
		clrText = color.NRGBA{R: 40, G: 40, B: 40, A: 255}
		clrMuted = color.NRGBA{R: 140, G: 140, B: 140, A: 255}
	default: // Purple
		clrPurple = color.NRGBA{R: 86, G: 61, B: 124, A: 255}
		clrSidebar = color.NRGBA{R: 72, G: 52, B: 110, A: 255}
		clrAccent = color.NRGBA{R: 130, G: 100, B: 200, A: 255}
		clrBg = color.NRGBA{R: 237, G: 234, B: 248, A: 255}
		clrCard = color.NRGBA{R: 248, G: 246, B: 255, A: 255}
		clrBorder = color.NRGBA{R: 210, G: 205, B: 235, A: 255}
		clrText = color.NRGBA{R: 30, G: 20, B: 60, A: 255}
		clrMuted = color.NRGBA{R: 120, G: 100, B: 160, A: 255}
	}
}

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
