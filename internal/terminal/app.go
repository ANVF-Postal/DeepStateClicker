package terminal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"deepstateclicker/internal/game"
	"deepstateclicker/internal/save"
)

const savePath = "saves/autosave.json"

type App struct{ state game.State }

func New(state game.State) *App { return &App{state: state} }

func (a *App) Run() error {
	fmt.Println("DEEP STATE CLICKER")
	fmt.Println("Every dollar leaves a mark.")
	fmt.Println()
	fmt.Println("[N] New operation    [L] Load operation    [Q] Quit")
	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return scanner.Err()
	}
	switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
	case "q", "quit":
		return nil
	case "l", "load":
		loaded, err := save.Read(savePath)
		if err != nil {
			fmt.Printf("Load failed: %v\nStarting a new operation.\n\n", err)
		} else {
			a.state = loaded
		}
	case "n", "new", "":
		a.showIntro(scanner)
		a.state = game.NewState(time.Now())
	default:
		fmt.Println("Unknown choice. Starting a new operation.")
		a.showIntro(scanner)
		a.state = game.NewState(time.Now())
	}
	return a.gameLoop(scanner)
}

func (a *App) showIntro(scanner *bufio.Scanner) {
	fmt.Println()
	fmt.Println("FEDERAL RESERVE — INTERNAL OPERATIONS")
	fmt.Println("You are not an economist. You reconcile print counts, replace toner, and go home.")
	fmt.Println("Then one count comes back wrong.")
	fmt.Println("One dollar. No discrepancy in the ledger. No explanation in the logs.")
	fmt.Println("You run the count again.")
	fmt.Println()
	fmt.Print("Press ENTER to continue...")
	if scanner.Scan() {
		// Consume the pause using the same scanner as the menu and game loop.
	}
}

func (a *App) gameLoop(scanner *bufio.Scanner) error {
	input := make(chan string)
	go func() {
		for scanner.Scan() {
			input <- scanner.Text()
		}
		close(input)
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		a.render()
		fmt.Print("\n> ")
		select {
		case now := <-ticker.C:
			a.state.Advance(now)
		case line, ok := <-input:
			if !ok {
				_ = save.Write(savePath, a.state)
				return nil
			}
			cmd := strings.ToLower(strings.TrimSpace(line))
			now := time.Now()
			switch cmd {
			case "p", "print":
				if err := a.state.PrintOneDollar(now); err != nil {
					fmt.Println("\n", err)
				}
			case "s", "store":
				_ = a.state.OpenTab(game.TabStore)
			case "e", "email":
				if err := a.state.OpenTab(game.TabEmail); err != nil {
					fmt.Println("\n", err)
				} else if err := a.state.ReadEmail(now); err != nil {
					fmt.Println("\n", err)
				}
			case "b", "back":
				_ = a.state.OpenTab(game.TabPrint)
			case "save":
				if err := save.Write(savePath, a.state); err != nil {
					fmt.Println("\nSave failed:", err)
				} else {
					fmt.Println("\nSaved.")
				}
			case "q", "quit":
				if err := save.Write(savePath, a.state); err != nil {
					fmt.Println("Autosave failed:", err)
				}
				return nil
			case "h", "help":
				a.help()
			case "1":
				if a.state.CurrentTab == game.TabStore {
					if err := a.state.BuyCapacityUpgrade(now); err != nil {
						fmt.Println("\n", err)
					}
				}
			case "2":
				if a.state.CurrentTab == game.TabStore {
					if err := a.state.BuyAutomation(now); err != nil {
						fmt.Println("\n", err)
					}
				}
			case "":
			default:
				fmt.Println("\nUnknown command. Press h for help.")
			}
		}
	}
}

func (a *App) render() {
	fmt.Println("\n------------------------------------------------------------")
	fmt.Printf("DEEP STATE CLICKER                         RUN: %s\n", a.state.Status)
	fmt.Printf("TAB: %s    ELAPSED: %ds    RISK: %d/100\n", a.state.CurrentTab, a.state.Tick, a.state.Risk)
	fmt.Printf("CASH ON HAND  $%s / $%s\n", money(a.state.CashOnHand), money(a.state.CashCapacity))
	fmt.Printf("TOTAL PRINTED $%s\n", money(a.state.TotalPrinted))
	switch a.state.CurrentTab {
	case game.TabPrint:
		fmt.Println("\n[PRINT]")
		fmt.Println("  p  PRINT $1")
		fmt.Println("  s  STORE")
		if a.state.EmailUnlocked {
			fmt.Println("  e  EMAIL  * new message")
		} else {
			fmt.Println("  EMAIL  [locked]")
		}
	case game.TabStore:
		fmt.Println("\n[STORE]")
		if a.state.Upgrades.ExpandedCashCapacity {
			fmt.Println("  1  Cash box capacity — PURCHASED")
		} else {
			fmt.Printf("  1  Lockable cash box — $%s (capacity to $2,500)\n", money(game.CapacityUpgradeCost))
		}
		if a.state.Upgrades.PrintAutomation {
			fmt.Println("  2  Quiet print automation — PURCHASED")
		} else {
			fmt.Printf("  2  Quiet print automation — $%s (prints $1 every 2 seconds)\n", money(game.AutomationCost))
		}
		fmt.Println("  b  Back to PRINT")
	case game.TabEmail:
		fmt.Println("\n[EMAIL]")
		if a.state.EmailRead {
			fmt.Println("  IT OPERATIONS — Unusual print reconciliation. Can you come by before end of day?")
		} else {
			fmt.Println("  e  Read new message")
		}
		fmt.Println("  b  Back to PRINT")
	}
	if len(a.state.Events) > 0 {
		e := a.state.Events[len(a.state.Events)-1]
		fmt.Println("\nLATEST RECORD")
		fmt.Printf("  %s\n", e.Text)
	}
	fmt.Println("\nCommands: p print | s store | e email | b back | save | h help | q quit")
}

func (a *App) help() {
	fmt.Println("\nHelp: p prints one dollar; s opens Store; e opens Email when unlocked; b returns to PRINT; 1/2 buy Store upgrades; save writes a save; q autosaves and quits.")
}

func money(cents int64) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }
