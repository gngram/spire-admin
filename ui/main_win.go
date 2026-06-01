package ui

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/gngram/spire_admin/logger"
	"github.com/gngram/spire_admin/servers"
)

// ════════════════════════════════════════════════════════
//  DATA MODEL
// ════════════════════════════════════════════════════════

type Server struct {
	ID         int
	Name       string
	Server     *servers.SpireServer
	Status     servers.ServerHealthStatus
	Domain     string
	statusDot  *canvas.Circle
	statusLbl  *widget.Label
	domainLbl  *widget.Label
	lastUpdLbl *widget.Label
}

// ════════════════════════════════════════════════════════
//  APPLICATION STATE
// ════════════════════════════════════════════════════════

type SpireAdminApp struct {
	fyneApp fyne.App
	window  fyne.Window

	mu           sync.RWMutex
	parentSocket string
	servers      []*Server
	nextID       int

	RefreshUI    func()
	CurrentTheme string
	CurrentView  string
	windowWidth  uint16
	windowHeight uint16
}

func NewSpireAdminApp(parentSocket string) *SpireAdminApp {
	return &SpireAdminApp{
		parentSocket: parentSocket,
		servers:      make([]*Server, 0),
		nextID:       1,
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
		ID:     a.nextID,
		Name:   name,
		Server: spireServer,
		Domain: spireServer.Domain,
	}
	a.nextID++
	a.servers = append(a.servers, srv)
	a.mu.Unlock()

	spireServer.OnHealthChange = func(status servers.ServerHealthStatus) {
		a.UpdateStatus(srv.ID, status)
	}

	return srv, nil
}

