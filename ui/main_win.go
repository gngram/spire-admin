package ui

import (
	"fmt"
	"sync"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/gngram/spidar/logger"
	"github.com/gngram/spidar/servers"
	"github.com/gngram/spidar/config"
)

// ════════════════════════════════════════════════════════
//  DATA MODEL
// ════════════════════════════════════════════════════════


type Server struct {
	ID      int
	Name    string
	Server  *servers.SpireServer
	Status  servers.ServerHealthStatus
	statusDot *canvas.Circle
	statusLbl *widget.Label
}

// ════════════════════════════════════════════════════════
//  APPLICATION STATE
// ════════════════════════════════════════════════════════

type SpireAdminApp struct {
	fyneApp fyne.App
	window  fyne.Window

	mu        sync.RWMutex
	parentSocket string
	servers   []*Server
	Config    *config.Config
	nextID    int

	RefreshUI    func()
	CurrentTheme string
	CurrentView  string
}

func NewSpireAdminApp(parentSocket string) *SpireAdminApp {
	return &SpireAdminApp{
		Config:    config.NewConfig(),
		parentSocket: parentSocket,
		servers:   make([]*Server, 0),
		nextID:    1,
		CurrentTheme: "Purple",
		CurrentView:  "Servers",
	}
}

// ─── AddServer ───────────────────────────────────────────────────────────────
// AddServer adds a new server entry to the list and starts a background goroutine
// that periodically probes the server's TCP address and updates its status.
// Returns the newly created *Server.
func (a *SpireAdminApp) AddServer(name, host, port string) (*Server, error) {
	a.mu.Lock()
	spireServer, err := servers.NewSpireServer(host, port, a.parentSocket)
	if err != nil {
		logger.Error("Error creating server", err)
		a.mu.Unlock()
		return nil, err
	}

	srv := &Server{
		ID:      a.nextID,
		Name:    name,
		Server:  spireServer,
	}
	a.nextID++
	a.servers = append(a.servers, srv)
	a.mu.Unlock()

	spireServer.OnHealthChange = func(status servers.ServerHealthStatus) {
		a.UpdateStatus(srv.ID, status)
	}
	a.Config.AddServer(config.ServerConfig{
		Nickname: name,
		Address:  host,
		Port:     port,
	})

	return srv, nil
}

// ─── UpdateStatus ────────────────────────────────────────────────────────────
// UpdateStatus manually overrides a server's status and triggers a UI refresh.
// It is safe to call from any goroutine.
func (a *SpireAdminApp) UpdateStatus(id int, status servers.ServerHealthStatus) {
	var dot *canvas.Circle
	var lbl *widget.Label

	a.mu.Lock()
	for _, s := range a.servers {
		if s.ID == id {
			s.Status = status
			dot = s.statusDot
			lbl = s.statusLbl
			break
		}
	}
	a.mu.Unlock()

	if dot != nil || lbl != nil {
		fyne.Do(func() {
			if dot != nil {
				dot.FillColor = statusColor(status)
				dot.Refresh()
			}
			if lbl != nil {
				lbl.SetText(statusString(status))
			}
		})
	}
}

// RemoveServer stops polling and removes the server from the list.
func (a *SpireAdminApp) RemoveServer(id int) {
	a.mu.Lock()
	filtered := a.servers[:0]
	for _, s := range a.servers {
		if s.ID != id {
			filtered = append(filtered, s)
		} else {
			s.Server.Close()
		}
	}
	a.servers = filtered
	a.mu.Unlock()
}

// Snapshot returns a thread-safe copy of the server list for rendering.
func (a *SpireAdminApp) Snapshot() []*Server {
	a.mu.RLock()
	defer a.mu.RUnlock()
	snap := make([]*Server, len(a.servers))
	for i, s := range a.servers {
		cp := *s
		snap[i] = &cp
	}
	return snap
}

// ════════════════════════════════════════════════════════
//  CUSTOM THEME
// ════════════════════════════════════════════════════════

