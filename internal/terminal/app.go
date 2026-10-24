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
}

func New(state game.State) *App {
	state.Normalize()
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
			if a.screen == screenMenu && a.menu > 0 { a.menu-- } else if a.screen == screenGame && a.state.CurrentTab == game.TabMessages { a.selectMessage(-1) }
		case "down", "j":
			if a.screen == screenMenu && a.menu < 2 { a.menu++ } else if a.screen == screenGame && a.state.CurrentTab == game.TabMessages { a.selectMessage(1) }
		case "enter":
			switch a.screen {
			case screenMenu:
				return a, a.chooseMenu()
			case screenIntro:
				a.screen, a.message = screenGame, ""
				return a, tick()
			case screenGame:
				if a.state.CurrentTab == game.TabMessages { a.message = "" }
			}
		case "n":
			if a.screen == screenMenu { a.screen = screenIntro }
		case "l":
			if a.screen == screenMenu {
				a.loadGame()
				if a.screen == screenGame { return a, tick() }
			}
		case "q":
			if a.screen == screenMenu { return a, tea.Quit }
			a.autosave()
			return a, tea.Quit
		case "esc":
			switch a.screen {
			case screenMenu: return a, tea.Quit
			case screenIntro: a.screen = screenMenu
			case screenGame: _ = a.state.OpenTab(game.TabPrint); a.message = ""
			}
		default:
			if a.screen == screenGame { return a, a.handleGameKey(msg.String()) }
		}
	}
	return a, nil
}

func tick() tea.Cmd { return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }

func (a *App) chooseMenu() tea.Cmd {
	switch a.menu {
	case 0:
		a.state = game.NewState(time.Now())
		a.screen, a.message = screenIntro, ""
	case 1:
		a.loadGame()
		if a.screen == screenGame { return tick() }
	case 2: return tea.Quit
	}
	return nil
}
func (a *App) loadGame() {
	loaded, err := save.Read(savePath)
	if err != nil { a.message = "No readable save found. Choose NEW OPERATION to begin."; return }
	loaded.Normalize()
	a.state, a.screen, a.message = loaded, screenGame, "Operation loaded."
}

func (a *App) layout() uiLayout {
	w, h := a.width, a.height
	if w < 24 { w = 24 }; if h < 12 { h = 12 }
	sideBySide := w >= 100
	bodyWidth := max(24, w-12)
	choiceBase := 12
	if sideBySide { bodyWidth = max(24, w-max(28,min(36,w/3))-12); choiceBase = 6 }
	choiceY := 0
	if m := a.state.SelectedMessage(); m != nil && len(m.Choices)>0 {
		choiceY = 12 + choiceBase + len(strings.Split(wrapText(m.Body,bodyWidth), "\n"))
	}
	return uiLayout{
		Width:w, Height:h, TabsY:10, ContentY:12,
		ExploitY:17, StoreCapacityY:14, StoreCalibrationY:16, StoreAutomationY:18,
		MessageFirstY:13, MessageLastY:17, MessageChoiceFirstY:choiceY, TickerY:h-2,
		SideBySide:sideBySide,
	}
}

