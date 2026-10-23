package terminal

import (
	"fmt"
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

type App struct {
	state   game.State
	screen  screen
	width   int
	height  int
	menu    int
	message string
}

func New(state game.State) *App {
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
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return a, a.handleMouse(msg.X, msg.Y)
		}
	case tickMsg:
		if a.screen == screenGame {
			a.state.Advance(time.Time(msg))
			return a, tick()
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			a.autosave()
			return a, tea.Quit
		case "up", "k":
			if a.screen == screenMenu && a.menu > 0 {
				a.menu--
			}
		case "down", "j":
			if a.screen == screenMenu && a.menu < 2 {
				a.menu++
			}
		case "enter":
			switch a.screen {
			case screenMenu:
				return a, a.chooseMenu()
			case screenIntro:
				a.screen = screenGame
				a.message = ""
				return a, tick()
			}
		case "n":
			if a.screen == screenMenu {
				a.screen = screenIntro
			}
		case "l":
			if a.screen == screenMenu {
				a.loadGame()
				if a.screen == screenGame {
					return a, tick()
				}
			}
		case "q":
			if a.screen == screenMenu {
				return a, tea.Quit
			}
			a.autosave()
			return a, tea.Quit
		case "esc":
			switch a.screen {
			case screenMenu:
				return a, tea.Quit
			case screenIntro:
				a.screen = screenMenu
			case screenGame:
				_ = a.state.OpenTab(game.TabPrint)
				a.message = ""
			}
		default:
			if a.screen == screenGame {
				return a, a.handleGameKey(msg.String())
			}
		}
	}
	return a, nil
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (a *App) chooseMenu() tea.Cmd {
	switch a.menu {
	case 0:
		a.state = game.NewState(time.Now())
		a.screen = screenIntro
		a.message = ""
	case 1:
		a.loadGame()
		if a.screen == screenGame {
			return tick()
		}
	case 2:
		return tea.Quit
	}
	return nil
}

func (a *App) loadGame() {
	loaded, err := save.Read(savePath)
	if err != nil {
		a.message = "No readable save found. Choose NEW OPERATION to begin."
		return
	}
	a.state = loaded
	a.screen = screenGame
	a.message = "Operation loaded."
}

// handleMouse maps clicks to the same actions available from the keyboard.
func (a *App) handleMouse(x, y int) tea.Cmd {
	switch a.screen {
	case screenMenu:
		// The menu is intentionally left-aligned. These rows correspond to its
		// three options; clicking an option selects it immediately.
		switch {
		case y == 9:
			a.menu = 0
			return a.chooseMenu()
		case y == 10:
			a.menu = 1
			return a.chooseMenu()
		case y == 11:
			a.menu = 2
			return a.chooseMenu()
		}
	case screenIntro:
		if y >= 8 {
			a.screen = screenGame
			a.message = ""
			return tick()
		}
	case screenGame:
		// Tab strip near the top of the dashboard.
		if y >= 10 && y <= 13 {
			switch {
			case x >= 2 && x <= 11:
				_ = a.state.OpenTab(game.TabPrint)
				a.message = ""
			case x >= 13 && x <= 23:
				_ = a.state.OpenTab(game.TabStore)
				a.message = ""
			case x >= 25 && x <= 36 && a.state.EmailUnlocked:
				if err := a.state.OpenTab(game.TabEmail); err != nil {
					a.message = err.Error()
				}
			}
			return nil
		}
		// Click the visible primary action or upgrade row.
		switch a.state.CurrentTab {
		case game.TabPrint:
			if y >= 15 && y <= 18 {
				return a.handleGameKey("p")
			}
			if y >= 19 && y <= 21 {
				return a.handleGameKey("s")
			}
			if y >= 22 && y <= 24 && a.state.EmailUnlocked {
				return a.handleGameKey("e")
			}
		case game.TabStore:
			if y >= 15 && y <= 17 {
				return a.handleGameKey("1")
			}
			if y >= 18 && y <= 20 {
				return a.handleGameKey("2")
			}
			if y >= 21 && y <= 23 {
				return a.handleGameKey("b")
			}
		case game.TabEmail:
			if y >= 20 && y <= 23 {
				return a.handleGameKey("b")
			}
		}
	}
	return nil
}

