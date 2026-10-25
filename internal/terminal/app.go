package terminal

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

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
	diskFrame int64
	newsHeadline string
	newsStartedAt time.Time
	injectFlashUntil time.Time
	injectMouseHeld bool
	injectHeldRed bool
	mainView string
	selectedUpgrade int
	selectedRackModule int
	rackFocusRight bool
	rackCatalogSelected bool
	rackRemovalArmedModule string
	upgradeBuyArmed bool
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
		if a.screen == screenGame && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
			delta := 1
			if msg.Button == tea.MouseButtonWheelUp { delta = -1 }
			if a.mouseOnTabRow(msg.Y) { a.cycleTab(delta) } else if a.state.CurrentTab == game.TabPrint && a.mainView == "rack" {
				leftWidth, _ := paneWidths(a.layout())
				paneHeight := max(4, panelLineLimit(a.layout(), a.message != "")-2)
				if msg.X < 2+leftWidth {
					visibleLeft := max(1, paneHeight-2)
					a.rackLeftScroll = max(0, min(max(0, len(game.ModuleCatalog)-visibleLeft), a.rackLeftScroll+delta))
				} else {
					visible := visibleRackBays(paneHeight)
					a.rackRightScroll = max(0, min(max(0, len(a.rackBays())-visible), a.rackRightScroll+delta))
				}
			} else if a.state.CurrentTab == game.TabPrint && a.mainView == "upgrades" { a.selectedUpgrade=max(0,min(len(game.ModuleCatalog)-1,a.selectedUpgrade+delta)) }
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
			a.diskFrame++
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
	// Keep the interface away from the terminal's bottom row and rightmost
	// column. Some terminals wrap or scroll when a frame touches either edge.
	w, h := a.width-1, a.height-1
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
		if strings.Contains(line, "[MAIN]") || strings.Contains(line, "[STORAGE]") || strings.Contains(line, "[MESSAGES") {
			arrowAt := strings.LastIndex(line, ">")
			if arrowAt >= 0 && x >= arrowAt {
				a.cycleTab(1)
				return nil
			}
			messagesAt := strings.Index(line, "MESSAGES")
			storageAt := strings.Index(line, "STORAGE")
			if messagesAt >= 0 && x >= messagesAt {
				_ = a.state.OpenTab(game.TabMessages)
				if m := a.state.SelectedMessage(); m != nil { _ = a.state.SelectMessage(m.ID, time.Now()) }
			} else if storageAt >= 0 && x >= storageAt {
				_ = a.state.OpenTab(game.TabStorage)
			} else { _ = a.state.OpenTab(game.TabPrint); a.mainView = "print" }
			a.message = ""
			return nil
		}

		switch a.state.CurrentTab {
		case game.TabPrint, game.TabStore:
			if strings.Contains(line, "INJECT") && a.mainView != "rack" && a.mainView != "upgrades" { return a.handleGameKey("x") }
			if a.mainView == "upgrades" {
				leftWidth, _ := paneWidths(a.layout())
				if x >= 2 && x < 2+leftWidth {
					for row, paneTitle := range lines {
						if strings.Contains(paneTitle, "MODULE CATALOG") {
							idx := y-row-1
							a.upgradeBuyArmed = false
							if idx >= 0 && idx < len(game.ModuleCatalog) {
								a.selectedUpgrade = idx
							}
							return nil
						}
					}
				} else if mouseWithin(line, x, "[ENTER / B] BUY / UPGRADE") || mouseWithin(line, x, "[CONFIRM] CLICK BUY / UPGRADE AGAIN") {
					return a.armOrConfirmUpgradeBuy()
				}
			}
			if a.mainView == "rack" {
				leftWidth, _ := paneWidths(a.layout())
				leftPaneHit := x >= 2 && x < 2+leftWidth
				if leftPaneHit {
					paneHeight := max(4, panelLineLimit(a.layout(), a.message != "")-2)
					visibleLeft := max(1, paneHeight-2)
					start := min(a.rackLeftScroll, max(0, len(game.ModuleCatalog)-visibleLeft))
					for row, paneTitle := range lines {
						if strings.Contains(paneTitle, "MODULES") && strings.Contains(paneTitle, "┌") {
							idx := start+y-row-1
							if idx >= start && idx < min(len(game.ModuleCatalog), start+visibleLeft) {
								if x == leftWidth {
									a.rackRemovalArmedModule = ""
									a.selectedUpgrade = idx
									a.rackCatalogSelected = true
									a.rackFocusRight = false
									spec := game.ModuleCatalog[idx]
									m := a.state.Module(spec.ID)
									if m != nil && m.Level > 0 && m.Installed {
										if err := a.state.RemoveModule(spec.ID, time.Now()); err != nil {
											a.message = err.Error()
										} else {
											a.message = spec.Name+" removed from the rack."
										}
									} else if m != nil && m.Level > 0 && !m.Installed {
										if err := a.state.InstallModule(spec.ID, time.Now()); err != nil {
											a.message = err.Error()
										} else {
											a.message = spec.Name+" inserted. Initialization sequence started."
											a.rackFocusRight = true
											a.selectRackModule(spec.ID)
										}
									} else if idx >= 0 {
										a.message = "Purchase "+spec.Name+" in UPGRADES before installing it."
									}
								} else if a.rackCatalogSelected && a.selectedUpgrade == idx {
									a.toggleSelectedRackCatalogModule(time.Now())
								} else {
									a.selectedUpgrade = idx
									a.rackCatalogSelected = true
									a.rackFocusRight = false
									a.rackRemovalArmedModule = ""
									a.message = "Selected "+game.ModuleCatalog[idx].Name+". Click again to install/remove."
								}
							}
							return nil
						}
					}
					return nil
				}

				rightPaneStart := 3+leftWidth
				if x >= rightPaneStart {
					startR := min(a.rackRightScroll, max(0, len(a.installedModules())-max(1,(max(4,panelLineLimit(a.layout(),a.message!="")-2)-9)/5)))
					installed := a.installedModules()
					visibleRight := max(1, (max(4,panelLineLimit(a.layout(),a.message!="")-2)-9)/5)
					endR := min(len(installed), startR+visibleRight)
					for row, paneTitle := range lines {
						if strings.Contains(paneTitle, "INSTALLED RACK") {
							cardY := row+1
							for i := startR; i < endR; i++ {
								mod := installed[i]
								cardHeight := rackCardHeight(mod)
								if y >= cardY && y < cardY+cardHeight {
									a.selectedRackModule = i
									a.rackFocusRight = true
									if mod.Jammed && mouseWithin(line, x, "[REMOVE]") {
										if a.rackRemovalArmedModule == mod.ID {
											if err := a.state.RemoveJammedLegacyModule(time.Now()); err != nil {
												a.message = err.Error()
											} else {
												a.message = "Specialist removed the legacy print controller."
											}
											a.rackRemovalArmedModule = ""
										} else {
											a.rackRemovalArmedModule = mod.ID
											a.message = "Click [REMOVE] again to confirm specialist removal ($50.00)."
										}
									} else if !mod.Jammed && mouseWithin(line, x, "[X]") {
										if err := a.state.RemoveModule(mod.ID, time.Now()); err != nil {
											a.message = err.Error()
										} else {
											a.message = mod.ID+" removed from the rack."
										}
										a.rackRemovalArmedModule = ""
									} else {
										a.rackRemovalArmedModule = ""
									}
									return nil
								}
								cardY += cardHeight+1
							}
							return nil
						}
					}
				}
			}
			switch {
			// MAIN places both controls on the same row. Check the actual
			// label under the pointer so the R hitbox cannot be swallowed by U.
			case mouseWithin(line, x, "[R] HARDWARE RACK") && a.hasPurchasedModule():
				a.mainView = "rack"
				a.rackFocusRight = false
				a.rackCatalogSelected = false
				a.rackRemovalArmedModule = ""
				a.message = ""
			case mouseWithin(line, x, "[U] UPGRADES"):
				a.mainView = "upgrades"
				a.selectedUpgrade = 0
				a.upgradeBuyArmed = false
				a.message = ""
			case strings.Contains(line, "[ENTER] ACQUIRE / REMOVE SELECTED") && mouseWithin(line, x, "[ENTER] ACQUIRE / REMOVE SELECTED"):
				a.activateMainSelection()
			case mouseWithin(line, x, "[B] BACK TO OPERATIONS"):
				a.mainView = "print"
				a.message = ""
			case a.mainView == "upgrades" && mouseWithin(line, x, "[ENTER / B] BUY / UPGRADE"):
				a.activateMainSelection()
			case a.mainView == "rack" && (strings.Contains(line, "MODULES") || strings.Contains(line, "INSTALLED RACK")):
				a.rackFocusRight = strings.Contains(line, "INSTALLED RACK") || x > lWidthForRack(a.width)
			}
		case game.TabStorage:
			if strings.Contains(line,"BUY LOCKBOX") { return a.handleGameKey("1") }
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
func (a *App) armOrConfirmUpgradeBuy() tea.Cmd {
	if a.upgradeBuyArmed {
		a.upgradeBuyArmed = false
		return a.handleGameKey("b")
	}
	a.upgradeBuyArmed = true
	a.message = "Click BUY / UPGRADE again to confirm. Press B or ENTER to buy immediately."
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
	key = strings.ToLower(key)
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
		if a.state.CurrentTab == game.TabStorage { if err := a.state.BuyLockbox(now); err != nil { a.message = err.Error() } else { a.message = "Lockbox added. Apartment capacity increased." } } else if a.state.CurrentTab == game.TabPrint && a.mainView == "upgrades" { a.buySelectedModule(now) } else if a.state.CurrentTab == game.TabStore || a.state.CurrentTab == game.TabPrint {
			a.mainView = "upgrades"; a.selectedUpgrade = 0
		} else if a.state.CurrentTab == game.TabMessages {
			if err := a.state.ReplyToSelected(0, now); err != nil { a.message = err.Error() } else { a.message = "Reply sent. The inbox is quiet for now." }
		}
	case "2":
		if a.state.CurrentTab == game.TabPrint && a.mainView == "upgrades" {
			a.buySelectedModule(now)
		} else if (a.state.CurrentTab == game.TabStore || a.state.CurrentTab == game.TabPrint) && a.hasPurchasedModule() {
			a.mainView = "rack"
			a.rackFocusRight = false
			a.rackCatalogSelected = false
			a.rackRemovalArmedModule = ""
		} else if a.state.CurrentTab == game.TabPrint {
			a.message = "Hardware rack unlocks after you acquire your first module."
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
	case "t": if a.state.StorageUnlocked { _ = a.state.OpenTab(game.TabStorage); a.message = "" }
	case "u": if a.state.CurrentTab == game.TabPrint { a.mainView = "upgrades"; a.selectedUpgrade = 0; a.upgradeBuyArmed = false; a.message = "" }
	case "r": if a.state.CurrentTab == game.TabPrint && a.hasPurchasedModule() { a.mainView = "rack"; a.rackFocusRight = false; a.rackCatalogSelected = false; a.rackRemovalArmedModule = ""; a.message = "" } else if a.state.CurrentTab == game.TabPrint { a.message = "Hardware rack unlocks after you acquire your first module." }
	case "m", "e":
		if err := a.state.OpenTab(game.TabMessages); err != nil { a.message = err.Error() } else {
			if m := a.state.SelectedMessage(); m != nil { _ = a.state.SelectMessage(m.ID, now) }
			a.message = ""
		}
	case "i":
		if err := a.state.IgnoreSelectedMessage(now); err != nil { a.message = err.Error() } else { a.message = "You leave the message unanswered." }
	case "b":
		if a.state.CurrentTab == game.TabPrint && a.mainView == "upgrades" { a.buySelectedModule(now) } else { _ = a.state.OpenTab(game.TabPrint); a.mainView = "print"; a.message = "" }
	case "esc": _ = a.state.OpenTab(game.TabPrint); a.mainView = "print"; a.message = ""
	case "v":
		if err := save.Write(savePath, a.state); err != nil { a.message = "Save failed: "+err.Error() } else { a.message = "Saved to "+savePath }
	case "h", "?": a.message = "X/P inject  •  U upgrades  •  R rack  •  S main  •  T storage  •  M messages  •  1–3 reply  •  V save  •  Q quit"
	case "q": a.autosave(); return tea.Quit
	}
	return nil
}
func (a *App) autosave() { if a.screen == screenGame { if err := save.Write(savePath, a.state); err != nil { a.message = "Autosave failed: "+err.Error() } } }

func (a *App) View() string {
	var view string
	switch a.screen {
	case screenMenu:
		view = a.menuView()
	case screenIntro:
		view = a.introView()
	default:
		view = a.gameView()
	}
	// Bubble Tea redraws a full-screen view as a grid. Keep that grid's
	// dimensions stable and overwrite blank cells so changing panels cannot
	// leave stale borders or cause the terminal to scroll.
	return stableTerminalFrame(view, max(1, a.width-1), max(1, a.height-1))
}

func stableTerminalFrame(view string, width, height int) string {
	width = max(1, width)
	height = max(1, height)

	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			// Last-resort clipping is ANSI-aware: terminal style sequences
			// must never be cut in half and printed as visible characters.
			line = truncate(line, width)
		}
		if cells := lipgloss.Width(line); cells < width {
			line += strings.Repeat(" ", width-cells)
		}
		lines[i] = line
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return strings.Join(lines, "\n")
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
		a.upgradeBuyArmed = false
		return
	}
	if a.mainView == "rack" {
		a.rackRemovalArmedModule = ""
		if a.rackFocusRight {
			bays := a.rackBays()
			a.selectedRackModule = max(0, min(max(0, len(bays)-1), a.selectedRackModule+delta))
			paneHeight := max(4, panelLineLimit(a.layout(), a.message != "")-2)
			visible := visibleRackBays(paneHeight)
			if a.selectedRackModule >= a.rackRightScroll+visible { a.rackRightScroll = a.selectedRackModule-visible+1 }
			if a.selectedRackModule < a.rackRightScroll { a.rackRightScroll = a.selectedRackModule }
		} else {
			a.selectedUpgrade = max(0, min(len(game.ModuleCatalog)-1, a.selectedUpgrade+delta))
			a.rackCatalogSelected = true
			visibleLeft := max(2, (a.height-14)/3)
			if a.selectedUpgrade >= a.rackLeftScroll+visibleLeft { a.rackLeftScroll = a.selectedUpgrade-visibleLeft+1 }
			if a.selectedUpgrade < a.rackLeftScroll { a.rackLeftScroll = a.selectedUpgrade }
		}
	}
}