var (
	clrPurple  = color.NRGBA{R: 86, G: 61, B: 124, A: 255}
	clrSidebar = color.NRGBA{R: 72, G: 52, B: 110, A: 255}
	clrAccent  = color.NRGBA{R: 130, G: 100, B: 200, A: 255}
	clrBg      = color.NRGBA{R: 237, G: 234, B: 248, A: 255}
	clrCard    = color.NRGBA{R: 248, G: 246, B: 255, A: 255}
	clrBorder  = color.NRGBA{R: 210, G: 205, B: 235, A: 255}
	clrGreen   = color.NRGBA{R: 34, G: 197, B: 94, A: 255}
	clrYellow  = color.NRGBA{R: 234, G: 179, B: 8, A: 255}
	clrRed     = color.NRGBA{R: 239, G: 68, B: 68, A: 255}
	clrText    = color.NRGBA{R: 30, G: 20, B: 60, A: 255}
	clrMuted   = color.NRGBA{R: 120, G: 100, B: 160, A: 255}
)

type spireTheme struct{}

func (spireTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return clrBg
	case theme.ColorNameForeground:
		return clrText
	case theme.ColorNamePrimary:
		return clrPurple
	case theme.ColorNameButton:
		return clrCard
	case theme.ColorNameInputBackground:
		return clrCard
	case theme.ColorNameShadow:
		return color.NRGBA{R: 86, G: 61, B: 124, A: 30}
	}
	return theme.DefaultTheme().Color(n, v)
}
func (spireTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (spireTheme) Icon(n fyne.ThemeIconName) fyne.Resource  { return theme.DefaultTheme().Icon(n) }
func (spireTheme) Size(n fyne.ThemeSizeName) float32        { return theme.DefaultTheme().Size(n) }

func statusColor(s servers.ServerHealthStatus) color.Color {
	switch s {
	case servers.Online:
		return clrGreen
	case servers.Connecting:
		return clrYellow
	default:
		return clrRed
	}
}

func statusString(s servers.ServerHealthStatus) string {
	switch s {
	case servers.Online:
		return "Online"
	case servers.Connecting:
		return "Connecting"
	default:
		return "Offline"
	}
}

// ════════════════════════════════════════════════════════
//  UI COMPONENTS
// ════════════════════════════════════════════════════════

func (a *SpireAdminApp) buildServerRow(srv *Server, refresh func()) fyne.CanvasObject {
	// ── Server icon chip ──────────────────────────────────
	iconBg := canvas.NewRectangle(clrAccent)
	iconBg.CornerRadius = 6
	iconLbl := canvas.NewText("≡", color.White)
	iconLbl.TextSize = 16
	iconLbl.TextStyle = fyne.TextStyle{Bold: true}
	iconChip := container.NewStack(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(32, 32)), iconBg),
		container.NewCenter(iconLbl),
	)

	// ── Name ─────────────────────────────────────────────
	nameLbl := widget.NewLabel(srv.Name)
	nameLbl.TextStyle = fyne.TextStyle{Bold: true}
	nameCol := container.NewHBox(iconChip, nameLbl)

	// ── Domain ──────────────────────────────────────────
	domainLbl := widget.NewLabel(srv.Server.Domain)

	// ── Status indicator ─────────────────────────────────
	dot := canvas.NewCircle(statusColor(srv.Status))
	dotWrap := container.NewCenter(container.New(layout.NewGridWrapLayout(fyne.NewSize(10, 10)), dot))
	statusLbl := widget.NewLabel(statusString(srv.Status))
	statusLbl.TextStyle = fyne.TextStyle{Bold: true}
	statusCol := container.NewHBox(dotWrap, statusLbl)

	// Cache UI element references in the original server struct for in-place updates
	a.mu.Lock()
	for _, originalSrv := range a.servers {
		if originalSrv.ID == srv.ID {
			originalSrv.statusDot = dot
			originalSrv.statusLbl = statusLbl
			break
		}
	}
	a.mu.Unlock()

	// ── Action buttons ───────────────────────────────────
	openBtn := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		ShowServerWindow(srv.Server)
	})
	openBtn.Importance = widget.LowImportance

	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		dialog.ShowConfirm(
			"Remove Server",
			fmt.Sprintf("Remove %q from the list?", srv.Name),
			func(ok bool) {
				if ok {
					a.RemoveServer(srv.ID)
					refresh()
				}
			},
			a.window,
		)
	})
	delBtn.Importance = widget.LowImportance

	actions := container.NewHBox(openBtn, delBtn)

	row := container.New(
		layout.NewGridLayoutWithColumns(4),
		nameCol,
		domainLbl,
		statusCol,
		container.NewCenter(actions),
	)

	bg := canvas.NewRectangle(clrCard)
	bg.CornerRadius = 8
	bg.StrokeColor = clrBorder
	bg.StrokeWidth = 1

	return container.NewStack(bg, container.NewPadded(row))
}