func (a *App) handleGameKey(key string) tea.Cmd {
	now := time.Now()
	switch key {
	case "p", " ":
		if err := a.state.PrintOneDollar(now); err != nil {
			a.message = err.Error()
		} else {
			a.message = "One dollar. Crisp, unrecorded, and real."
		}
	case "1":
		if a.state.CurrentTab == game.TabStore {
			if err := a.state.BuyCapacityUpgrade(now); err != nil {
				a.message = err.Error()
			} else {
				a.message = "Cash box installed."
			}
		}
	case "2":
		if a.state.CurrentTab == game.TabStore {
			if err := a.state.BuyAutomation(now); err != nil {
				a.message = err.Error()
			} else {
				a.message = "Automation enabled."
			}
		}
	case "s":
		_ = a.state.OpenTab(game.TabStore)
		a.message = ""
	case "e":
		if err := a.state.OpenTab(game.TabEmail); err != nil {
			a.message = err.Error()
		} else if err := a.state.ReadEmail(now); err != nil {
			a.message = err.Error()
		} else {
			a.message = "Message opened."
		}
	case "b", "esc":
		_ = a.state.OpenTab(game.TabPrint)
		a.message = ""
	case "v":
		if err := save.Write(savePath, a.state); err != nil {
			a.message = "Save failed: " + err.Error()
		} else {
			a.message = "Saved to " + savePath
		}
	case "h", "?":
		a.message = "P print  •  S store  •  E email  •  B back  •  V save  •  Q quit"
	case "q":
		a.autosave()
		return tea.Quit
	}
	return nil
}

func (a *App) autosave() {
	if a.screen == screenGame {
		if err := save.Write(savePath, a.state); err != nil {
			a.message = "Autosave failed: " + err.Error()
		}
	}
}

func (a *App) View() string {
	switch a.screen {
	case screenMenu:
		return a.menuView()
	case screenIntro:
		return a.introView()
	default:
		return a.gameView()
	}
}

var (
	ink       = lipgloss.Color("#D8D8D0")
	muted     = lipgloss.Color("#777970")
	red       = lipgloss.Color("#B84B45")
	amber     = lipgloss.Color("#D6B36A")
	lineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#42453F"))
	titleStyle = lipgloss.NewStyle().Foreground(ink).Bold(true)
	subtitleStyle = lipgloss.NewStyle().Foreground(muted).Italic(true)
	labelStyle = lipgloss.NewStyle().Foreground(muted)
	selectedStyle = lipgloss.NewStyle().Foreground(amber).Bold(true)
	dangerStyle = lipgloss.NewStyle().Foreground(red)
)