func (a *App) installedModules() []game.ModuleState {
	var result []game.ModuleState
	for _, m := range a.state.Modules { if m.Installed && !m.Jammed { result = append(result,m) } }
	for _, m := range a.state.Modules { if m.Installed && m.Jammed { result = append(result,m) } }
	return result
}

func (a *App) selectRackModule(id string) {
	for i, m := range a.rackBays() {
		if m != nil && m.ID == id {
			a.selectedRackModule = i
			return
		}
	}
}

func (a *App) hasPurchasedModule() bool {
	for _, spec := range game.ModuleCatalog {
		if m := a.state.Module(spec.ID); m != nil && m.Level > 0 {
			return true
		}
	}
	return false
}

func (a *App) hasReadyModules() bool {
	for _, spec := range game.ModuleCatalog {
		if m := a.state.Module(spec.ID); m != nil && m.Level > 0 && !m.Installed && !m.Jammed {
			return true
		}
	}
	return false
}

func formatUptime(seconds int64) string {
	if seconds < 0 { seconds = 0 }
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%03d:%02d:%02d", hours, minutes, secs)
}

func (a *App) activateMainSelection() {
	now := time.Now()
	if a.mainView == "upgrades" {
		a.buySelectedModule(now)
		return
	}
	if a.mainView != "rack" {
		return
	}
	if a.rackFocusRight {
		bays := a.rackBays()
		if a.selectedRackModule < 0 || a.selectedRackModule >= len(bays) {
			return
		}
		m := bays[a.selectedRackModule]
		if m == nil {
			a.message = "This bay is empty. Select a purchased module from the left pane."
			return
		}
		if m.Jammed {
			if a.rackRemovalArmedModule != m.ID {
				a.rackRemovalArmedModule = m.ID
				a.message = "Select [REMOVE] again to confirm specialist removal ($50.00)."
				return
			}
			if err := a.state.RemoveJammedLegacyModule(now); err != nil {
				a.message = err.Error()
			} else {
				a.message = "Specialist removed the legacy print controller."
			}
			a.rackRemovalArmedModule = ""
			return
		}
		if err := a.state.RemoveModule(m.ID, now); err != nil {
			a.message = err.Error()
		} else {
			a.message = "Module removed from rack."
		}
		a.rackRemovalArmedModule = ""
		return
	}
	if !a.rackCatalogSelected {
		a.selectedUpgrade = max(0, min(len(game.ModuleCatalog)-1, a.selectedUpgrade))
		a.rackCatalogSelected = true
		a.message = "Selected "+game.ModuleCatalog[a.selectedUpgrade].Name+". Press ENTER to install/remove."
		return
	}
	a.toggleSelectedRackCatalogModule(now)
}

