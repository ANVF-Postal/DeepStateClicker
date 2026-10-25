package terminal

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"deepstateclicker/internal/game"
)

func TestViewUsesStableTerminalFrameAcrossPanels(t *testing.T) {
	app := New(game.NewState(time.Now()))
	app.width, app.height = 100, 31

	cases := []struct {
		name string
		set  func()
	}{
		{
			name: "main",
			set: func() {
				app.screen = screenGame
				app.state.CurrentTab = game.TabPrint
				app.mainView = "print"
			},
		},
		{
			name: "upgrades",
			set: func() {
				app.screen = screenGame
				app.state.CurrentTab = game.TabPrint
				app.mainView = "upgrades"
			},
		},
		{
			name: "rack",
			set: func() {
				app.screen = screenGame
				app.state.CurrentTab = game.TabPrint
				app.mainView = "rack"
			},
		},
		{
			name: "storage",
			set: func() {
				app.screen = screenGame
				app.state.StorageUnlocked = true
				app.state.CurrentTab = game.TabStorage
			},
		},
		{
			name: "menu",
			set: func() { app.screen = screenMenu },
		},
	}

	const wantWidth = 99
	const wantHeight = 30
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.set()
			lines := strings.Split(app.View(), "\n")
			if len(lines) != wantHeight {
				t.Fatalf("frame has %d rows; want %d", len(lines), wantHeight)
			}
			for row, line := range lines {
				if got := lipgloss.Width(line); got != wantWidth {
					t.Errorf("row %d has width %d; want %d", row, got, wantWidth)
				}
			}
		})
	}
}


func TestTruncateDoesNotLeakPartialTrueColorSequences(t *testing.T) {
	input := "\x1b[38;2;119;121;112mAVAILABLE\x1b[0m"
	got := truncate(input, 6)
	plain := ansiSequences.ReplaceAllString(got, "")
	if plain != "AVAIL…" {
		t.Fatalf("visible truncated text = %q; want %q", plain, "AVAIL…")
	}
	if strings.Contains(got, ">38;2") || strings.Contains(got, ";121;112m") && !strings.Contains(got, "\x1b[38;2;119;121;112m") {
		t.Fatalf("truncation leaked part of a colour escape sequence: %q", got)
	}
	if width := lipgloss.Width(got); width > 6 {
		t.Fatalf("truncated text is %d cells wide; want <= 6", width)
	}
}

func TestUpgradeCatalogueOmitsAvailableStatusColumn(t *testing.T) {
	app := New(game.NewState(time.Now()))
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.mainView = "upgrades"
	lines := plainViewLines(app.upgradesPanel(app.layout()))
	for i, line := range lines {
		if i == 0 {
			continue // The pane title is MODULE CATALOG, not a status column.
		}
		if strings.Contains(line, " AVAILABLE") || strings.Contains(line, " LOCKED") {
			t.Fatalf("upgrade catalogue still contains an inline status tag: %q", line)
		}
	}
}

func TestUppercaseRackShortcutAndMainRackClick(t *testing.T) {
	app := New(game.NewState(time.Now()))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.state.Modules = append(app.state.Modules, game.ModuleState{
		ID: "queue_calibration", Level: 1, Installed: true,
	})
	app.mainView = "upgrades"

	app.handleGameKey("R")
	if app.mainView != "rack" {
		t.Fatalf("uppercase R left main view at %q; want rack", app.mainView)
	}

	app.mainView = "print"
	lines := plainViewLines(app.View())
	for y, line := range lines {
		x := strings.Index(line, "[R] HARDWARE RACK")
		if x >= 0 {
			app.handleMouse(x, y)
			if app.mainView != "rack" {
				t.Fatalf("clicking rack label left main view at %q; want rack", app.mainView)
			}
			return
		}
	}
	t.Fatal("main view did not render the hardware rack label")
}


func rackLine(t *testing.T, app *App, contains string) (int, string) {
	t.Helper()
	lines := plainViewLines(app.View())
	for y, line := range lines {
		if strings.Contains(line, contains) {
			return y, line
		}
	}
	t.Fatalf("could not find %q in rack view", contains)
	return -1, ""
}