func (a *SpireAdminApp) buildTable(refresh func()) fyne.CanvasObject {
	servers := a.Snapshot()

	makeHdr := func(text string) fyne.CanvasObject {
		l := widget.NewLabel(text)
		l.TextStyle = fyne.TextStyle{Bold: true}
		return l
	}
	header := container.NewPadded(container.New(
		layout.NewGridLayoutWithColumns(4),
		makeHdr("Name:"),
		makeHdr("Domain:"),
		makeHdr("Status:"),
		makeHdr("Actions:"),
	))

	rows := container.NewVBox()
	for _, srv := range servers {
		rows.Add(a.buildServerRow(srv, refresh))
		rows.Add(widget.NewSeparator())
	}

	tableBg := canvas.NewRectangle(clrCard)
	tableBg.CornerRadius = 12
	tableBg.StrokeColor = clrBorder
	tableBg.StrokeWidth = 1

	content := container.NewVBox(header, widget.NewSeparator(), rows)
	return container.NewStack(tableBg, container.NewPadded(content))
}

func (a *SpireAdminApp) showAddServerDialog(refresh func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Admin Server")

	addrEntry := widget.NewEntry()
	addrEntry.SetPlaceHolder("127.0.0.1")

	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("8081")

	items := []*widget.FormItem{
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Address", addrEntry),
		widget.NewFormItem("Port", portEntry),
	}

	d := dialog.NewForm("Server", "Add", "Cancel", items, func(ok bool) {
		inputErr := fmt.Errorf("Invalid input!")
		if !ok || nameEntry.Text == "" || addrEntry.Text == "" {
			
			dialog.ShowError(inputErr, a.window)
			return
		}
		addr := addrEntry.Text
		port := portEntry.Text
		_, err := a.AddServer(nameEntry.Text, addr, port)
		if err != nil {
			dialog.ShowError(inputErr, a.window)
			return
		}
		refresh()
	}, a.window)
	d.Resize(fyne.NewSize(300, 200))
	d.Show()
}