func (a *App) toggleSelectedRackCatalogModule(now time.Time) {
	if a.selectedUpgrade < 0 || a.selectedUpgrade >= len(game.ModuleCatalog) {
		return
	}
	spec := game.ModuleCatalog[a.selectedUpgrade]
	m := a.state.Module(spec.ID)
	if m == nil || m.Level <= 0 {
		a.message = "Purchase "+spec.Name+" in UPGRADES before installing it."
		return
	}
	var err error
	if m.Installed {
		err = a.state.RemoveModule(spec.ID, now)
		if err == nil {
			a.message = spec.Name+" removed from the rack."
		}
	} else {
		err = a.state.InstallModule(spec.ID, now)
		if err == nil {
			a.message = spec.Name+" inserted. Initialization sequence started."
			a.rackFocusRight = true
			a.selectRackModule(spec.ID)
		}
	}
	if err != nil {
		a.message = err.Error()
	}
}

func (a *App) buySelectedModule(now time.Time) {
	if a.selectedUpgrade < 0 || a.selectedUpgrade >= len(game.ModuleCatalog) {
		return
	}
	spec := game.ModuleCatalog[a.selectedUpgrade]
	if spec.ID == "obfuscation" && a.state.Risk < 25 {
		a.message = "Signal obfuscation remains unavailable until scrutiny increases."
		return
	}
	if err := a.state.PurchaseModule(spec.ID, now); err != nil {
		a.message = err.Error()
	} else {
		m := a.state.Module(spec.ID)
		a.message = spec.Name+" revision "+fmt.Sprint(m.Level)+" purchased. Ready for installation in HARDWARE RACK."
	}
	a.upgradeBuyArmed = false
}