func TestModulePurchaseWaitsInUpgradesUntilRackInstall(t *testing.T) {
	now := time.Unix(1, 0)
	app := New(game.NewState(now))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.state.CashOnHand = 100_000
	app.mainView = "upgrades"
	app.selectedUpgrade = 0

	app.buySelectedModule(now)
	m := app.state.Module("window_expansion")
	if m == nil || m.Level != 1 || m.Installed || m.Booting {
		t.Fatalf("purchase should be owned but not installed or booting: %#v", m)
	}
	if app.mainView != "upgrades" {
		t.Fatalf("purchase changed screen to %q; want upgrades", app.mainView)
	}
	if !app.hasReadyModules() {
		t.Fatal("purchased, uninstalled module should trigger rack-ready state")
	}
}

func TestUpgradeBuyButtonRequiresSecondClick(t *testing.T) {
	app := New(game.NewState(time.Unix(1, 0)))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.state.CashOnHand = 100_000
	app.mainView = "upgrades"
	app.selectedUpgrade = 0

	y, line := rackLine(t, app, "[ENTER / B] BUY / UPGRADE")
	x := strings.Index(line, "[ENTER / B] BUY / UPGRADE")
	app.handleMouse(x, y)
	if app.state.Module("window_expansion") != nil {
		t.Fatal("first click should only arm the purchase")
	}
	y, line = rackLine(t, app, "[CONFIRM] CLICK BUY / UPGRADE AGAIN")
	x = strings.Index(line, "[CONFIRM] CLICK BUY / UPGRADE AGAIN")
	app.handleMouse(x, y)
	m := app.state.Module("window_expansion")
	if m == nil || m.Level != 1 || m.Installed {
		t.Fatalf("confirmed purchase should be ready but uninstalled: %#v", m)
	}
	if app.mainView != "upgrades" {
		t.Fatalf("confirmed purchase changed screen to %q; want upgrades", app.mainView)
	}
}

func TestRackCatalogueSelectThenClickInstallsAndSymbolRemoves(t *testing.T) {
	now := time.Unix(1, 0)
	app := New(game.NewState(now))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.state.CashOnHand = 100_000
	if err := app.state.PurchaseModule("window_expansion", now); err != nil {
		t.Fatal(err)
	}
	app.mainView = "rack"
	app.rackCatalogSelected = false

	headerY, _ := rackLine(t, app, "┌─ MODULES")
	y := headerY + 1
	_, line := rackLine(t, app, "WINDOW EXPANSION")
	nameX := strings.Index(line, "WINDOW EXPANSION")
	app.handleMouse(nameX, y)
	m := app.state.Module("window_expansion")
	if !app.rackCatalogSelected || m.Installed {
		t.Fatalf("first row click should select without installing: selected=%v module=%#v", app.rackCatalogSelected, m)
	}
	if !strings.Contains(plainViewLines(app.View())[y], "[WINDOW EXPANSION]") {
		t.Fatal("selected module name should be enclosed in square brackets")
	}

	// Clicking the selected row a second time installs the module.
	app.handleMouse(nameX, y)
	if !m.Installed || !m.Booting {
		t.Fatalf("second row click should install and start boot: %#v", m)
	}

	// The X at the end of the left row is an explicit one-click remove action.
	app.rackFocusRight = false
	app.rackCatalogSelected = false
	app.handleMouse(app.paneLeftSymbolX(), y)
	if m.Installed {
		t.Fatalf("catalogue X shortcut should remove immediately: %#v", m)
	}
}

func (a *App) paneLeftSymbolX() int {
	leftWidth, _ := paneWidths(a.layout())
	return leftWidth
}

func rackLineWithContext(t *testing.T, app *App, target, context string) (int, string) {
	t.Helper()
	lines := plainViewLines(app.View())
	for y, line := range lines {
		if strings.Contains(line, target) && strings.Contains(line, context) {
			return y, line
		}
	}
	t.Fatalf("could not find target %q with context %q in rack view", target, context)
	return -1, ""
}