// ─── UpdateStatus ────────────────────────────────────────────────────────────
// UpdateStatus manually overrides a server's status and triggers a UI refresh.
// It is safe to call from any goroutine.
func (a *SpireAdminApp) UpdateStatus(id int, status servers.ServerHealthStatus) {
	var dot *canvas.Circle
	var lbl *widget.Label
	var dLbl *widget.Label
	var luLbl *widget.Label
	var domain string
	var lastUpdated string

	a.mu.Lock()
	for _, s := range a.servers {
		if s.ID == id {
			s.Status = status
			s.Domain = s.Server.Domain
			dot = s.statusDot
			lbl = s.statusLbl
			dLbl = s.domainLbl
			luLbl = s.lastUpdLbl
			domain = s.Domain
			lastUpdated = s.Server.GetLastUpdated().Format("15:04:05")
			break
		}
	}
	a.mu.Unlock()

	if dot != nil || lbl != nil || dLbl != nil || luLbl != nil {
		fyne.Do(func() {
			if dot != nil {
				dot.FillColor = statusColor(status)
				dot.Refresh()
			}
			if lbl != nil {
				lbl.SetText(statusString(status))
			}
			if dLbl != nil {
				dLbl.SetText(domain)
			}
			if luLbl != nil {
				luLbl.SetText(lastUpdated)
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

type clickableStack struct {
	widget.BaseWidget
	content    fyne.CanvasObject
	onTap      func()
	onHoverIn  func()
	onHoverOut func()
}

func newClickableStack(content fyne.CanvasObject, onTap func()) *clickableStack {
	c := &clickableStack{content: content, onTap: onTap}
	c.ExtendBaseWidget(c)
	return c
}

func (c *clickableStack) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.content)
}

func (c *clickableStack) Tapped(_ *fyne.PointEvent) {
	if c.onTap != nil {
		c.onTap()
	}
}

func (c *clickableStack) TappedSecondary(_ *fyne.PointEvent) {}

func (c *clickableStack) MouseIn(_ *desktop.MouseEvent) {
	if c.onHoverIn != nil {
		c.onHoverIn()
	}
}

func (c *clickableStack) MouseOut() {
	if c.onHoverOut != nil {
		c.onHoverOut()
	}
}

func (c *clickableStack) MouseMoved(_ *desktop.MouseEvent) {}

func showServerInfo(srv *Server, window fyne.Window) {
	details := fmt.Sprintf("Name : %s\nAddress : %s\nPort : %s\nDomain : %s\nStatus : %s",
		srv.Name, srv.Server.Address, srv.Server.Port, srv.Server.Domain, statusString(srv.Server.HealthStatus))

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
	backgroundContainer := container.New(layout.NewMaxLayout(), bgRect, grid)
	scroller := container.NewVScroll(backgroundContainer)

	d := dialog.NewCustom("Server Details", "Close", scroller, window)
	d.Resize(fyne.NewSize(400, 250))
	d.Show()
}

func (a *SpireAdminApp) buildServerRow(srv *Server, refresh func()) fyne.CanvasObject {
	// ── Server icon chip ──────────────────────────────────
	iconBg := canvas.NewRectangle(clrAccent)
	iconBg.CornerRadius = 6
	iconLbl := canvas.NewText("≡", color.White)
	iconLbl.TextSize = 16
	iconLbl.TextStyle = fyne.TextStyle{Bold: true}
	iconChip := newClickableStack(container.NewStack(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(32, 32)), iconBg),
		container.NewCenter(iconLbl),
	), func() {
		ShowServerWindow(srv.Server, a.windowWidth, a.windowHeight)
	})

	iconChip.onHoverIn = func() {
		iconBg.FillColor = clrPurple
		iconBg.Refresh()
	}
	iconChip.onHoverOut = func() {
		iconBg.FillColor = clrAccent
		iconBg.Refresh()
	}

	// ── Name ─────────────────────────────────────────────
	nameLbl := widget.NewLabel(srv.Name)
	nameLbl.TextStyle = fyne.TextStyle{Bold: true}
	nameClickable := newClickableStack(nameLbl, func() {
		showServerInfo(srv, a.window)
	})
	nameCol := container.NewHBox(iconChip, nameClickable)

	// ── Domain ──────────────────────────────────────────
	domainLbl := widget.NewLabel(srv.Domain)

	// ── Status indicator ─────────────────────────────────
	dot := canvas.NewCircle(statusColor(srv.Status))
	dotWrap := container.NewCenter(container.New(layout.NewGridWrapLayout(fyne.NewSize(10, 10)), dot))
	statusLbl := widget.NewLabel(statusString(srv.Status))
	statusLbl.TextStyle = fyne.TextStyle{Bold: true}
	statusCol := container.NewHBox(dotWrap, statusLbl)

	// ── Last Updates ─────────────────────────────────────
	lastUpdatedStr := srv.Server.GetLastUpdated().Format("15:04:05")
	lastUpdLbl := widget.NewLabel(lastUpdatedStr)

	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		go func() {
			srv.Server.FetchInfo()
			a.UpdateStatus(srv.ID, srv.Server.GetCachedHealthStatus())
		}()
	})
	refreshBtn.Importance = widget.LowImportance

	// Cache UI element references in the original server struct for in-place updates
	a.mu.Lock()
	for _, originalSrv := range a.servers {
		if originalSrv.ID == srv.ID {
			originalSrv.statusDot = dot
			originalSrv.statusLbl = statusLbl
			originalSrv.domainLbl = domainLbl
			originalSrv.lastUpdLbl = lastUpdLbl
			break
		}
	}
	a.mu.Unlock()

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

	actions := container.NewHBox(lastUpdLbl, refreshBtn, delBtn)

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
		makeHdr("Last Updates:"),
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
		if !ok {
			return
		}
		if nameEntry.Text == "" || addrEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Invalid input!"), a.window)
			return
		}
		addr := addrEntry.Text
		port := portEntry.Text

		for _, srv := range a.Snapshot() {
			if srv.Server.Address == addr && srv.Server.Port == port {
				dialog.ShowError(fmt.Errorf("Server %s:%s already exists", addr, port), a.window)
				return
			}
		}

		_, err := a.AddServer(nameEntry.Text, addr, port)
		if err != nil {
			dialog.ShowError(err, a.window)
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

	serverList := a.Snapshot()

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
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()

			cfg, err := servers.LoadServersConfig(reader.URI().Path())
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if cfg != nil {
				for _, sc := range cfg.Servers {
					exists := false
					for _, srv := range a.Snapshot() {
						if srv.Server.Address == sc.Address && srv.Server.Port == sc.Port {
							exists = true
							break
						}
					}
					if exists {
						dialog.ShowError(fmt.Errorf("Server %s:%s already exists", sc.Address, sc.Port), a.window)
						continue
					}
					if _, err := a.AddServer(sc.Nickname, sc.Address, sc.Port); err != nil {
						dialog.ShowError(fmt.Errorf("Failed to add %s:%s: %v", sc.Address, sc.Port, err), a.window)
					}
				}
				refresh()
			}
		}, a.window)
		fd.Resize(fyne.NewSize(700, 500))
		fd.Show()
		fd.SetView(dialog.ListView)
	})
	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, a.window)
				return
			}
			if writer == nil {
				return
			}
			defer writer.Close()

			serverMap := make(map[string]*servers.SpireServer)
			for _, s := range a.Snapshot() {
				serverMap[s.Name] = s.Server
			}
			if err := servers.SaveServersConfig(writer.URI().Path(), serverMap); err != nil {
				dialog.ShowError(err, a.window)
			} else {
				dialog.ShowInformation("Save Configurations", "Configuration successfully saved.", a.window)
			}
		}, a.window)
		fd.Resize(fyne.NewSize(700, 500))
		fd.Show()
		fd.SetView(dialog.ListView)
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
	footerLbl := widget.NewLabel(fmt.Sprintf("Total Servers: %d", len(serverList)))

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

func OpenDashboard(parentSocket string, width uint16, height uint16) {
	adm := NewSpireAdminApp(parentSocket)
	adm.windowWidth = width
	adm.windowHeight = height
	adm.fyneApp = app.NewWithID("com.github.gngram.spire_admin")
	adm.fyneApp.Settings().SetTheme(&spireTheme{})

	adm.fyneApp.Lifecycle().SetOnStopped(func() {
		for _, s := range adm.Snapshot() {
			s.Server.Close()
		}
	})

	adm.window = adm.fyneApp.NewWindow("spire-admin")
	adm.window.Resize(fyne.NewSize(float32(width), float32(height)))
	adm.window.CenterOnScreen()
	adm.window.SetMaster()
	adm.window.SetFixedSize(true)

	// Seed with demo data
	// adm.AddServer("Production Server", "spire-prod.example.com", "8081")
	// adm.AddServer("Staging Server", "spire-staging.example.com", "8081")
	// adm.AddServer("Development Server", "spire-dev.example.com", "8081")
	// adm.AddServer("Test Server", "spire-test.example.com", "8081")
	// adm.AddServer("Local Server", "localhost", "8081")

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