func lWidthForRack(width int) int { return max(24,min(38,width/3)) }

func moduleGain(id string, level int) string {
	switch id {
	case "window_expansion": return fmt.Sprintf("+%ds EXPLOIT WINDOW", level)
	case "queue_calibration": return fmt.Sprintf("+%ds QUEUE WINDOW", level)
	case "auto_injector": return fmt.Sprintf("+%d AUTO INJECTION", level)
	case "obfuscation": return "NO ACTIVE EFFECT"
	default: return "SYSTEM HARDWARE"
	}
}

func bootMessage(m game.ModuleState) string {
	elapsed := m.BootTicksTotal-m.BootTicksRemaining
	messages := []string{"BUS ENUMERATION IN PROGRESS", "FIRMWARE SIGNATURE VERIFIED", "CALIBRATING TIMING BUS", "LOADING CONTROL ROUTINES", "RUNNING INTEGRITY CHECK", "FINALIZING MODULE HANDSHAKE"}
	if elapsed < 0 { elapsed = 0 }
	idx := min(len(messages)-1, elapsed*len(messages)/max(1,m.BootTicksTotal))
	return messages[idx]
}

func (a *App) gameView() string {
	l := a.layout()
	var b strings.Builder
	b.WriteString(a.newsStripView(max(12, l.Width-4))+"\n")
	b.WriteString(lineStyle.Render(strings.Repeat("─", max(20, l.Width-4)))+"\n")
	tabs := []game.Tab{game.TabPrint}
	if a.state.StorageUnlocked { tabs = append(tabs, game.TabStorage) }
	if a.state.EmailUnlocked { tabs = append(tabs, game.TabMessages) }
	var labels []string
	unread := unreadCount(a.state.Messages)
	urgent := a.state.ITSuspicion >= 4 || (a.state.PendingReplyTicks > 0 && a.state.PendingReplyTicks <= 2)
	for _, tab := range tabs {
		label := "MAIN"
		if tab == game.TabStorage { label = "STORAGE" }
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
		if a.state.CurrentTab == game.TabStorage { selectedLabel = "[STORAGE]" }
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
	case game.TabStorage: panel = a.storagePanel(l)
	case game.TabMessages: panel = a.messagesPanel(l)
	}
	panelLines := panelLineLimit(l, a.message != "")
	panel = fitLines(panel, panelLines)
	b.WriteString(panel+"\n")
	if a.message != "" { b.WriteString(subtitleStyle.Render(truncate(a.message, max(12, l.Width-4)))+"\n") }
	for strings.Count(b.String(), "\n") < l.TickerY-1 { b.WriteString("\n") }
	b.WriteString(labelStyle.Render("X/P inject  U upgrades  R rack  S main  T storage  M messages  1–3 reply  V save  H help  Q quit"))
	return lipgloss.NewStyle().Padding(1,2).Render(b.String())
}