func TestRackRightModuleXRemovesModule(t *testing.T) {
	app := New(game.NewState(time.Unix(1, 0)))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.state.CashOnHand = 100_000
	now := time.Unix(1, 0)
	if err := app.state.PurchaseModule("window_expansion", now); err != nil { t.Fatal(err) }
	if err := app.state.InstallModule("window_expansion", now); err != nil { t.Fatal(err) }
	app.mainView = "rack"
	app.rackFocusRight = true
	app.selectRackModule("window_expansion")

	y, line := rackLineWithContext(t, app, "WINDOW EXPANSION", "[X]")
	x := strings.Index(line, "[X]")
	app.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	if m := app.state.Module("window_expansion"); m.Installed {
		t.Fatalf("clicking the actual module X should remove it: %#v (x=%d y=%d line=%q)", m, x, y, line)
	}
}

func TestLegacyRemovalButtonRequiresSecondClick(t *testing.T) {
	app := New(game.NewState(time.Unix(1, 0)))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.state.CashOnHand = 10_000
	app.mainView = "rack"
	// The jammed module is the bottom bay. Scroll the rack viewport to it.
	app.rackRightScroll = len(app.rackBays())-visibleRackBays(max(4,panelLineLimit(app.layout(),false)-2))
	app.rackRightScroll = max(0,app.rackRightScroll)

	y, line := rackLineWithContext(t, app, "$50 - [REMOVE]", "JAMMED - SPECIALIST REMOVAL")
	x := strings.Index(line, "[REMOVE]")
	app.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	if m := app.state.Module("legacy_controller"); m == nil || !m.Installed {
		t.Fatalf("first click must only arm the actual specialist button: %#v (line=%q)", m, line)
	}
	if app.rackRemovalArmedModule != "legacy_controller" {
		t.Fatalf("first click should arm specialist removal, got %q", app.rackRemovalArmedModule)
	}

	y, line = rackLineWithContext(t, app, "$50 - [REMOVE]", "JAMMED - SPECIALIST REMOVAL")
	x = strings.Index(line, "[REMOVE]")
	app.Update(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft, X: x, Y: y})
	if m := app.state.Module("legacy_controller"); m == nil || m.Installed {
		t.Fatalf("second click should confirm removal through Update: %#v (line=%q)", m, line)
	}
	if app.state.CashOnHand != 5_000 {
		t.Fatalf("specialist removal should cost $50, cash=%d", app.state.CashOnHand)
	}
	if app.rackRemovalArmedModule != "" {
		t.Fatalf("confirmation should clear armed removal, got %q", app.rackRemovalArmedModule)
	}
}


func TestRackModuleCardsKeepFixedEnclosureHeight(t *testing.T) {
	inner := 42
	installed := game.ModuleState{ID: "window_expansion", Level: 1, Installed: true}
	booting := installed
	booting.Booting = true
	booting.BootTicksTotal = 10
	booting.BootTicksRemaining = 4
	jammed := game.ModuleState{ID: "legacy_controller", Level: 1, Installed: true, Jammed: true}

	for _, tc := range []struct {
		name string
		module game.ModuleState
		want string
	}{
		{name: "operational", module: installed, want: "WINDOW EXPANSION"},
		{name: "booting", module: booting, want: "WINDOW EXPANSION"},
		{name: "jammed legacy", module: jammed, want: "JAMMED - SPECIALIST REMOVAL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "WINDOW EXPANSION"
			if tc.module.ID == "legacy_controller" {
				name = "LEGACY PRINT CONTROLLER"
			}
			lines := rackModuleCard(tc.module, name, inner, false, 4, 7)
			if len(lines) != rackBayHeight {
				t.Fatalf("module card has %d rows; want %d", len(lines), rackBayHeight)
			}
			if strings.Contains(ansiSequences.ReplaceAllString(lines[0], ""), name) {
				t.Fatalf("module title should be inside the enclosure, not interrupt the top border: %q", lines[0])
			}
			if !strings.Contains(ansiSequences.ReplaceAllString(strings.Join(lines, "\n"), ""), tc.want) {
				t.Fatalf("module card does not show expected text %q: %q", tc.want, lines)
			}
			for row, line := range lines {
				if got := lipgloss.Width(line); got != inner+2 {
					t.Errorf("row %d has width %d; want %d", row, got, inner+2)
				}
			}
			if tc.name == "operational" && !strings.Contains(ansiSequences.ReplaceAllString(lines[4], ""), ".") {
				t.Fatal("operational module should have a status LED in the reserved activity row")
			}
			if tc.name == "jammed legacy" {
				plain := ansiSequences.ReplaceAllString(strings.Join(lines, "\n"), "")
				if strings.Contains(plain, "REV") {
					t.Fatal("jammed legacy module should not display a revision line")
				}
				if !strings.Contains(plain, "$50 - [REMOVE]") {
					t.Fatal("jammed legacy module should display the specialist removal control")
				}
			}
		})
	}
}