func (a *App) menuView() string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("DEEP STATE"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(amber).Bold(true).Render("CLICKER"))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Every dollar leaves a mark."))
	b.WriteString("\n\n")
	b.WriteString(subtitleStyle.Render("Every dollar leaves a mark."))
	b.WriteString("\n\n")
	options := []string{"NEW OPERATION", "LOAD OPERATION", "QUIT"}
	for i, option := range options {
		if i == a.menu {
			b.WriteString(selectedStyle.Render("  ▸ " + option))
		} else {
			b.WriteString(labelStyle.Render("    " + option))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if a.message != "" {
		b.WriteString(dangerStyle.Render(a.message))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("↑/↓ navigate   ENTER select   N new   L load   Q quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (a *App) introView() string {
	content := strings.Join([]string{
		"",
		titleStyle.Render("FEDERAL RESERVE  /  INTERNAL OPERATIONS"),
		lineStyle.Render(strings.Repeat("─", max(0, min(a.width-6, 66)))),
		"",
		"You are not an economist.",
		"You reconcile print counts, replace toner, and go home.",
		"",
		"Then one count comes back wrong.",
		"",
		dangerStyle.Render("ONE DOLLAR."),
		"No discrepancy in the ledger. No explanation in the logs.",
		"You run the count again.",
		"",
		subtitleStyle.Render("The number does not change."),
		"",
		labelStyle.Render("Press ENTER to continue."),
	}, "\n")
	return lipgloss.NewStyle().Padding(2, 3).Width(max(40, min(a.width-4, 76))).Render(content)
}

func (a *App) gameView() string {
	var b strings.Builder
	header := titleStyle.Render("DEEP STATE CLICKER") + labelStyle.Render("   /   FEDERAL RESERVE INTERNAL TERMINAL")
	b.WriteString(header + "\n")
	b.WriteString(lineStyle.Render(strings.Repeat("─", max(20, min(a.width-6, 100)))) + "\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("RUN: %-8s  UPTIME: %04ds  EXPOSURE: %02d/100", strings.ToUpper(string(a.state.Status)), a.state.Tick, a.state.Risk)))
	b.WriteString("\n\n")

	cashPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted).Padding(1, 2).Width(max(28, min(a.width-8, 50)))
	cash := labelStyle.Render("CASH ON HAND") + "\n" +
		lipgloss.NewStyle().Foreground(amber).Bold(true).Render("$"+money(a.state.CashOnHand)) +
		labelStyle.Render("  /  $"+money(a.state.CashCapacity)) + "\n" +
		labelStyle.Render("TOTAL PRINTED  $"+money(a.state.TotalPrinted))
	b.WriteString(cashPanel.Render(cash) + "\n\n")

	tabs := []game.Tab{game.TabPrint, game.TabStore}
	if a.state.EmailUnlocked {
		tabs = append(tabs, game.TabEmail)
	}
	var tabLabels []string
	for _, tab := range tabs {
		if tab == a.state.CurrentTab {
			tabLabels = append(tabLabels, selectedStyle.Render("["+string(tab)+"]"))
		} else {
			tabLabels = append(tabLabels, labelStyle.Render(" "+string(tab)+" "))
		}
	}
	b.WriteString(strings.Join(tabLabels, "   ") + "\n")
	b.WriteString(lineStyle.Render(strings.Repeat("─", max(20, min(a.width-6, 100)))) + "\n\n")

	switch a.state.CurrentTab {
	case game.TabPrint:
		b.WriteString(titleStyle.Render("PRINT CONTROL") + "\n\n")
		b.WriteString("  " + selectedStyle.Render("[P] PRINT $1") + "\n")
		b.WriteString(labelStyle.Render("  Manual printing. No receipt. No audit trail.") + "\n")
		if a.state.EmailUnlocked {
			b.WriteString("\n  " + selectedStyle.Render("[E] INTERNAL EMAIL  •  NEW MESSAGE") + "\n")
		} else {
			b.WriteString("\n  " + labelStyle.Render("INTERNAL EMAIL  •  LOCKED") + "\n")
		}
		b.WriteString("\n  " + labelStyle.Render("[S] Open storage and machine upgrades") + "\n")
	case game.TabStore:
		b.WriteString(titleStyle.Render("STORAGE & MACHINE UPGRADES") + "\n\n")
		if a.state.Upgrades.ExpandedCashCapacity {
			b.WriteString("  " + selectedStyle.Render("[1] LOCKABLE CASH BOX  •  INSTALLED") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s  Lockable cash box  —  $%s\n", selectedStyle.Render("[1]"), money(game.CapacityUpgradeCost)))
			b.WriteString(labelStyle.Render("      Expands cash-on-hand capacity to $2,500.") + "\n")
		}
		if a.state.Upgrades.PrintAutomation {
			b.WriteString("  " + selectedStyle.Render("[2] QUIET PRINT AUTOMATION  •  INSTALLED") + "\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s  Quiet print automation  —  $%s\n", selectedStyle.Render("[2]"), money(game.AutomationCost)))
			b.WriteString(labelStyle.Render("      Prints $1 every two seconds while the game is open.") + "\n")
		}
		b.WriteString("\n  " + labelStyle.Render("[B] Return to print control") + "\n")
	case game.TabEmail:
		b.WriteString(titleStyle.Render("INTERNAL EMAIL") + "\n\n")
		b.WriteString("  FROM: IT OPERATIONS\n")
		b.WriteString("  SUBJECT: Print reconciliation discrepancy\n\n")
		b.WriteString("  Unusual print reconciliation. Can you come by before end of day?\n")
		b.WriteString(labelStyle.Render("  The sender has not explained how they found it.") + "\n")
		b.WriteString("\n  " + labelStyle.Render("[B] Return to print control") + "\n")
	}

	if len(a.state.Events) > 0 {
		e := a.state.Events[len(a.state.Events)-1]
		b.WriteString("\n" + lineStyle.Render(strings.Repeat("─", max(20, min(a.width-6, 100)))) + "\n")
		b.WriteString(labelStyle.Render("LATEST RECORD") + "\n")
		b.WriteString(e.Text + "\n")
	}
	if a.message != "" {
		b.WriteString("\n" + subtitleStyle.Render(a.message) + "\n")
	}
	b.WriteString("\n" + labelStyle.Render("[P] print   [S] store   [E] email   [B] back   [V] save   [H] help   [Q] quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func money(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}