func (a *App) cashPanelView(width int) string {
	boxWidth := max(28,min(width-4,68))
	inner := boxWidth-2
	title := " CAPITAL "
	top := "┌─"+title+strings.Repeat("─",max(0,inner-len(title)-1))+"┐"
	cashLine := " CAPITAL ON HAND  $"+money(a.state.CashOnHand)
	capacityLine := " CAPACITY         $"+money(a.state.CashCapacity)
	financialLine := " EST. NET WORTH ~$"+money(a.state.EstimatedNetWorth)+"  |  PRINTED $"+money(a.state.TotalPrinted)
	util := 0
	if a.state.CashCapacity > 0 { util = int(a.state.CashOnHand*100/a.state.CashCapacity) }
	barWidth := max(8,inner-16)
	filled := min(barWidth,max(0,util*barWidth/100))
	bar := strings.Repeat("━",filled)+strings.Repeat("─",barWidth-filled)
	barColor := green
	if util >= 80 { barColor = amber }
	if util >= 100 {
		barColor = red
		r := []rune(bar)
		if len(r)>0 {
			if a.uiFrame%6 < 3 { r[len(r)-1] = '!' } else { r[len(r)-1] = ' ' }
		}
		bar = string(r)
	}
	status := fmt.Sprintf("%d%% UTILIZED",util)
	if util >= 100 { status = fmt.Sprintf("%d%% OVERFLOW",util) }
	barLine := bar+"  "+status
	rows := []string{
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(amber).Bold(true).Render(truncate(cashLine,inner))+strings.Repeat(" ",max(0,inner-lipgloss.Width(truncate(cashLine,inner))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(ink).Render(truncate(capacityLine,inner))+strings.Repeat(" ",max(0,inner-lipgloss.Width(truncate(capacityLine,inner))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(cyan).Bold(true).Render(truncate(financialLine,inner))+strings.Repeat(" ",max(0,inner-lipgloss.Width(truncate(financialLine,inner))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
		lipgloss.NewStyle().Foreground(muted).Render("│")+lipgloss.NewStyle().Foreground(barColor).Render(truncate(barLine,inner))+strings.Repeat(" ",max(0,inner-lipgloss.Width(truncate(barLine,inner))))+lipgloss.NewStyle().Foreground(muted).Render("│"),
	}
	return lipgloss.NewStyle().Width(boxWidth).Render(strings.Join(append([]string{top},append(rows,"└"+strings.Repeat("─",inner)+"┘")...), "\n"))
}

func (a *App) printPanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MAIN  /  PRINT OPERATIONS")+"   ")
	if a.state.Status == game.RunFederal || a.state.Status == game.RunSandbox { b.WriteString(okStyle.Render("SYSTEM READY")) } else { b.WriteString(dangerStyle.Render("SYSTEM OFFLINE")) }
	b.WriteString("\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("RUN: %-8s  UPTIME: %s  EXPOSURE: %02d/100", strings.ToUpper(string(a.state.Status)), formatUptime(a.state.UptimeSeconds), a.state.Risk))+"\n")
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
	b.WriteString("\n"+selectedStyle.Render("[U] UPGRADES")+"   ")
	if a.hasPurchasedModule() {
		rackLabel := "[R] HARDWARE RACK"
		switch {
		case a.hasReadyModules() && a.uiFrame%6 < 3:
			b.WriteString(okStyle.Render(rackLabel)+"\n")
		case a.hasReadyModules():
			b.WriteString(labelStyle.Render(rackLabel)+"\n")
		default:
			b.WriteString(selectedStyle.Render(rackLabel)+"\n")
		}
	} else {
		b.WriteString(labelStyle.Render("[R] HARDWARE RACK — LOCKED")+"\n")
	}
	b.WriteString(labelStyle.Render("The machine is old. The unused bays still accept modules."))
	return b.String()
}

func (a *App) upgradesPanel(l uiLayout) string {
	leftWidth, rightWidth := paneWidths(l)
	paneHeight := panelLineLimit(l, a.message != "")
	var left, right strings.Builder
	for i, spec := range game.ModuleCatalog {
		m := a.state.Module(spec.ID)
		revision := ""
		if m != nil && m.Level > 0 {
			revision = fmt.Sprintf(" Rev %02d", m.Level)
		}
		marker := " "
		if i == a.selectedUpgrade { marker = "›" }
		nameWidth := max(8, leftWidth-4-lipgloss.Width(revision))
		name := truncate(marker+" "+spec.Name, nameWidth)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(spec.Color))
		if i == a.selectedUpgrade { style = style.Bold(true).Underline(true) }
		left.WriteString(padCells(style.Render(name), labelStyle.Render(revision), leftWidth-2)+"\n")
	}
	idx := max(0, min(len(game.ModuleCatalog)-1, a.selectedUpgrade))
	spec := game.ModuleCatalog[idx]
	m := a.state.Module(spec.ID)
	level := 0
	installed := false
	if m != nil {
		level = m.Level
		installed = m.Installed
	}
	right.WriteString(titleStyle.Render(spec.Name)+"\n")
	right.WriteString(wrapText(spec.Description, max(18, rightWidth-4))+"\n")
	right.WriteString(labelStyle.Render(fmt.Sprintf("CURRENT REVISION: %02d / %02d", level, spec.MaxLevel))+"\n")
	right.WriteString(labelStyle.Render(fmt.Sprintf("SUSPICION PROFILE: %d / 5", spec.Suspicion))+"\n")
	if spec.ID == "obfuscation" && a.state.Risk < 25 {
		right.WriteString(dangerStyle.Render("REQUIREMENT: scrutiny 25+")+"\n")
	} else if level >= spec.MaxLevel {
		if installed {
			right.WriteString(okStyle.Render("MAXIMUM REVISION INSTALLED")+"\n")
		} else {
			right.WriteString(okStyle.Render("MAXIMUM REVISION OWNED")+"\n")
			right.WriteString(labelStyle.Render("INSTALL IT FROM HARDWARE RACK")+"\n")
		}
	} else {
		if level == 0 {
			right.WriteString(labelStyle.Render("ACQUISITION COST: $"+money(spec.Cost))+"\n")
		} else {
			right.WriteString(labelStyle.Render("NEXT REVISION COST: $"+money(spec.Cost*int64(level+1)))+"\n")
			if !installed {
				right.WriteString(okStyle.Render("READY FOR INSTALLATION IN HARDWARE RACK")+"\n")
			} else {
				right.WriteString(labelStyle.Render("PURCHASING THIS REVISION REQUIRES REINSTALLATION")+"\n")
			}
		}
		buyLabel := "[ENTER / B] BUY / UPGRADE"
		if a.upgradeBuyArmed {
			buyLabel = "[CONFIRM] CLICK BUY / UPGRADE AGAIN"
			right.WriteString(okStyle.Render(buyLabel)+"\n")
		} else {
			right.WriteString(selectedStyle.Render(buyLabel)+"\n")
		}
	}
	right.WriteString(labelStyle.Render("[R] HARDWARE RACK   [ESC] RETURN TO OPERATIONS"))
	return lipgloss.JoinHorizontal(lipgloss.Top,
		windowPane("MODULE CATALOG", left.String(), leftWidth, paneHeight),
		" ",
		windowPane("MODULE DETAILS", right.String(), rightWidth, paneHeight))
}

const rackBayHeight = 6

func visibleRackBays(paneHeight int) int {
	// Reserve one row for the rack header and one for the action hint.
	return max(1, (max(1, paneHeight-2)-2)/rackBayHeight)
}

// rackBays returns populated bays first, followed by visible empty slots.
// Removing the jammed legacy controller exposes two additional bays.
func (a *App) rackBays() []*game.ModuleState {
	var bays []*game.ModuleState
	for i := range a.state.Modules {
		m := &a.state.Modules[i]
		if m.Installed && !m.Jammed {
			bays = append(bays, m)
		}
	}
	for i := range a.state.Modules {
		m := &a.state.Modules[i]
		if m.Installed && m.Jammed {
			bays = append(bays, m)
		}
	}
	capacity := 3
	legacy := a.state.Module("legacy_controller")
	if legacy == nil || !legacy.Installed {
		capacity = 5
	}
	// Always leave at least one empty bay as the next insertion target.
	capacity = max(capacity, len(bays)+1)
	for len(bays) < capacity {
		bays = append(bays, nil)
	}
	return bays
}

func (a *App) rackPanel(l uiLayout) string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("MAIN  /  HARDWARE RACK")+"\n")
	bays := a.rackBays()
	occupied := 0
	for _, bay := range bays {
		if bay != nil { occupied++ }
	}
	out.WriteString(labelStyle.Render(fmt.Sprintf("RACK BAY ARRAY: %02d SLOTS  ·  IN USE: %02d", len(bays), occupied))+"\n")
	leftWidth, rightWidth := paneWidths(l)
	paneHeight := max(4, panelLineLimit(l, a.message != "")-2)
	visibleLeft := max(1, paneHeight-2)
	visibleRight := visibleRackBays(paneHeight)
	var left strings.Builder

	start := min(a.rackLeftScroll, max(0, len(game.ModuleCatalog)-visibleLeft))
	end := min(len(game.ModuleCatalog), start+visibleLeft)
	leftInner := leftWidth-2
	nameWidth := max(1, leftInner-2)
	for i := start; i < end; i++ {
		spec := game.ModuleCatalog[i]
		m := a.state.Module(spec.ID)
		symbol := " "
		if m != nil && m.Level > 0 {
			if m.Installed { symbol = "X" } else { symbol = ">" }
		}
		name := spec.Name
		if a.rackCatalogSelected && i == a.selectedUpgrade {
			name = "["+name+"]"
		}
		row := padCells(name, "", nameWidth)+" "+symbol
		rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color(spec.Color)).Width(leftInner)
		left.WriteString(rowStyle.Render(row)+"\n")
	}

	startR := min(max(0, a.rackRightScroll), max(0, len(bays)-visibleRight))
	endR := min(len(bays), startR+visibleRight)
	paneInner := rightWidth-2
	contentWidth := paneInner-1 // reserve the final interior column for the vertical scrollbar
	cardWidth := max(20, min(contentWidth, 54))
	cardInner := cardWidth-2
	cardPad := max(0, (contentWidth-cardWidth)/2)
	rightLines := []string{titleStyle.Render("INSTALLED RACK")}
	for i := startR; i < endR; i++ {
		m := bays[i]
		var card []string
		name, color := "", teal
		if m == nil {
			card = emptyRackBayCard(cardInner)
		} else {
			name = m.ID
			for _, spec := range game.ModuleCatalog {
				if spec.ID == m.ID {
					name, color = spec.Name, lipgloss.Color(spec.Color)
					break
				}
			}
			if m.ID == "legacy_controller" { name, color = "LEGACY PRINT CONTROLLER", orange }
			selected := i == a.selectedRackModule && a.rackFocusRight
			card = rackModuleCard(*m, name, cardInner, a.rackRemovalArmedModule == m.ID, a.uiFrame, a.diskFrame)
			for row := range card {
				paddedLine := strings.Repeat(" ", cardPad)+card[row]+strings.Repeat(" ", max(0, contentWidth-cardPad-lipgloss.Width(card[row])))
				if m.Jammed {
					card[row] = orangeRackStyle(selected).Render(paddedLine)
				} else {
					style := lipgloss.NewStyle().Foreground(color)
					if selected { style = style.Bold(true) }
					card[row] = style.Render(paddedLine)
				}
			}
		}
		if m == nil {
			for row := range card {
				paddedLine := strings.Repeat(" ", cardPad)+card[row]+strings.Repeat(" ", max(0, contentWidth-cardPad-lipgloss.Width(card[row])))
				card[row] = labelStyle.Render(paddedLine)
			}
		}
		rightLines = append(rightLines, card...)
	}
	rightLines = append(rightLines, labelStyle.Render("[X] REMOVE MODULE  ·  [REMOVE] CONFIRM SPECIALIST"))
	leftPane := windowPane("MODULES", left.String(), leftWidth, paneHeight)
	rightPane := windowPaneWithVerticalScrollbar("RACK BUS", rightLines, rightWidth, paneHeight, len(bays), startR, visibleRight)
	out.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, " ", rightPane))
	return out.String()
}