func TestRemovingLegacyControllerRevealsAdditionalEmptyBays(t *testing.T) {
	app := New(game.NewState(time.Unix(1, 0)))
	before := app.rackBays()
	if len(before) != 3 {
		t.Fatalf("legacy rack should initially show 3 bays, got %d", len(before))
	}
	app.state.CashOnHand = 10_000
	if err := app.state.RemoveJammedLegacyModule(time.Unix(2, 0)); err != nil {
		t.Fatalf("remove legacy controller: %v", err)
	}
	after := app.rackBays()
	if len(after) != 5 {
		t.Fatalf("removing legacy controller should reveal 2 additional bays, got %d", len(after))
	}
	empty := 0
	for _, bay := range after {
		if bay == nil {
			empty++
		}
	}
	if empty < 4 {
		t.Fatalf("expected newly unlocked empty bays, got %d empty slots", empty)
	}
}

func TestRackScrollbarIsVerticalAtPaneRightEdge(t *testing.T) {
	app := New(game.NewState(time.Unix(1, 0)))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.state.CashOnHand = 100_000
	if err := app.state.PurchaseModule("window_expansion", time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	if err := app.state.InstallModule("window_expansion", time.Unix(3, 0)); err != nil {
		t.Fatal(err)
	}
	app.mainView = "rack"

	lines := plainViewLines(app.View())
	leftWidth, rightWidth := paneWidths(app.layout())
	railX := 2 + leftWidth + 1 + rightWidth - 2
	foundHeader := false
	railRows := 0
	for _, line := range lines {
		if strings.Contains(line, "INSTALLED RACK") {
			foundHeader = true
		}
		if foundHeader && strings.Contains(line, "└") {
			break
		}
		if foundHeader {
			runes := []rune(line)
			if railX >= len(runes) {
				t.Fatalf("right rack pane line is too narrow for scrollbar at x=%d: %q", railX, line)
			}
			if runes[railX] != '█' && runes[railX] != '░' {
				t.Fatalf("scrollbar cell at x=%d should be vertical track/thumb, got %q in %q", railX, string(runes[railX]), line)
			}
			railRows++
		}
	}
	if railRows < 6 {
		t.Fatalf("expected a vertical scrollbar spanning the rack pane, found %d rows", railRows)
	}
}

func TestRemovingLegacyModuleButtonCreatesBlankBayCards(t *testing.T) {
	app := New(game.NewState(time.Unix(1, 0)))
	app.width, app.height = 100, 31
	app.screen = screenGame
	app.state.CurrentTab = game.TabPrint
	app.mainView = "rack"

	lines := plainViewLines(app.View())
	_, rightWidth := paneWidths(app.layout())
	paneInner := rightWidth-2
	contentWidth := paneInner-1
	cardWidth := max(20,min(contentWidth,54))
	emptyTop := "┌"+strings.Repeat("─",cardWidth-2)+"┐"
	foundEmptyBay := false
	for y, line := range lines {
		if !strings.Contains(line, emptyTop) || y+rackBayHeight > len(lines) {
			continue
		}
		cardText := strings.Join(lines[y:y+rackBayHeight], "\n")
		if !strings.Contains(cardText, "LEGACY PRINT CONTROLLER") &&
			!strings.Contains(cardText, "WINDOW EXPANSION") &&
			!strings.Contains(cardText, "QUEUE CALIBRATION") &&
			!strings.Contains(cardText, "AUTOMATIC INJECTOR") {
			foundEmptyBay = true
			break
		}
	}
	if !foundEmptyBay {
		t.Fatal("rack should render at least one empty bay as a full blank enclosure")
	}
}
