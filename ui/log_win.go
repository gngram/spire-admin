package ui

import (
	"image/color"
	"io"
	"log"
	"os"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var (
	logBuffer string
	logMu     sync.Mutex
	logLabel  *widget.Label
	logScroll *container.Scroll
)

func init() {
	logBuffer = "System initialized...\nWaiting for events...\n"
	// Pipe standard log output to both the console and our UI logger
	log.SetOutput(io.MultiWriter(os.Stdout, &uiLogWriter{}))
}

type uiLogWriter struct{}

func (w *uiLogWriter) Write(p []byte) (n int, err error) {
	logMu.Lock()
	logBuffer += string(p)
	if len(logBuffer) > 50000 { // Keep the buffer size manageable
		logBuffer = logBuffer[len(logBuffer)-50000:]
	}
	text := logBuffer
	lbl := logLabel
	scr := logScroll
	logMu.Unlock()

	if lbl != nil {
		fyne.Do(func() {
			lbl.SetText(text)
			if scr != nil {
				scr.ScrollToBottom()
			}
		})
	}
	return len(p), nil
}

func (a *SpireAdminApp) buildLogsContent() fyne.CanvasObject {
	title := canvas.NewText("Logs", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("View application events and system logs.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	topBar := container.NewBorder(nil, nil, titleBlock, nil)

	thickLine := canvas.NewRectangle(clrBorder)
	thickLine.SetMinSize(fyne.NewSize(0, 1))

	gapBefore := canvas.NewRectangle(color.Transparent)
	gapBefore.SetMinSize(fyne.NewSize(0, 16))

	logMu.Lock()
	logLabel = widget.NewLabel(logBuffer)
	logScroll = container.NewVScroll(logLabel)
	logMu.Unlock()

	bg := canvas.NewRectangle(clrCard)
	bg.CornerRadius = 8
	bg.StrokeColor = clrBorder
	bg.StrokeWidth = 1

	card := container.NewStack(bg, container.NewPadded(logScroll))

	return container.NewPadded(
		container.NewBorder(
			container.NewVBox(topBar, thickLine, gapBefore),
			nil, nil, nil,
			container.NewPadded(card),
		),
	)
}