// rackModuleCard returns a six-row hardware face regardless of boot state.
// Its title sits inside the enclosure; the boot bar is replaced by an HDD-style
// status LED once initialization finishes, keeping the bay dimensions stable.
func rackModuleCard(m game.ModuleState, name string, inner int, removalArmed bool, frame, diskFrame int64) []string {
	top := "┌"+strings.Repeat("─", inner)+"┐"
	if m.Jammed {
		button := "$50 - [REMOVE]"
		if removalArmed {
			button = selectedStyle.Render(button)
		} else {
			button = dangerStyle.Render(button)
		}
		return []string{
			top,
			"│"+padCells(" "+truncate(name, max(1,inner-1)), "", inner)+"│",
			"│"+padCells(" JAMMED - SPECIALIST REMOVAL", "", inner)+"│",
			"│"+strings.Repeat(" ", inner)+"│",
			"│"+padCells("", button, inner)+"│",
			"└"+strings.Repeat("─", inner)+"┘",
		}
	}
	remove := "[X]"
	title := "│"+padCells(" "+truncate(name, max(1,inner-5)), remove, inner)+"│"
	revLine := "│"+padCells(fmt.Sprintf(" REV %02d", m.Level), moduleGain(m.ID, m.Level), inner)+"│"
	statusLine := ""
	if m.Booting {
		statusLine = "│"+padCells(" "+bootMessage(m), "", inner)+"│"
	} else if m.ActivatedFlash {
		statusLine = "│"+padCells(" ACTIVATED!", "", inner)+"│"
	} else {
		flicker := "*  *  *  *  *"
		if frame%7 < 3 { flicker = "* *   *  * *" }
		statusLine = "│"+padCells(" "+flicker, "", inner)+"│"
	}
	activityLine := ""
	switch {
	case m.Booting:
		barWidth := max(1, inner-3)
		done := m.BootTicksTotal-m.BootTicksRemaining
		filled := min(barWidth, max(0, done*barWidth/max(1,m.BootTicksTotal)))
		activityLine = "│"+padCells(" ["+strings.Repeat("█",filled)+strings.Repeat("·",barWidth-filled)+"]", "", inner)+"│"
	case m.ActivatedFlash:
		barWidth := max(1, inner-3)
		activityLine = "│"+padCells(" ["+strings.Repeat("█",barWidth)+"]", "", inner)+"│"
	default:
		led := labelStyle.Render(".")
		if diskFrame%13 < 4 {
			led = okStyle.Render(".")
		}
		activityLine = "│"+strings.Repeat(" ", max(0,inner-1))+led+"│"
	}
	return []string{top, title, revLine, statusLine, activityLine, "└"+strings.Repeat("─",inner)+"┘"}
}

