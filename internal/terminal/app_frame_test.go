package terminal

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

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
