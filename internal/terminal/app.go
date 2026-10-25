package terminal

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"deepstateclicker/internal/game"
	"deepstateclicker/internal/save"
)

const savePath = "saves/autosave.json"

type screen int
const (
	screenMenu screen = iota
	screenIntro
	screenGame
)
type tickMsg time.Time
type uiPulseMsg time.Time

type uiLayout struct {
	Width, Height int
	TabsY, ContentY, ExploitY, StoreCapacityY, StoreCalibrationY, StoreAutomationY int
	MessageFirstY, MessageLastY, MessageChoiceFirstY, TickerY int
	SideBySide bool
}

type App struct {
	state game.State
	screen screen
	width, height int
	menu int
	message string
	uiFrame int64
	newsHeadline string
	newsStartedAt time.Time
	injectFlashUntil time.Time
	injectMouseHeld bool
	injectHeldRed bool
	mainView string
	selectedUpgrade int
	selectedRackModule int
	rackFocusRight bool
	rackLeftScroll int
	rackRightScroll int
}

func New(state game.State) *App {
	state.Normalize()
	if state.CurrentTab == game.TabStore { state.CurrentTab = game.TabPrint }
	return &App{state: state, screen: screenMenu, width: 80, height: 24}
}
func (a *App) Run() error {
	_, err := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
func (a *App) Init() tea.Cmd { return nil }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			a.injectMouseHeld = false
			a.injectHeldRed = false
		}
		if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonLeft && a.injectMouseHeld {
			// Keep the failed-injection feedback visible for the duration of a held click.
		}
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			a.injectMouseHeld = a.isInjectAt(msg.X, msg.Y)
			a.injectHeldRed = false
			return a, a.handleMouse(msg.X, msg.Y)
		}
		if a.screen == screenGame && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) && a.mouseOnTabRow(msg.Y) {
			delta := 1
			if msg.Button == tea.MouseButtonWheelUp { delta = -1 }
			a.cycleTab(delta)
		}
	case tickMsg:
		if a.screen == screenGame {
			previousNews := a.state.NewsIndex
			a.state.Advance(time.Time(msg))
			if a.state.NewsIndex != previousNews { a.startNewsDispatch() }
			return a, tick()
		}
	case uiPulseMsg:
		if a.screen == screenGame {
			a.uiFrame++
			return a, uiPulse()
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			a.autosave()
			return a, tea.Quit
		case "up", "k":
			if a.screen == screenMenu && a.menu > 0 { a.menu-- } else if a.screen == screenGame && a.state.CurrentTab == game.TabMessages { a.selectMessage(-1) } else if a.screen == screenGame { a.moveMainSelection(-1) }
		case "down", "j":
			if a.screen == screenMenu && a.menu < 2 { a.menu++ } else if a.screen == screenGame && a.state.CurrentTab == game.TabMessages { a.selectMessage(1) } else if a.screen == screenGame { a.moveMainSelection(1) }
		case "left":
			if a.screen == screenGame && a.mainView == "rack" { a.rackFocusRight = false }
		case "right":
			if a.screen == screenGame && a.mainView == "rack" { a.rackFocusRight = true }
		case "enter":
			switch a.screen {
			case screenMenu:
				return a, a.chooseMenu()
			case screenIntro:
				a.screen, a.message = screenGame, ""
				return a, tea.Batch(tick(), uiPulse())
			case screenGame:
				if a.state.CurrentTab == game.TabMessages { a.message = "" } else { a.activateMainSelection() }
			}
		case "n":
			if a.screen == screenMenu { a.screen = screenIntro }
		case "l":
			if a.screen == screenMenu {
				a.loadGame()
				if a.screen == screenGame { return a, tea.Batch(tick(), uiPulse()) }
			}
		case "q":
			if a.screen == screenMenu { return a, tea.Quit }
			a.autosave()
			return a, tea.Quit
		case "esc":
			switch a.screen {
			case screenMenu: return a, tea.Quit
			case screenIntro: a.screen = screenMenu
			case screenGame: _ = a.state.OpenTab(game.TabPrint); a.mainView = "print"; a.message = ""
			}
		default:
			if a.screen == screenGame { return a, a.handleGameKey(msg.String()) }
		}
	}
	return a, nil
}

func tick() tea.Cmd { return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }
func uiPulse() tea.Cmd { return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg { return uiPulseMsg(t) }) }

func (a *App) chooseMenu() tea.Cmd {
	switch a.menu {
	case 0:
		a.state = game.NewState(time.Now())
		a.screen, a.message = screenIntro, ""
	case 1:
		a.loadGame()
		if a.screen == screenGame { return tea.Batch(tick(), uiPulse()) }
	case 2: return tea.Quit
	}
	return nil
}
func (a *App) loadGame() {
	loaded, err := save.Read(savePath)
	if err != nil { a.message = "No readable save found. Choose NEW OPERATION to begin."; return }
	loaded.Normalize()
	if loaded.CurrentTab == game.TabStore { loaded.CurrentTab = game.TabPrint }
	a.state, a.screen, a.message = loaded, screenGame, "Operation loaded."
}

func (a *App) layout() uiLayout {
	w, h := a.width, a.height
	if w < 24 { w = 24 }
	if h < 12 { h = 12 }
	sideBySide := w >= 100
	panelY := 4
	return uiLayout{
		Width:w, Height:h, TabsY:2, ContentY:panelY,
		ExploitY:panelY+8, StoreCapacityY:panelY+11, StoreCalibrationY:panelY+12, StoreAutomationY:panelY+13,
		MessageFirstY:panelY+1, MessageLastY:h-5, MessageChoiceFirstY:h-9, TickerY:h-2,
		SideBySide:sideBySide,
	}
}

// The same layout object drives both drawing and mouse hit regions.
var ansiSequences = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

func plainViewLines(view string) []string {
	plain := ansiSequences.ReplaceAllString(view, "")
	return strings.Split(plain, "\n")
}

