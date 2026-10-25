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