func emptyRackBayCard(inner int) []string {
	return []string{
		"┌"+strings.Repeat("─", inner)+"┐",
		"│"+strings.Repeat(" ", inner)+"│",
		"│"+strings.Repeat(" ", inner)+"│",
		"│"+strings.Repeat(" ", inner)+"│",
		"│"+strings.Repeat(" ", inner)+"│",
		"└"+strings.Repeat("─", inner)+"┘",
	}
}

func orangeRackStyle(selected bool) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(orange)
	if selected { style = style.Bold(true) }
	return style
}

func rackCardHeight(game.ModuleState) int { return rackBayHeight }


func windowPane(title, content string, width, height int) string {
	width = max(18, width)
	height = max(3, height)
	inner := width-2
	topLabel := "┌─ "+title+" "
	top := topLabel+strings.Repeat("─", max(0, width-lipgloss.Width(topLabel)-1))+"┐"
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	contentHeight := height-2
	if len(lines) > contentHeight { lines = lines[:contentHeight] }
	for len(lines) < contentHeight { lines = append(lines, "") }
	var out strings.Builder
	out.WriteString(labelStyle.Render(top)+"\n")
	for _, line := range lines {
		if lipgloss.Width(line) > inner { line = truncate(line, inner) }
		out.WriteString(labelStyle.Render("│")+line+strings.Repeat(" ", max(0, inner-lipgloss.Width(line)))+labelStyle.Render("│")+"\n")
	}
	out.WriteString(labelStyle.Render("└"+strings.Repeat("─", inner)+"┘"))
	return out.String()
}


func windowPaneWithVerticalScrollbar(title string, content []string, width, height, total, start, visible int) string {
	width = max(18, width)
	height = max(3, height)
	inner := width-2
	contentWidth := inner-1
	topLabel := "┌─ "+title+" "
	top := topLabel+strings.Repeat("─", max(0, width-lipgloss.Width(topLabel)-1))+"┐"
	contentHeight := height-2

	// The scrollbar rail is one cell inside the right border. Its thumb tracks
	// bay selection/scroll position vertically, rather than using a horizontal
	// progress strip in the rack title.
	thumbHeight := contentHeight
	thumbStart := 0
	if total > visible {
		thumbHeight = max(1, contentHeight*visible/max(1,total))
		thumbStart = min(max(0,contentHeight-thumbHeight), start*max(1,contentHeight-thumbHeight)/max(1,total-visible))
	}
	var out strings.Builder
	out.WriteString(labelStyle.Render(top)+"\n")
	for row := 0; row < contentHeight; row++ {
		line := ""
		if row < len(content) { line = content[row] }
		if lipgloss.Width(line) > contentWidth { line = truncate(line, contentWidth) }
		line += strings.Repeat(" ", max(0,contentWidth-lipgloss.Width(line)))
		scrollChar := "░"
		scrollStyle := labelStyle
		if row >= thumbStart && row < thumbStart+thumbHeight {
			scrollChar = "█"
			scrollStyle = okStyle
		}
		out.WriteString(labelStyle.Render("│")+line+scrollStyle.Render(scrollChar)+labelStyle.Render("│")+"\n")
	}
	out.WriteString(labelStyle.Render("└"+strings.Repeat("─", inner)+"┘"))
	return out.String()
}