func (a *App) handleMouse(x, y int) tea.Cmd {
	// Bubble Tea mouse coordinates already include the terminal's rendered rows.
	// Do not offset Y: doing so made every hitbox one row lower than its label.

	// Hit testing uses the actual rendered view, including its padding and any
	// rows added by wrapping. This avoids hard-coded coordinates drifting when
	// money values, messages, or terminal dimensions change.
	lines := plainViewLines(a.View())
	if y < 0 || y >= len(lines) {
		return nil
	}
	line := lines[y]

	switch a.screen {
	case screenMenu:
		switch {
		case strings.Contains(line, "NEW OPERATION"):
			a.menu = 0
			return a.chooseMenu()
		case strings.Contains(line, "LOAD OPERATION"):
			a.menu = 1
			return a.chooseMenu()
		case strings.Contains(line, "QUIT"):
			a.menu = 2
			return a.chooseMenu()
		}
	case screenIntro:
		if y >= 3 {
			a.screen, a.message = screenGame, ""
			return tea.Batch(tick(), uiPulse())
		}
	case screenGame:
		// The tab strip is identified by its rendered labels, not a fixed row.
		if strings.Contains(line, "[MAIN]") || strings.Contains(line, "[MESSAGES") {
			arrowAt := strings.LastIndex(line, ">")
			if arrowAt >= 0 && x >= arrowAt {
				a.cycleTab(1)
				return nil
			}
			messagesAt := strings.Index(line, "MESSAGES")
			if messagesAt >= 0 && x >= messagesAt {
				_ = a.state.OpenTab(game.TabMessages)
				if m := a.state.SelectedMessage(); m != nil { _ = a.state.SelectMessage(m.ID, time.Now()) }
			} else { _ = a.state.OpenTab(game.TabPrint) }
			a.message = ""
			return nil
		}

		switch a.state.CurrentTab {
		case game.TabPrint, game.TabStore:
			if strings.Contains(line, "INJECT") && a.mainView != "rack" && a.mainView != "upgrades" { return a.handleGameKey("x") }
			if a.mainView == "upgrades" {
				for row, text := range lines {
					if strings.Contains(text,"MODULE CATALOGUE") {
						idx := (y-row-1)/2
						if idx >= 0 && idx < len(game.ModuleCatalog) { a.selectedUpgrade = idx; return nil }
					}
				}
			}
			if a.mainView == "rack" {
				for i, spec := range game.ModuleCatalog {
					if strings.Contains(line,spec.Name) { a.selectedUpgrade = i; a.rackFocusRight = false; return nil }
				}
				for i, mod := range a.installedModules() {
					name := mod.ID
					for _, spec := range game.ModuleCatalog { if spec.ID==mod.ID { name=spec.Name; break } }
					if mod.ID=="legacy_controller" { name="LEGACY PRINT CONTROLLER" }
					if strings.Contains(line,name) || (mod.Jammed && strings.Contains(line,"JAMMED")) { a.selectedRackModule=i; a.rackFocusRight=true; return nil }
				}
			}
			switch {
			case strings.Contains(line, "[U] UPGRADES"): a.mainView = "upgrades"; a.selectedUpgrade = 0
			case strings.Contains(line, "[R] HARDWARE RACK"): a.mainView = "rack"; a.rackFocusRight = false
			case strings.Contains(line, "[B] BACK TO OPERATIONS"): a.mainView = "print"
			case a.mainView == "upgrades" && strings.Contains(line, "BUY"): a.activateMainSelection()
			case a.mainView == "rack" && (strings.Contains(line, "AVAILABLE MODULES") || strings.Contains(line, "INSTALLED RACK")):
				a.rackFocusRight = x > lWidthForRack(a.width)
			}
		case game.TabMessages:
			switch {
			case strings.Contains(line, "[I] Leave unanswered"):
				return a.handleGameKey("i")
			case strings.Contains(line, "[1] "):
				return a.handleGameKey("1")
			case strings.Contains(line, "[2] "):
				return a.handleGameKey("2")
			case strings.Contains(line, "[3] "):
				return a.handleGameKey("3")
			}

			// Only the side-by-side inbox actually renders a clickable message list.
			l := a.layout()
			if l.SideBySide && x < max(28, min(36, l.Width/3))+2 {
				for row, text := range lines {
					if strings.Contains(text, "INBOX") {
						idx := y - row - 1
						if idx >= 0 && idx < min(len(a.state.Messages), 5) {
							_ = a.state.SelectMessage(a.state.Messages[idx].ID, time.Now())
						}
						return nil
					}
				}
			}
		}
	}
	return nil
}
func (a *App) selectMessage(delta int) {
	if len(a.state.Messages) == 0 { return }
	current := 0
	for i, m := range a.state.Messages { if m.ID == a.state.SelectedMessageID { current = i; break } }
	current = max(0, min(len(a.state.Messages)-1, current+delta))
	_ = a.state.SelectMessage(a.state.Messages[current].ID, time.Now())
}

func (a *App) handleGameKey(key string) tea.Cmd {
	now := time.Now()
	switch key {
	case "p", " ", "x":
		if err := a.state.QueueExploit(now); err != nil {
			if err == game.ErrWindowClosed {
				a.injectFlashUntil = now.Add(150 * time.Millisecond)
				a.injectHeldRed = a.injectMouseHeld
				a.message = ""
			} else { a.message = err.Error() }
		} else { a.injectHeldRed = false; a.message = "" }
	case "1":
		if a.state.CurrentTab == game.TabPrint && a.mainView == "upgrades" { a.buySelectedModule(now) } else if a.state.CurrentTab == game.TabStore || a.state.CurrentTab == game.TabPrint {
			a.mainView = "upgrades"; a.selectedUpgrade = 0
		} else if a.state.CurrentTab == game.TabMessages {
			if err := a.state.ReplyToSelected(0, now); err != nil { a.message = err.Error() } else { a.message = "Reply sent. The inbox is quiet for now." }
		}
	case "2":
		if a.state.CurrentTab == game.TabPrint && a.mainView == "upgrades" { a.buySelectedModule(now) } else if a.state.CurrentTab == game.TabStore || a.state.CurrentTab == game.TabPrint {
			a.mainView = "rack"
		} else if a.state.CurrentTab == game.TabMessages {
			if err := a.state.ReplyToSelected(1, now); err != nil { a.message = err.Error() } else { a.message = "Reply sent. The inbox is quiet for now." }
		}
	case "3":
		if a.state.CurrentTab == game.TabPrint && a.mainView == "upgrades" { a.buySelectedModule(now) } else if a.state.CurrentTab == game.TabStore || a.state.CurrentTab == game.TabPrint {
			a.mainView = "print"
		} else if a.state.CurrentTab == game.TabMessages {
			if err := a.state.ReplyToSelected(2, now); err != nil { a.message = err.Error() } else { a.message = "Reply sent. The inbox is quiet for now." }
		}
	case "s": _ = a.state.OpenTab(game.TabPrint); a.mainView = "print"; a.message = ""
	case "u": if a.state.CurrentTab == game.TabPrint { a.mainView = "upgrades"; a.selectedUpgrade = 0; a.message = "" }
	case "r": if a.state.CurrentTab == game.TabPrint { a.mainView = "rack"; a.rackFocusRight = false; a.message = "" }
	case "m", "e":
		if err := a.state.OpenTab(game.TabMessages); err != nil { a.message = err.Error() } else {
			if m := a.state.SelectedMessage(); m != nil { _ = a.state.SelectMessage(m.ID, now) }
			a.message = ""
		}
	case "i":
		if err := a.state.IgnoreSelectedMessage(now); err != nil { a.message = err.Error() } else { a.message = "You leave the message unanswered." }
	case "b", "esc": _ = a.state.OpenTab(game.TabPrint); a.mainView = "print"; a.message = ""
	case "v":
		if err := save.Write(savePath, a.state); err != nil { a.message = "Save failed: "+err.Error() } else { a.message = "Saved to "+savePath }
	case "h", "?": a.message = "X/P inject  •  MAIN: 1–3 upgrades  •  M messages  •  1–3 reply  •  I ignore  •  V save  •  Q quit"
	case "q": a.autosave(); return tea.Quit
	}
	return nil
}
func (a *App) autosave() { if a.screen == screenGame { if err := save.Write(savePath, a.state); err != nil { a.message = "Autosave failed: "+err.Error() } } }