func (a *SpireAdminApp) buildSidebar() fyne.CanvasObject {
	bg := canvas.NewRectangle(clrSidebar)

	logoTxt := canvas.NewText("spire-admin", color.White)
	logoTxt.TextSize = 15
	logoTxt.TextStyle = fyne.TextStyle{Bold: true}
	logoIcon := widget.NewIcon(theme.ComputerIcon())
	logoRow := container.NewPadded(container.NewHBox(logoIcon, logoTxt))

	makeNavItem := func(icon fyne.Resource, label string, active bool, action func()) fyne.CanvasObject {
		btn := widget.NewButtonWithIcon(label, icon, action)
		btn.Alignment = widget.ButtonAlignLeading
		if active {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		return container.NewPadded(btn)
	}

	nav := container.NewVBox(
		logoRow,
		widget.NewSeparator(),
		makeNavItem(theme.ComputerIcon(), "Servers", a.CurrentView == "Servers", func() {
			a.CurrentView = "Servers"
			if a.RefreshUI != nil {
				a.RefreshUI()
			}
		}),
		makeNavItem(theme.SettingsIcon(), "Settings", a.CurrentView == "Settings", func() {
			a.CurrentView = "Settings"
			if a.RefreshUI != nil {
				a.RefreshUI()
			}
		}),
		makeNavItem(theme.FileTextIcon(), "Logs", a.CurrentView == "Logs", func() {
			a.CurrentView = "Logs"
			if a.RefreshUI != nil {
				a.RefreshUI()
			}
		}),
		makeNavItem(theme.InfoIcon(), "About", false, func() {}),
	)

	adminBtn := widget.NewButtonWithIcon("Admin ▾", theme.AccountIcon(), func() {})
	adminBtn.Alignment = widget.ButtonAlignLeading
	adminBtn.Importance = widget.LowImportance
	bottom := container.NewVBox(widget.NewSeparator(), container.NewPadded(adminBtn))

	inner := container.NewBorder(nav, bottom, nil, nil)
	return container.NewStack(bg, inner)
}

func (a *SpireAdminApp) buildContent(refresh func()) fyne.CanvasObject {
	if a.CurrentView == "Settings" {
		return a.buildSettingsContent()
	} else if a.CurrentView == "Logs" {
		return a.buildLogsContent()
	}

	servers := a.Snapshot()

	// ── Header ───────────────────────────────────────────
	title := canvas.NewText("Servers", clrText)
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := canvas.NewText("Manage the list of Spire servers you want to administrate.", clrMuted)
	subtitle.TextSize = 12
	titleBlock := container.NewVBox(title, subtitle)

	addBtn := widget.NewButtonWithIcon("Server", theme.ContentAddIcon(), func() {
		a.showAddServerDialog(refresh)
	})
	addBtn.Importance = widget.HighImportance

	loadBtn := widget.NewButtonWithIcon("Load", theme.FolderOpenIcon(), func() {
		dialog.ShowInformation("Load Configuations", "Config loading is not implemented in this demo.", a.window)
	})
	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		dialog.ShowInformation("Save Configuations", "Configuration saved (demo).", a.window)
	})

	topBar := container.NewBorder(nil, nil,
		titleBlock,
		container.NewCenter(container.NewHBox(addBtn, loadBtn, saveBtn)),
	)

	// ── Table ────────────────────────────────────────────
	table := a.buildTable(refresh)

	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(0, 20))

	// ── Footer ───────────────────────────────────────────
	footerLbl := widget.NewLabel(fmt.Sprintf("Total Servers: %d", len(servers)))

	return container.NewPadded(
		container.NewBorder(
			container.NewVBox(topBar, widget.NewSeparator(), gap),
			container.NewPadded(footerLbl),
			nil, nil,
			container.NewPadded(table),
		),
	)
}

// ════════════════════════════════════════════════════════
//  MAIN
// ════════════════════════════════════════════════════════

func OpenDashboard(parentSocket string) {
	adm := NewSpireAdminApp(parentSocket)
	adm.fyneApp = app.New()
	adm.fyneApp.Settings().SetTheme(&spireTheme{})

	adm.fyneApp.Lifecycle().SetOnStopped(func() {
		for _, s := range adm.Snapshot() {
			s.Server.Close()
		}
	})

	adm.window = adm.fyneApp.NewWindow("spire-admin")
	adm.window.Resize(fyne.NewSize(1020, 700))

	// Seed with demo data
	//adm.AddServer("Production Server", "spire-prod.example.com", "8081")
	//adm.AddServer("Staging Server", "spire-staging.example.com", "8081")
	//adm.AddServer("Development Server", "spire-dev.example.com", "8081")
	//adm.AddServer("Test Server", "spire-test.example.com", "8081")
	//adm.AddServer("Local Server", "localhost", "8081")

	var buildUI func()
	buildUI = func() {
		sidebar := adm.buildSidebar()
		content := adm.buildContent(buildUI)

		split := container.NewHSplit(sidebar, content)
		split.SetOffset(0.22)
		adm.window.SetContent(split)
	}
	adm.RefreshUI = buildUI

	buildUI()
	adm.window.ShowAndRun()
}