func paneWidths(l uiLayout) (int, int) {
	available := max(40, l.Width-4)
	left := max(24, min(38, available/3))
	right := max(18, available-left-1)
	return left, right
}

func panelLineLimit(l uiLayout, hasMessage bool) int {
	n := max(4, l.TickerY-l.ContentY-2)
	if hasMessage { n-- }
	return n
}

func rackScrollbar(total, start, visible int) string {
	const height = 6
	if total <= visible { return "[══════]" }
	thumb := max(1, height*visible/total)
	pos := min(height-thumb, max(0, start*height/max(1, total-visible)))
	return "["+strings.Repeat("░", pos)+strings.Repeat("█", thumb)+strings.Repeat("░", height-pos-thumb)+"]"
}

func padCells(left, right string, width int) string {
	left = truncate(left, max(0, width-lipgloss.Width(right)))
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	if lipgloss.Width(left)+lipgloss.Width(right) >= width { gap = 0 }
	return left+strings.Repeat(" ", gap)+right+strings.Repeat(" ", max(0, width-lipgloss.Width(left)-lipgloss.Width(right)-gap))
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

func (a *App) storagePanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("STORAGE  /  PERSONAL PROPERTY")+"\n")
	b.WriteString(labelStyle.Render("RESIDENCE: CITY APARTMENT  ·  LIQUID CAPITAL CAPACITY")+"\n\n")
	b.WriteString(titleStyle.Render("APARTMENT INVENTORY")+"\n")
	b.WriteString(fmt.Sprintf("  Loose cash capacity     $%s\n",money(game.InitialCashCapacity)))
	b.WriteString(fmt.Sprintf("  Lockboxes owned          %d\n",a.state.LockboxCount))
	b.WriteString(fmt.Sprintf("  Total capacity            $%s\n",money(a.state.CashCapacity)))
	b.WriteString(fmt.Sprintf("  Capital currently held    $%s\n\n",money(a.state.CashOnHand)))
	if a.state.CashOnHand >= game.LockboxCost {
		b.WriteString(selectedStyle.Render("[1] BUY LOCKBOX  —  $"+money(game.LockboxCost))+"\n")
	} else {
		b.WriteString(labelStyle.Render("[1] BUY LOCKBOX  —  $"+money(game.LockboxCost)+" (INSUFFICIENT CAPITAL)")+"\n")
	}
	nextCapacity := game.LockboxCapacity
	if a.state.LockboxCount >= 3 { nextCapacity = 350_000 }
	b.WriteString(labelStyle.Render("  Capacity increase: +$"+money(nextCapacity))+"\n")
	if a.state.LockboxCount >= 3 { b.WriteString(dangerStyle.Render("  CLUTTER: additional boxes attract attention and waste space.")+"\n") }
	b.WriteString("\n"+labelStyle.Render("Cash above capacity remains yours, but increases exposure."))
	b.WriteString("\n"+labelStyle.Render("Larger containers and manual transfers will follow in a later storage pass."))
	b.WriteString("\n\n"+labelStyle.Render("[S] MAIN  ·  [T] STORAGE  ·  [1] BUY LOCKBOX"))
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

func visiblePaneText(line string, start, width int) string {
	runes := []rune(line)
	if start >= len(runes) || width <= 0 { return "" }
	end := min(len(runes), start+width)
	return string(runes[start:end])
}

func mouseWithin(line string,x int,label string) bool {
	at:=strings.Index(line,label)
	return at>=0 && x>=at && x<at+len(label)
}

func (a *App) mouseOnTabRow(y int) bool {
	lines := plainViewLines(a.View())
	return y >= 0 && y < len(lines) && (strings.Contains(lines[y], "[MAIN]") || strings.Contains(lines[y], "[STORAGE]") || strings.Contains(lines[y], "[MESSAGES"))
}

func (a *App) isInjectAt(x, y int) bool {
	if a.screen != screenGame || a.state.CurrentTab != game.TabPrint { return false }
	lines := plainViewLines(a.View())
	if y < 0 || y >= len(lines) { return false }
	return strings.Contains(lines[y], "INJECT")
}

func (a *App) cycleTab(delta int) {
	tabs := []game.Tab{game.TabPrint}
	if a.state.StorageUnlocked { tabs = append(tabs, game.TabStorage) }
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
func truncate(s string, width int) string {
	if width < 1 {
		return ""
	}
	plain := ansiSequences.ReplaceAllString(s, "")
	if lipgloss.Width(plain) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}

	// Copy ANSI control sequences whole, while counting only visible rune
	// cells. Truncating the raw string by rune count can split true-colour
	// escapes (for example ESC[38;2;119;121;112m) and corrupt the whole frame.
	target := width - 1
	var out strings.Builder
	used := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			if loc := ansiSequences.FindStringIndex(s[i:]); loc != nil && loc[0] == 0 {
				out.WriteString(s[i : i+loc[1]])
				i += loc[1]
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		runeText := string(r)
		cells := lipgloss.Width(runeText)
		if used+cells > target {
			break
		}
		out.WriteString(runeText)
		used += cells
		i += size
	}
	out.WriteRune('…')
	if strings.Contains(s, "\x1b[") {
		out.WriteString("\x1b[0m")
	}
	return out.String()
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
func money(cents int64) string {
	whole := fmt.Sprintf("%d",cents/100)
	for i:=len(whole)-3; i>0; i-=3 { whole = whole[:i]+","+whole[i:] }
	return fmt.Sprintf("%s.%02d",whole,cents%100)
}