func (a *App) View() string {
	switch a.screen {
	case screenMenu: return a.menuView()
	case screenIntro: return a.introView()
	default: return a.gameView()
	}
}

// Centralized color vocabulary. The current terminal theme stays restrained;
// extra hues are reserved for future crypto, casino, and message-category UIs.
var (
	cyan = lipgloss.Color("#69C6D2")
	blue = lipgloss.Color("#7198D4")
	teal = lipgloss.Color("#65B9A6")
	violet = lipgloss.Color("#A88AD6")
	magenta = lipgloss.Color("#D27CC1")
	orange = lipgloss.Color("#D98B5F")
	pink = lipgloss.Color("#D9879A")
	lime = lipgloss.Color("#B7C96A")
	ink = lipgloss.Color("#D8D8D0")
	muted = lipgloss.Color("#777970")
	red = lipgloss.Color("#B84B45")
	amber = lipgloss.Color("#D6B36A")
	green = lipgloss.Color("#9BC5A0")
	lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#42453F"))
	titleStyle = lipgloss.NewStyle().Foreground(ink).Bold(true)
	subtitleStyle = lipgloss.NewStyle().Foreground(muted).Italic(true)
	labelStyle = lipgloss.NewStyle().Foreground(muted)
	selectedStyle = lipgloss.NewStyle().Foreground(amber).Bold(true)
	dangerStyle = lipgloss.NewStyle().Foreground(red)
	okStyle = lipgloss.NewStyle().Foreground(green).Bold(true)
)

func (a *App) menuView() string {
	var b strings.Builder
	b.WriteString("\n\n"+titleStyle.Render("DEEP STATE")+"\n")
	b.WriteString(lipgloss.NewStyle().Foreground(amber).Bold(true).Render("CLICKER")+"\n\n")
	b.WriteString(subtitleStyle.Render("Every dollar leaves a mark.")+"\n\n")
	options := []string{"NEW OPERATION", "LOAD OPERATION", "QUIT"}
	for i, option := range options {
		if i == a.menu { b.WriteString(selectedStyle.Render("  ▸ "+option)) } else { b.WriteString(labelStyle.Render("    "+option)) }
		b.WriteString("\n")
	}
	if a.message != "" { b.WriteString("\n"+dangerStyle.Render(a.message)+"\n") }
	b.WriteString("\n"+labelStyle.Render("↑/↓ navigate   ENTER select   N new   L load   Q quit"))
	return lipgloss.NewStyle().Padding(1,2).Render(b.String())
}

func (a *App) introView() string {
	width := max(24, min(a.width-6, 76))
	content := strings.Join([]string{
		"", titleStyle.Render("FEDERAL RESERVE  /  INTERNAL OPERATIONS"),
		lineStyle.Render(strings.Repeat("─", max(0,width-6))), "",
		"Your shift is ordinary. The queue is already running.",
		"Jobs spool, presses cycle, and the records reconcile.",
		"",
		"Then you find a discrepancy that should not be possible.",
		"The exploit is already set up. You have tested it, checked it,",
		"and left it waiting inside the rhythm of the next batch.",
		"",
		dangerStyle.Render("The machine keeps moving."),
		"All that remains is to decide when to interfere.",
		"", subtitleStyle.Render("The queue is live. Your first opportunity will come."),
		"", labelStyle.Render("Press ENTER or click to begin."),
	}, "\n")
	return lipgloss.NewStyle().Padding(2,3).Width(width).Render(content)
}

func (a *App) moveMainSelection(delta int) {
	if a.mainView == "upgrades" {
		a.selectedUpgrade = max(0, min(len(game.ModuleCatalog)-1, a.selectedUpgrade+delta))
		return
	}
	if a.mainView == "rack" {
		if a.rackFocusRight {
			installed := a.installedModules()
			a.selectedRackModule = max(0, min(max(0,len(installed)-1), a.selectedRackModule+delta))
			if a.selectedRackModule >= a.rackRightScroll+4 { a.rackRightScroll = a.selectedRackModule-3 }
			if a.selectedRackModule < a.rackRightScroll { a.rackRightScroll = a.selectedRackModule }
		} else {
			a.selectedUpgrade = max(0, min(len(game.ModuleCatalog)-1, a.selectedUpgrade+delta))
			if a.selectedUpgrade >= a.rackLeftScroll+5 { a.rackLeftScroll = a.selectedUpgrade-4 }
			if a.selectedUpgrade < a.rackLeftScroll { a.rackLeftScroll = a.selectedUpgrade }
		}
	}
}

func (a *App) installedModules() []game.ModuleState {
	var result []game.ModuleState
	for _, m := range a.state.Modules { if m.Installed { result = append(result,m) } }
	return result
}