// The same layout object drives both drawing and mouse hit regions.
func (a *App) handleMouse(x, y int) tea.Cmd {
	l := a.layout()
	switch a.screen {
	case screenMenu:
		// Menu rows are derived from the menu's top padding and title block.
		switch y {
		case 8: a.menu = 0; return a.chooseMenu()
		case 9: a.menu = 1; return a.chooseMenu()
		case 10: a.menu = 2; return a.chooseMenu()
		}
	case screenIntro:
		if y >= 8 { a.screen, a.message = screenGame, ""; return tick() }
	case screenGame:
		if y == l.TabsY {
			switch {
			case x >= 2 && x < 15: _ = a.state.OpenTab(game.TabPrint); a.message = ""
			case x >= 16 && x < 29: _ = a.state.OpenTab(game.TabStore); a.message = ""
			case x >= 30 && a.state.EmailUnlocked: _ = a.state.OpenTab(game.TabMessages); a.message = ""
			}
			return nil
		}
		switch a.state.CurrentTab {
		case game.TabPrint:
			if y == l.ExploitY { return a.handleGameKey("x") }
			if y == l.ExploitY+2 { return a.handleGameKey("s") }
			if y == l.ExploitY+3 && a.state.EmailUnlocked { return a.handleGameKey("m") }
		case game.TabStore:
			if y == l.StoreCapacityY { return a.handleGameKey("1") }
			if y == l.StoreCalibrationY { return a.handleGameKey("2") }
			if y == l.StoreAutomationY { return a.handleGameKey("3") }
		case game.TabMessages:
			if y >= l.MessageFirstY && y <= l.MessageLastY {
				idx := y-l.MessageFirstY
				if idx >= 0 && idx < min(len(a.state.Messages),5) {
					_ = a.state.SelectMessage(a.state.Messages[idx].ID, time.Now())
					return nil
				}
			}
			if l.MessageChoiceFirstY > 0 && y >= l.MessageChoiceFirstY && y <= l.MessageChoiceFirstY+3 {
				switch y-l.MessageChoiceFirstY {
				case 0: return a.handleGameKey("1")
				case 1: return a.handleGameKey("2")
				case 2: return a.handleGameKey("3")
				case 3: return a.handleGameKey("i")
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
		if err := a.state.QueueExploit(now); err != nil { a.message = err.Error() } else { a.message = "Instruction queued. The batch continues." }
	case "1":
		if a.state.CurrentTab == game.TabStore {
			if err := a.state.BuyCapacityUpgrade(now); err != nil { a.message = err.Error() } else { a.message = "Cash box installed." }
		} else if a.state.CurrentTab == game.TabMessages {
			if err := a.state.ReplyToSelected(0, now); err != nil { a.message = err.Error() } else { a.message = "Reply sent. The inbox is quiet for now." }
		}
	case "2":
		if a.state.CurrentTab == game.TabStore {
			if err := a.state.BuyWindowCalibration(now); err != nil { a.message = err.Error() } else { a.message = "Queue indicator calibration installed." }
		} else if a.state.CurrentTab == game.TabMessages {
			if err := a.state.ReplyToSelected(1, now); err != nil { a.message = err.Error() } else { a.message = "Reply sent. The inbox is quiet for now." }
		}
	case "3":
		if a.state.CurrentTab == game.TabStore {
			if err := a.state.BuyAutomation(now); err != nil { a.message = err.Error() } else { a.message = "Exploit automation enabled." }
		} else if a.state.CurrentTab == game.TabMessages {
			if err := a.state.ReplyToSelected(2, now); err != nil { a.message = err.Error() } else { a.message = "Reply sent. The inbox is quiet for now." }
		}
	case "s": _ = a.state.OpenTab(game.TabStore); a.message = ""
	case "m", "e": if err := a.state.OpenTab(game.TabMessages); err != nil { a.message = err.Error() } else { a.message = "" }
	case "i":
		if err := a.state.IgnoreSelectedMessage(now); err != nil { a.message = err.Error() } else { a.message = "You leave the message unanswered." }
	case "b", "esc": _ = a.state.OpenTab(game.TabPrint); a.message = ""
	case "v":
		if err := save.Write(savePath, a.state); err != nil { a.message = "Save failed: "+err.Error() } else { a.message = "Saved to "+savePath }
	case "h", "?": a.message = "X/P exploit  •  S store  •  M messages  •  1–3 reply/upgrade  •  I ignore  •  V save  •  Q quit"
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

var (
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

func (a *App) gameView() string {
	l := a.layout()
	var b strings.Builder
	header := titleStyle.Render("DEEP STATE CLICKER")+"  "+labelStyle.Render("FEDERAL RESERVE / INTERNAL TERMINAL")
	b.WriteString(header+"\n")
	b.WriteString(lineStyle.Render(strings.Repeat("─", max(20,l.Width-4)))+"\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("RUN: %-8s  UPTIME: %04ds  EXPOSURE: %02d/100", strings.ToUpper(string(a.state.Status)), a.state.Tick, a.state.Risk))+"\n")
	b.WriteString("\n")
	cashPanel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(muted).Padding(0,2).Width(max(20,min(l.Width-6,60)))
	cash := labelStyle.Render("CASH ON HAND")+"  "+lipgloss.NewStyle().Foreground(amber).Bold(true).Render("$"+money(a.state.CashOnHand))+
		labelStyle.Render(" / $"+money(a.state.CashCapacity))+"    "+labelStyle.Render("TOTAL PRINTED ")+lipgloss.NewStyle().Foreground(ink).Render("$"+money(a.state.TotalPrinted))+
		"\n"+labelStyle.Render("EST. NET WORTH  ")+lipgloss.NewStyle().Foreground(ink).Bold(true).Render("$"+money(a.state.EstimatedNetWorth))
	b.WriteString(cashPanel.Render(cash)+"\n")
	tabs := []game.Tab{game.TabPrint,game.TabStore}
	if a.state.EmailUnlocked { tabs = append(tabs,game.TabMessages) }
	var labels []string
	for _, tab := range tabs {
		label := string(tab)
		if tab == game.TabMessages && unreadCount(a.state.Messages)>0 { label += fmt.Sprintf(" !%d",unreadCount(a.state.Messages)) }
		if tab == a.state.CurrentTab { labels=append(labels,selectedStyle.Render("["+label+"]")) } else { labels=append(labels,labelStyle.Render(" "+label+" ")) }
	}
	b.WriteString("\n"+strings.Join(labels,"     ")+"\n")
	b.WriteString(lineStyle.Render(strings.Repeat("─",max(20,l.Width-4)))+"\n")

	var panel string
	switch a.state.CurrentTab {
	case game.TabPrint: panel = a.printPanel(l)
	case game.TabStore: panel = a.storePanel(l)
	case game.TabMessages: panel = a.messagesPanel(l)
	}
	b.WriteString(panel+"\n")
	if a.message != "" { b.WriteString(subtitleStyle.Render(a.message)+"\n") }
	ticker := tickerText(a.state, max(12,l.Width-4))
	b.WriteString("\n"+lipgloss.NewStyle().Foreground(lipgloss.Color("#B8A46E")).Width(max(12,l.Width-4)).MaxHeight(1).Render(ticker)+"\n")
	b.WriteString(labelStyle.Render("X/P exploit  S store  M messages  1–3 choose  I ignore  V save  H help  Q quit"))
	return lipgloss.NewStyle().Padding(1,2).Render(b.String())
}

func (a *App) printPanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("PRINT OPERATIONS")+"   "+labelStyle.Render(fmt.Sprintf("BATCH %05d",a.state.Batch.Number))+"\n")
	b.WriteString(labelStyle.Render("QUEUE STATUS  "))
	if a.state.Batch.WindowOpen { b.WriteString(okStyle.Render("● OPPORTUNITY")) } else { b.WriteString(labelStyle.Render("● ROUTINE CYCLE")) }
	b.WriteString("\n")
	width := max(10,min(l.Width-12,54))
	progress := a.state.Batch.Progress*width/game.BatchCycleTicks
	if progress < 0 { progress=0 }; if progress > width { progress=width }
	bar := strings.Repeat("━",progress)+strings.Repeat("─",width-progress)
	marker := "▶"
	if a.state.Batch.WindowOpen { marker = okStyle.Render("◆") }
	if progress < width { runes := []rune(bar); runes[progress] = []rune(marker)[0]; bar = string(runes) }
	b.WriteString("  "+lineStyle.Render(bar)+"\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("  SPOOLING  %02d%%",a.state.Batch.Progress*100/game.BatchCycleTicks))+"\n\n")
	if a.state.Batch.WindowOpen {
		b.WriteString("  "+okStyle.Render("[X]  INJECT")+"   "+labelStyle.Render(fmt.Sprintf("QUEUED: %d",a.state.Batch.QueuedDollars))+"\n")
	} else {
		b.WriteString("  "+labelStyle.Render("[X]  INJECT")+"   "+labelStyle.Render("—") +"\n")
	}
	b.WriteString(labelStyle.Render("  The queue continues whether you act or not.")+"\n")
	b.WriteString("\n  "+labelStyle.Render(a.state.Batch.LastOutcome)+"\n")
	b.WriteString("\n  "+selectedStyle.Render("[S] Storage & upgrades"))
	if a.state.EmailUnlocked { b.WriteString("\n  "+selectedStyle.Render("[M] Open messages")) }
	return b.String()
}

func (a *App) storePanel(l uiLayout) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("STORAGE & MACHINE UPGRADES")+"\n\n")
	if a.state.Upgrades.ExpandedCashCapacity { b.WriteString(okStyle.Render("[1] LOCKABLE CASH BOX — INSTALLED")+"\n") } else {
		b.WriteString(fmt.Sprintf("  %s Lockable cash box — $%s\n",selectedStyle.Render("[1]"),money(game.CapacityUpgradeCost)))
		b.WriteString(labelStyle.Render("      Expands cash-on-hand capacity to $2,500.")+"\n")
	}
	if a.state.Upgrades.WindowCalibration { b.WriteString(okStyle.Render("[2] QUEUE INDICATOR CALIBRATION — INSTALLED")+"\n") } else {
		b.WriteString(fmt.Sprintf("  %s Queue indicator calibration — $15.00\n",selectedStyle.Render("[2]")))
		b.WriteString(labelStyle.Render("      Makes the brief opportunity easier to catch.")+"\n")
	}
	if a.state.Upgrades.PrintAutomation { b.WriteString(okStyle.Render("[3] QUIET EXPLOIT AUTOMATION — INSTALLED")+"\n") } else {
		b.WriteString(fmt.Sprintf("  %s Quiet exploit automation — $%s\n",selectedStyle.Render("[3]"),money(game.AutomationCost)))
		b.WriteString(labelStyle.Render("      Queues an instruction during an opportunity.")+"\n")
	}
	b.WriteString("\n"+labelStyle.Render("Money must be in cash on hand to buy an upgrade."))
	return b.String()
}

func (a *App) messagesPanel(l uiLayout) string {
	if len(a.state.Messages) == 0 {
		return titleStyle.Render("MESSAGES")+"   "+labelStyle.Render("INTERNAL MAIL")+"\n\n"+labelStyle.Render("No messages.")
	}
	selected := a.state.SelectedMessage()
	if selected == nil { return titleStyle.Render("MESSAGES")+"\n" }
	if l.SideBySide {
		leftWidth := max(28, min(36, l.Width/3))
		rightWidth := max(40, l.Width-leftWidth-8)
		var left, right strings.Builder
		left.WriteString(titleStyle.Render("INBOX")+"\n")
		visible := min(len(a.state.Messages), 5)
		for i:=0; i<visible; i++ {
			m:=a.state.Messages[i]
			cursor:=" "; if m.ID==a.state.SelectedMessageID { cursor="›" }
			status:=" "; if !m.Read { status="●" }
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
		return lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(leftWidth).PaddingRight(1).Render(left.String()),
			lipgloss.NewStyle().Width(rightWidth).PaddingLeft(1).Render(right.String()))
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("MESSAGES")+"   "+labelStyle.Render("INTERNAL MAIL")+"\n")
	visible:=min(len(a.state.Messages),5)
	for i:=0;i<visible;i++ {
		m:=a.state.Messages[i]
		cursor:=" "; if m.ID==a.state.SelectedMessageID { cursor="›" }
		status:=" "; if !m.Read { status="●" }
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

func unreadCount(messages []game.Message) int { n:=0; for _,m:=range messages { if !m.Read { n++ } }; return n }
func tickerText(s game.State,width int) string {
	if len(game.NewsHeadlines)==0 { return "" }
	headline:=game.NewsHeadlines[min(s.NewsIndex,len(game.NewsHeadlines)-1)]
	source:="  WORLD BULLETIN  /  "
	full:=source+headline+"     •     "+headline
	if len(full)<=width { return full }
	offset:=int(s.Tick/2)%max(1,len(full))
	loop:=full+ "     "+full
	return truncate(loop[offset:],width)
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