func (a *App) activateMainSelection() {
	now := time.Now()
	if a.mainView == "upgrades" { a.buySelectedModule(now); return }
	if a.mainView == "rack" {
		if a.rackFocusRight {
			installed := a.installedModules()
			if a.selectedRackModule >= 0 && a.selectedRackModule < len(installed) {
				m := installed[a.selectedRackModule]
				var err error
				if m.Jammed { err = a.state.RemoveJammedLegacyModule(now) } else { err = a.state.RemoveModule(m.ID,now) }
				if err != nil { a.message = err.Error() } else { a.message = "Module removed from rack." }
			}
		} else {
			if a.selectedUpgrade >= 0 && a.selectedUpgrade < len(game.ModuleCatalog) {
				spec := game.ModuleCatalog[a.selectedUpgrade]
				if m := a.state.Module(spec.ID); m != nil && m.Installed {
					a.message = "Module already installed. Open UPGRADES to improve its revision."
				} else if err := a.state.BuyOrUpgradeModule(spec.ID,now); err != nil { a.message = err.Error() } else { a.message = spec.Name+" installed in the rack." }
			}
		}
	}
}

func (a *App) buySelectedModule(now time.Time) {
	if a.selectedUpgrade < 0 || a.selectedUpgrade >= len(game.ModuleCatalog) { return }
	spec := game.ModuleCatalog[a.selectedUpgrade]
	if spec.ID == "obfuscation" && a.state.Risk < 25 { a.message = "Signal obfuscation remains unavailable until scrutiny increases."; return }
	if err := a.state.BuyOrUpgradeModule(spec.ID,now); err != nil { a.message = err.Error() } else { a.message = spec.Name+" upgraded to revision "+fmt.Sprint(a.state.Module(spec.ID).Level)+"." }
}

func lWidthForRack(width int) int { return max(24,min(38,width/3)) }

func (a *App) gameView() string {
	l := a.layout()
	var b strings.Builder
	b.WriteString(a.newsStripView(max(12, l.Width-4))+"\n")
	b.WriteString(lineStyle.Render(strings.Repeat("─", max(20, l.Width-4)))+"\n")
	tabs := []game.Tab{game.TabPrint}
	if a.state.EmailUnlocked { tabs = append(tabs, game.TabMessages) }
	var labels []string
	unread := unreadCount(a.state.Messages)
	urgent := a.state.ITSuspicion >= 4 || (a.state.PendingReplyTicks > 0 && a.state.PendingReplyTicks <= 2)
	for _, tab := range tabs {
		label := "MAIN"
		if tab == game.TabMessages { label = "MESSAGES"; if unread > 0 { label += fmt.Sprintf(" !%d", unread) } }
		rendered := "["+label+"]"
		if tab == a.state.CurrentTab { labels = append(labels, selectedStyle.Render(rendered))
		} else if tab == game.TabMessages && unread > 0 && urgent {
			if a.uiFrame%5 < 3 { labels = append(labels, dangerStyle.Render(rendered)) } else { labels = append(labels, labelStyle.Render(rendered)) }
		} else if tab == game.TabMessages && unread > 0 && a.uiFrame%5 < 3 { labels = append(labels, selectedStyle.Render(rendered))
		} else { labels = append(labels, labelStyle.Render(rendered)) }
	}
	tabLine := strings.Join(labels, "   ")
	if lipgloss.Width(tabLine) > max(12, l.Width-8) {
		selectedLabel := "[MAIN]"
		if a.state.CurrentTab == game.TabMessages {
			selectedLabel = "[MESSAGES"
			if unread > 0 { selectedLabel += fmt.Sprintf(" !%d", unread) }
			selectedLabel += "]"
		}
		tabLine = selectedStyle.Render(selectedLabel) + " " + selectedStyle.Render(">")
	}
	b.WriteString(tabLine+"\n")
	b.WriteString(lineStyle.Render(strings.Repeat("─", max(20, l.Width-4)))+"\n")
	var panel string
	switch a.state.CurrentTab {
	case game.TabPrint, game.TabStore:
		switch a.mainView {
		case "upgrades": panel = a.upgradesPanel(l)
		case "rack": panel = a.rackPanel(l)
		default: panel = a.printPanel(l)
		}
	case game.TabMessages: panel = a.messagesPanel(l)
	}
	panelLines := max(4, l.TickerY-l.ContentY-2)
	if a.message != "" { panelLines-- }
	panel = fitLines(panel, panelLines)
	b.WriteString(panel+"\n")
	if a.message != "" { b.WriteString(subtitleStyle.Render(truncate(a.message, max(12, l.Width-4)))+"\n") }
	for strings.Count(b.String(), "\n") < l.TickerY-1 { b.WriteString("\n") }
	b.WriteString(labelStyle.Render("X/P inject  1–3 upgrades/replies  M messages  I ignore  V save  H help  Q quit"))
	return lipgloss.NewStyle().Padding(1,2).Render(b.String())
}


func (a *App) cashPanelView(width int) string {
	boxWidth := max(28,min(width-4,68))
	inner := boxWidth-2
	title := " CAPITAL "
	top := "┌─"+title+strings.Repeat("─",max(0,inner-len(title)-1))+"┐"
	cashLine := " CAPITAL ON HAND  $"+money(a.state.CashOnHand)
	capacityLine := " CAPACITY         $"+money(a.state.CashCapacity)
	netLine := " EST. NET WORTH   ~$"+money(a.state.EstimatedNetWorth)
	totalLine := " TOTAL PRINTED    $"+money(a.state.TotalPrinted)
	util := 0
	if a.state.CashCapacity > 0 { util = int(a.state.CashOnHand*100/a.state.CashCapacity) }
	barWidth := max(10,inner-14)
	filled := min(barWidth,max(0,util*barWidth/100))
	bar := strings.Repeat("━",filled)+strings.Repeat("─",barWidth-filled)
	barColor := green
	if util >= 80 { barColor = amber }
	if util >= 100 {
		r := []rune(bar)
		if len(r)>0 { r[len(r)-1] = '!' }
		bar = string(r)
	}
	status := fmt.Sprintf(" %3d%% UTILIZED",util)
	if util >= 100 { status = fmt.Sprintf(" %3d%% OVERFLOW",util) }
	lines := []string{
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(amber).Bold(true).Render(truncate(cashLine,inner))+strings.Repeat(" ",max(0,inner-len([]rune(truncate(cashLine,inner)))) )+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(ink).Render(truncate(capacityLine,inner))+strings.Repeat(" ",max(0,inner-len([]rune(truncate(capacityLine,inner)))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(truncate(netLine,inner))+strings.Repeat(" ",max(0,inner-len([]rune(truncate(netLine,inner)))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(ink).Render(truncate(totalLine,inner))+strings.Repeat(" ",max(0,inner-len([]rune(truncate(totalLine,inner)))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(barColor).Render(truncate(bar,inner))+strings.Repeat(" ",max(0,inner-len([]rune(truncate(bar,inner)))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(barColor).Render(truncate(status,inner))+strings.Repeat(" ",max(0,inner-len([]rune(truncate(status,inner)))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
	}
	return lipgloss.NewStyle().Width(boxWidth).Render(strings.Join(append([]string{top},append(lines,"└"+strings.Repeat("─",inner)+"┘")...), "\n"))
}

func (a *App) printPanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MAIN  /  PRINT OPERATIONS")+"   ")
	if a.state.Status == game.RunFederal || a.state.Status == game.RunSandbox { b.WriteString(okStyle.Render("SYSTEM READY")) } else { b.WriteString(dangerStyle.Render("SYSTEM OFFLINE")) }
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("RUN: %-8s  UPTIME: %04ds  EXPOSURE: %02d/100", strings.ToUpper(string(a.state.Status)), a.state.Tick, a.state.Risk))+"\n")
	b.WriteString(a.cashPanelView(l.Width)+"\n\n")
	b.WriteString(titleStyle.Render("PRINT WINDOW")+"   "+labelStyle.Render(fmt.Sprintf("BATCH %05d", a.state.Batch.Number))+"\n")
	if a.state.Batch.WindowOpen { b.WriteString(okStyle.Render("● OPPORTUNITY OPEN")+"   "+labelStyle.Render(fmt.Sprintf("QUEUED: %d", a.state.Batch.QueuedDollars))+"\n")
	} else { b.WriteString(labelStyle.Render("● ROUTINE CYCLE")+"   "+labelStyle.Render(fmt.Sprintf("QUEUED: %d", a.state.Batch.QueuedDollars))+"\n") }
	width := max(10, min(l.Width-12, 54))
	progress := a.state.Batch.Progress*width/game.BatchCycleTicks
	if progress < 0 { progress = 0 }; if progress > width { progress = width }
	bar := strings.Repeat("━", progress)+strings.Repeat("─", width-progress)
	marker := "▶"; if a.state.Batch.WindowOpen { marker = "◆" }
	if progress < width { runes := []rune(bar); runes[progress] = []rune(marker)[0]; bar = string(runes) }
	b.WriteString("  "+lineStyle.Render(bar)+"\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("  SPOOLING  %02d%%", a.state.Batch.Progress*100/game.BatchCycleTicks))+"\n")
	injectStyle := labelStyle
	if a.state.Batch.WindowOpen { injectStyle = okStyle }
	if a.injectHeldRed || time.Now().Before(a.injectFlashUntil) { injectStyle = dangerStyle.Bold(true) }
	b.WriteString("  "+injectStyle.Render("[X]  INJECT")+"\n")
	outcome := strings.ToLower(a.state.Batch.LastOutcome)
	if a.state.Batch.LastOutcome != "" && !strings.Contains(outcome, "queue moved on") && !strings.Contains(outcome, "no instruction was sent") {
		b.WriteString("  "+labelStyle.Render(truncate(a.state.Batch.LastOutcome, max(12, l.Width-8)))+"\n")
	}
	b.WriteString("\n"+selectedStyle.Render("[U] UPGRADES")+"   "+selectedStyle.Render("[R] HARDWARE RACK")+"\n")
	b.WriteString(labelStyle.Render("The machine is old. The unused bays still accept modules."))
	return b.String()
}

func (a *App) upgradesPanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MAIN  /  UPGRADES")+"\n")
	b.WriteString(labelStyle.Render("MODULE DEVELOPMENT  ·  SELECT AN ITEM, REVIEW ITS REVISION, THEN BUY")+"\n\n")
	leftWidth := max(22,min(34,l.Width/3))
	var left,right strings.Builder
	left.WriteString(titleStyle.Render("MODULE CATALOGUE")+"\n")
	for i,spec := range game.ModuleCatalog {
		status := "AVAILABLE"
		m := a.state.Module(spec.ID)
		if m != nil && m.Installed { status = fmt.Sprintf("REV %d",m.Level) }
		if spec.ID=="obfuscation" && a.state.Risk<25 { status="LOCKED" }
		line := truncate(fmt.Sprintf("%s %s",map[bool]string{true:"›",false:" "}[i==a.selectedUpgrade],spec.Name),leftWidth-2)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(spec.Color)).Background(lipgloss.Color("#101010"))
		if i==a.selectedUpgrade { style = style.Bold(true).Underline(true) }
		left.WriteString(style.Render(line)+"\n")
		left.WriteString(labelStyle.Render("  "+status)+"\n")
	}
	spec := game.ModuleCatalog[max(0,min(len(game.ModuleCatalog)-1,a.selectedUpgrade))]
	m := a.state.Module(spec.ID)
	level := 0
	if m!=nil { level=m.Level }
	right.WriteString(titleStyle.Render(spec.Name)+"\n\n")
	right.WriteString(wrapText(spec.Description,max(20,l.Width-leftWidth-10))+"\n\n")
	right.WriteString(labelStyle.Render(fmt.Sprintf("CURRENT REVISION: %d / %d",level,spec.MaxLevel))+"\n")
	right.WriteString(labelStyle.Render(fmt.Sprintf("SUSPICION PROFILE: %d / 5",spec.Suspicion))+"\n")
	if spec.ID=="obfuscation" && a.state.Risk<25 {
		right.WriteString(dangerStyle.Render("REQUIREMENT: operation scrutiny 25+")+"\n")
	} else if level>=spec.MaxLevel {
		right.WriteString(okStyle.Render("MAXIMUM REVISION INSTALLED")+"\n")
	} else {
		cost:=spec.Cost*int64(level+1)
		right.WriteString(labelStyle.Render("NEXT REVISION COST: $"+money(cost))+"\n\n")
		right.WriteString(selectedStyle.Render("[ENTER / B] BUY / UPGRADE MODULE")+"\n")
	}
	right.WriteString("\n"+labelStyle.Render("[R] HARDWARE RACK   [ESC] RETURN TO OPERATIONS"))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,lipgloss.NewStyle().Width(leftWidth).PaddingRight(1).Render(left.String()),lipgloss.NewStyle().Width(max(20,l.Width-leftWidth-5)).PaddingLeft(1).Render(right.String())))
	return b.String()
}

func (a *App) rackPanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MAIN  /  HARDWARE RACK")+"\n")
	installed := a.installedModules()
	b.WriteString(labelStyle.Render(fmt.Sprintf("BAYS IN USE: %d  ·  RACK CAPACITY: UNLIMITED",len(installed)))+"\n")
	leftWidth := max(24,min(38,l.Width/3))
	visibleLeft := 5
	visibleRight := max(2,min(4,(l.Height-16)/6))
	var left,right strings.Builder
	left.WriteString(titleStyle.Render("AVAILABLE MODULES")+"\n")
	start := a.rackLeftScroll
	end := min(len(game.ModuleCatalog),start+visibleLeft)
	if start>0 { left.WriteString(labelStyle.Render("  ▲ MORE ABOVE")+"\n") }
	for i:=start;i<end;i++ {
		spec:=game.ModuleCatalog[i]
		m:=a.state.Module(spec.ID)
		status:="NEW"
		if m!=nil && m.Installed { status=fmt.Sprintf("REV %d INSTALLED",m.Level) }
		if spec.ID=="obfuscation" && a.state.Risk<25 { status="LOCKED" }
		name:=truncate(fmt.Sprintf("%s %s",map[bool]string{true:"›",false:" "}[i==a.selectedUpgrade],spec.Name),leftWidth-2)
		style:=lipgloss.NewStyle().Foreground(lipgloss.Color("#080808")).Background(lipgloss.Color(spec.Color)).Padding(0,1)
		if i!=a.selectedUpgrade { style=style.Faint(true) }
		left.WriteString(style.Render(name)+"\n")
		left.WriteString(labelStyle.Render("  "+status)+"\n")
	}
	if end<len(game.ModuleCatalog) { left.WriteString(labelStyle.Render("  ▼ MORE BELOW")+"\n") }
	left.WriteString("\n"+labelStyle.Render("←/→ FOCUS · ↑/↓ SCROLL")+"\n")
	right.WriteString(titleStyle.Render("INSTALLED RACK")+"\n")
	if len(installed)==0 { right.WriteString(labelStyle.Render("No modules installed.")+"\n") }
	startR:=a.rackRightScroll
	endR:=min(len(installed),startR+visibleRight)
	if startR>0 { right.WriteString(labelStyle.Render("  ▲ MORE ABOVE")+"\n") }
	for i:=startR;i<endR;i++ {
		m:=installed[i]
		name:=m.ID
		color:=teal
		for _,spec:=range game.ModuleCatalog { if spec.ID==m.ID { name=spec.Name; color=lipgloss.Color(spec.Color); break } }
		if m.ID=="legacy_controller" { name="LEGACY PRINT CONTROLLER"; color=orange }
		stateLabel:=fmt.Sprintf("REVISION %02d",m.Level)
		if m.Jammed { stateLabel="JAMMED · SPECIALIST REMOVAL" }
		blockWidth:=max(18,min(l.Width-leftWidth-8,38))
		block:=[]string{"┌"+strings.Repeat("─",blockWidth-2)+"┐","│ "+truncate(name,blockWidth-4)+strings.Repeat(" ",max(0,blockWidth-4-lipgloss.Width(truncate(name,blockWidth-4))))+" │","│ "+truncate(stateLabel,blockWidth-4)+strings.Repeat(" ",max(0,blockWidth-4-lipgloss.Width(truncate(stateLabel,blockWidth-4))))+" │"}
		flicker:="*  *  *  *  *  *"
		if a.uiFrame%7<3 { flicker="* *   *  * *  *" }
		block=append(block,"│ "+truncate(flicker,blockWidth-4)+strings.Repeat(" ",max(0,blockWidth-4-lipgloss.Width(truncate(flicker,blockWidth-4))))+" │","└"+strings.Repeat("─",blockWidth-2)+"┘")
		st:=lipgloss.NewStyle().Foreground(color)
		if i==a.selectedRackModule && a.rackFocusRight { st=st.Bold(true) }
		right.WriteString(st.Render(strings.Join(block,"\n"))+"\n")
		if m.Jammed { right.WriteString(dangerStyle.Render("  Removal estimate: $50.00")+"\n") }
	}
	if endR<len(installed) { right.WriteString(labelStyle.Render("  ▼ MORE BELOW")+"\n") }
	right.WriteString("\n"+labelStyle.Render("ENTER: acquire/remove · [U] UPGRADES · [B] BACK")+"\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,lipgloss.NewStyle().Width(leftWidth).PaddingRight(1).Render(left.String()),lipgloss.NewStyle().Width(max(22,l.Width-leftWidth-5)).PaddingLeft(1).Render(right.String())))
	return b.String()
}

func (a *App) storePanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("STORAGE & MACHINE UPGRADES")+"\n\n")
	if a.state.Upgrades.ExpandedCashCapacity { b.WriteString(okStyle.Render("[1] LOCKABLE CASH BOX — INSTALLED")+"\n") } else {
		b.WriteString(fmt.Sprintf("  %s Lockable cash box — $%s; capacity to $2,500",selectedStyle.Render("[1]"),money(game.CapacityUpgradeCost))+"\n")
	}
	b.WriteString("\n")
	if a.state.Upgrades.WindowCalibration { b.WriteString(okStyle.Render("[2] QUEUE INDICATOR CALIBRATION — INSTALLED")+"\n") } else {
		b.WriteString("  "+selectedStyle.Render("[2]")+" Queue indicator calibration — $15.00; wider window\n")
	}
	b.WriteString("\n")
	if a.state.Upgrades.PrintAutomation { b.WriteString(okStyle.Render("[3] QUIET EXPLOIT AUTOMATION — INSTALLED")+"\n") } else {
		b.WriteString(fmt.Sprintf("  %s Quiet exploit automation — $%s; queues automatically\n",selectedStyle.Render("[3]"),money(game.AutomationCost)))
	}
	b.WriteString(labelStyle.Render("  Purchases require cash on hand."))
	return b.String()
}

func (a *App) messagesPanel(l uiLayout) string {
	if len(a.state.Messages) == 0 {
		return titleStyle.Render("MESSAGES")+"   "+labelStyle.Render("INTERNAL MAIL")+"\n\n"+labelStyle.Render("No messages.")
	}
	selected := a.state.SelectedMessage()
	if selected == nil { return titleStyle.Render("MESSAGES")+"\n" }
	if l.Height < 30 {
		if l.SideBySide {
			leftWidth:=max(28,min(36,l.Width/3))
			rightWidth:=max(40,l.Width-leftWidth-8)
			var left,right strings.Builder
			left.WriteString(titleStyle.Render("INBOX")+"\n")
			visible:=min(len(a.state.Messages),4)
			for i:=0;i<visible;i++ {
				m:=a.state.Messages[i]; cursor:=" "; if m.ID==a.state.SelectedMessageID { cursor="›" }; status:=" "; if !m.Read { status="●" }
				line:=truncate(fmt.Sprintf("%s%s %s",cursor,status,m.Subject),leftWidth-2)
				if m.ID==a.state.SelectedMessageID { left.WriteString(selectedStyle.Render(line)) } else { left.WriteString(labelStyle.Render(line)) }
				left.WriteString("\n")
			}
			right.WriteString(titleStyle.Render(truncate(selected.Subject,rightWidth-2))+"\n")
			right.WriteString(labelStyle.Render("FROM: "+truncate(selected.Sender,rightWidth-8))+"\n")
			compactBody:=strings.Join(strings.Fields(selected.Body)," ")
			bodyLines:=strings.Split(wrapText(compactBody,rightWidth-4),"\n")
			for i:=0;i<min(2,len(bodyLines));i++ { right.WriteString(truncate(bodyLines[i],rightWidth-3)+"\n") }
			if len(selected.Choices)>0 && a.state.PendingReplyTicks==0 {
				right.WriteString(selectedStyle.Render("[1] "+truncate(selected.Choices[0],rightWidth-7))+"\n")
				if len(selected.Choices)>1 { right.WriteString(selectedStyle.Render("[2] "+truncate(selected.Choices[1],rightWidth-7))+"\n") }
				if len(selected.Choices)>2 { right.WriteString(selectedStyle.Render("[3] "+truncate(selected.Choices[2],rightWidth-7))+"\n") }
			} else { right.WriteString(labelStyle.Render("Reply pending...")+"\n") }
			return lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Width(leftWidth).PaddingRight(1).Render(left.String()),
				lipgloss.NewStyle().Width(rightWidth).PaddingLeft(1).Render(right.String()))
		}
		var b strings.Builder
		b.WriteString(titleStyle.Render("INBOX")+"\n")
		b.WriteString(selectedStyle.Render("› "+truncate(selected.Subject,max(16,l.Width-10)))+"\n")
		b.WriteString(labelStyle.Render("FROM: "+truncate(selected.Sender,max(16,l.Width-12)))+"\n")
		compactBody:=strings.Join(strings.Fields(selected.Body)," ")
		bodyLines:=strings.Split(wrapText(compactBody,max(24,l.Width-8)),"\n")
		for i:=0;i<min(2,len(bodyLines));i++ { b.WriteString(truncate(bodyLines[i],max(12,l.Width-6))+"\n") }
		if len(selected.Choices)>0 && a.state.PendingReplyTicks==0 {
			b.WriteString(selectedStyle.Render("[1] "+truncate(selected.Choices[0],max(12,l.Width-10)))+"\n")
			if len(selected.Choices)>1 { b.WriteString(selectedStyle.Render("[2] "+truncate(selected.Choices[1],max(12,l.Width-10)))+"\n") }
			if len(selected.Choices)>2 { b.WriteString(selectedStyle.Render("[3] "+truncate(selected.Choices[2],max(12,l.Width-10)))+"\n") }
		} else { b.WriteString(labelStyle.Render("Reply pending...")+"\n") }
		return b.String()
	}
	if l.SideBySide {
		leftWidth:=max(28,min(36,l.Width/3)); rightWidth:=max(40,l.Width-leftWidth-8)
		var left,right strings.Builder
		left.WriteString(titleStyle.Render("INBOX")+"\n")
		visible:=min(len(a.state.Messages),5)
		for i:=0;i<visible;i++ {
			m:=a.state.Messages[i]; cursor:=" "; if m.ID==a.state.SelectedMessageID { cursor="›" }; status:=" "; if !m.Read { status="●" }
			line:=truncate(fmt.Sprintf("%s%s %s",cursor,status,m.Subject),leftWidth-2)
			if m.ID==a.state.SelectedMessageID { left.WriteString(selectedStyle.Render(line)) } else { left.WriteString(labelStyle.Render(line)) }
			left.WriteString("\n")
		}
		right.WriteString(titleStyle.Render("READING PANE")+"\n")
		right.WriteString(titleStyle.Render(selected.Subject)+"\n")
		right.WriteString(labelStyle.Render("FROM: "+selected.Sender+" <"+selected.Address+">")+"\n")
		right.WriteString(labelStyle.Render("TO: "+selected.To+"   RECEIVED: T+"+fmt.Sprintf("%ds",selected.AtTick))+"\n\n")
		right.WriteString(wrapText(selected.Body,rightWidth-4)+"\n")
		if len(selected.Choices)>0 && a.state.PendingReplyTicks==0 {
			right.WriteString("\n"+selectedStyle.Render("[1] ")+selected.Choices[0]+"\n")
			if len(selected.Choices)>1 { right.WriteString(selectedStyle.Render("[2] ")+selected.Choices[1]+"\n") }
			if len(selected.Choices)>2 { right.WriteString(selectedStyle.Render("[3] ")+selected.Choices[2]+"\n") }
			right.WriteString(labelStyle.Render("[I] Leave unanswered")+"\n")
		} else if a.state.PendingReplyTicks>0 { right.WriteString("\n"+labelStyle.Render("Reply sent. Waiting for a response...")+"\n") }
		return lipgloss.JoinHorizontal(lipgloss.Top,lipgloss.NewStyle().Width(leftWidth).PaddingRight(1).Render(left.String()),lipgloss.NewStyle().Width(rightWidth).PaddingLeft(1).Render(right.String()))
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("MESSAGES")+"   "+labelStyle.Render("INTERNAL MAIL")+"\n")
	visible:=min(len(a.state.Messages),5)
	for i:=0;i<visible;i++ {
		m:=a.state.Messages[i]; cursor:=" "; if m.ID==a.state.SelectedMessageID { cursor="›" }; status:=" "; if !m.Read { status="●" }
		line:=truncate(fmt.Sprintf("%s%s %-18s %s",cursor,status,m.Sender,m.Subject),max(20,l.Width-6))
		if m.ID==a.state.SelectedMessageID { b.WriteString(selectedStyle.Render(line)) } else { b.WriteString(labelStyle.Render(line)) }
		b.WriteString("\n")
	}
	b.WriteString(lineStyle.Render(strings.Repeat("─",max(20,l.Width-6)))+"\n")
	b.WriteString(titleStyle.Render(selected.Subject)+"\n")
	b.WriteString(labelStyle.Render("FROM: "+selected.Sender+" <"+selected.Address+">")+"\n")
	b.WriteString(labelStyle.Render("TO: "+selected.To+"    RECEIVED: T+"+fmt.Sprintf("%ds",selected.AtTick))+"\n\n")
	b.WriteString(wrapText(selected.Body,max(24,l.Width-8))+"\n")
	if len(selected.Choices)>0 && a.state.PendingReplyTicks==0 {
		b.WriteString("\n"+selectedStyle.Render("[1] ")+selected.Choices[0]+"\n")
		if len(selected.Choices)>1 { b.WriteString(selectedStyle.Render("[2] ")+selected.Choices[1]+"\n") }
		if len(selected.Choices)>2 { b.WriteString(selectedStyle.Render("[3] ")+selected.Choices[2]+"\n") }
		b.WriteString(labelStyle.Render("[I] Leave unanswered")+"\n")
	} else if a.state.PendingReplyTicks>0 { b.WriteString("\n"+labelStyle.Render("Reply sent. Waiting for a response...")+"\n") }
	return b.String()
}

func (a *App) startNewsDispatch() {
	if a.state.NewsIndex < 0 || a.state.NewsIndex >= len(game.NewsHeadlines) { return }
	a.newsHeadline = game.NewsHeadlines[a.state.NewsIndex]
	a.newsStartedAt = time.Now()
}

func (a *App) newsStripView(width int) string {
	if a.newsHeadline == "" {
		return lipgloss.NewStyle().Foreground(muted).Width(width).Align(lipgloss.Center).Render("--- NEWS BULLETIN ---")
	}
	now := time.Now()
	elapsed := now.Sub(a.newsStartedAt)
	const flashDuration = 2 * time.Second
	if elapsed < flashDuration {
		color := amber
		headline := strings.ToLower(a.newsHeadline)
		switch {
		case strings.Contains(headline, "emergency") || strings.Contains(headline, "erodes") || strings.Contains(headline, "families") || strings.Contains(headline, "shortages") || strings.Contains(headline, "extraordinary"):
			color = red
		case strings.Contains(headline, "approves") || strings.Contains(headline, "funding") || strings.Contains(headline, "eases") || strings.Contains(headline, "improves"):
			color = green
		}
		if a.uiFrame%4 < 2 { return lipgloss.NewStyle().Foreground(color).Bold(true).Width(width).Align(lipgloss.Center).Render("--- NEWS BULLETIN ---") }
		return lipgloss.NewStyle().Foreground(muted).Width(width).Align(lipgloss.Center).Render("--- NEWS BULLETIN ---")
	}
	runes := []rune(a.newsHeadline)
	offset := int((elapsed-flashDuration)/(100*time.Millisecond))
	// Start with a full screen of leading spaces; the headline enters from the
	// right edge, crosses the entire strip, then clears before the idle marker returns.
	totalTravel := width + len(runes)
	if offset >= totalTravel {
		a.newsHeadline = ""
		return lipgloss.NewStyle().Foreground(muted).Width(width).Align(lipgloss.Center).Render("--- NEWS BULLETIN ---")
	}
	track := strings.Repeat(" ", width) + string(runes) + strings.Repeat(" ", width)
	start := max(0, min(len([]rune(track)), offset))
	visibleRunes := []rune(track)[start:]
	if len(visibleRunes) > width { visibleRunes = visibleRunes[:width] }
	return lipgloss.NewStyle().Foreground(ink).Width(width).Render(string(visibleRunes))
}

func (a *App) mouseOnTabRow(y int) bool {
	lines := plainViewLines(a.View())
	return y >= 0 && y < len(lines) && (strings.Contains(lines[y], "[MAIN]") || strings.Contains(lines[y], "[MESSAGES"))
}

func (a *App) isInjectAt(x, y int) bool {
	if a.screen != screenGame || a.state.CurrentTab != game.TabPrint { return false }
	lines := plainViewLines(a.View())
	if y < 0 || y >= len(lines) { return false }
	return strings.Contains(lines[y], "INJECT")
}

func (a *App) cycleTab(delta int) {
	tabs := []game.Tab{game.TabPrint}
	if a.state.EmailUnlocked { tabs = append(tabs, game.TabMessages) }
	current := 0
	for i, tab := range tabs { if tab == a.state.CurrentTab { current = i; break } }
	current = (current+delta+len(tabs))%len(tabs)
	_ = a.state.OpenTab(tabs[current])
	a.message = ""
}

func fitLines(s string, maxLines int) string {
	lines:=strings.Split(s,"\n")
	if len(lines)<=maxLines { return s }
	if maxLines<=0 { return "" }
	lines=lines[:maxLines]
	if len(lines)>0 { lines[len(lines)-1]=truncate(lines[len(lines)-1],max(8,len([]rune(lines[len(lines)-1])))) }
	return strings.Join(lines,"\n")
}
func unreadCount(messages []game.Message) int { n:=0; for _,m:=range messages { if !m.Read { n++ } }; return n }
func tickerText(s game.State,width int, frame int64) string {
	if len(game.NewsHeadlines)==0 { return "" }
	headline:=game.NewsHeadlines[min(s.NewsIndex,len(game.NewsHeadlines)-1)]
	source:="  WORLD BULLETIN  /  "
	full:=source+headline+"     •     "+headline
	if len(full)<=width { return full }
	runes:=[]rune(full)
	offset:=int(frame/2)%max(1,len(runes))
	loop:=string(append(append([]rune{},runes...),append([]rune("     "),runes...)...))
	return truncate(string([]rune(loop)[offset:]),width)
}
func truncate(s string,width int) string {
	if width<1 { return "" }
	r:=[]rune(s); if len(r)<=width { return s }
	if width<=1 { return string(r[:width]) }
	return string(r[:width-1])+"…"
}
func wrapText(s string,width int) string {
	if width<8 { width=8 }
	var out []string
	for _,para:=range strings.Split(s,"\n") {
		words:=strings.Fields(para)
		if len(words)==0 { out=append(out,""); continue }
		line:=""
		for _,word:=range words {
			if line=="" { line=word } else if len([]rune(line))+1+len([]rune(word))<=width { line+=" "+word } else { out=append(out,line); line=word }
		}
		out=append(out,line)
	}
	return strings.Join(out,"\n")
}
func money(cents int64) string { return fmt.Sprintf("%d.%02d",cents/100,cents%100) }
