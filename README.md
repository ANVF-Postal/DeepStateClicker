# Deep State Clicker — Operations Console Prototype

A terminal-based idle/management game built in Go with Bubble Tea and Lip Gloss. The current prototype centers on a batch-based printing exploit, a formal financial dashboard, modular printer upgrades, an expandable hardware rack, and an early apartment-storage system.

## Requirements

- Go 1.23 or newer
- Internet access on first build so Go can download the TUI dependencies
- A modern terminal with Unicode support recommended

## Run

```sh
go mod tidy
go run ./cmd/deepstate
```

## Test

```sh
go test ./...
go vet ./...
```

## Current loop

- The printer queue runs continuously while the game is open. Each batch takes about one active minute, with the exploit opportunity early in the cycle.
- Press `x`, `p`, Space, or click `INJECT` during the opportunity to queue an extra printing instruction. Cash arrives when the batch completes.
- Cash capacity is not a hard stop. Capital can exceed capacity, but each excess dollar increases exposure. The capital panel's bar is green below 80%, amber from 80%, and signals overflow at 100%.
- MAIN has internal PRINT OPERATIONS, UPGRADES, and HARDWARE RACK views. Purchased printer upgrades are represented by installed modules; revisions improve module effects.
- The rack has no fixed bay limit. It can contain installed modules and an inherited, jammed legacy controller that costs money to remove safely.
- New and reinstalled modules appear in the rack immediately, then run a randomized firmware initialization lasting at most about 13 seconds under normal visible animation. Each boot selects 4–8 unique status messages and a distinct progress curve: some crawl before a major surge, some launch early, and others climb in two blocks before inching toward the finish. Small pauses and movements add texture between checkpoints. Effects remain disabled until the final phase completes, and closing the game pauses boot progress.
- MAIN reports persistent system UPTIME in `HHH:MM:SS`; closing and reopening the game does not reset it.
- STORAGE unlocks as capital approaches 80% of apartment capacity. Lockboxes add capacity, with diminishing space efficiency and clutter exposure after several boxes. Individual container balances and manual transfers are planned for the next storage pass.
- After enough extra cash has been printed, a message arrives from IT Operations. Open MESSAGES to read the inbox and choose a reply. Replies arrive after randomized active game seconds; timers pause when the application is closed.
- The news bulletin flashes, then scrolls each headline across the terminal. Headlines become more serious as the run continues.

Capital on hand is spendable liquidity. Estimated net worth is a separate, speculative figure and cannot be spent directly. The simulation does not progress offline.

## Controls

**Menu**
- Up/down or `j`/`k` — navigate
- Enter — select
- `n` — new operation
- `l` — load autosave
- `q` or Esc — quit

**In game**
- `x`, `p`, or Space — inject during the visible exploit window
- `u` — open UPGRADES; select a module and press Enter or `b` to purchase/upgrade
- `r` — open HARDWARE RACK; use left/right to focus a pane, up/down to scroll, Enter to acquire or remove the selected module
- `s` — return to MAIN / PRINT OPERATIONS
- `t` — open STORAGE when unlocked
- `m` or `e` — open MESSAGES when unlocked
- `1` / `2` / `3` — reply to the selected message
- `i` — leave the selected message unanswered
- Up/down or `j`/`k` — select an inbox message
- `v` — save
- `h` or `?` — help
- `q` — save and quit

Mouse clicks can select menu options, switch tabs, trigger INJECT, select messages and modules, and activate visible purchase controls. Wide terminals show two-pane layouts; narrow terminals use compact layouts. Scrollable rack panes support keyboard navigation.

## Architecture

- `internal/game`: batch lifecycle, module progression, economy, overflow risk, storage capacity, structured mail, character state, news progression, and deterministic state-held random generation
- `internal/save`: versioned JSON save envelope and atomic save writes
- `internal/terminal`: Bubble Tea input loop, responsive layout, mouse hit regions, and Lip Gloss presentation
- `cmd/deepstate`: application wiring

See `docs/PROTOTYPE.md` for the current design contract and deliberate non-goals.
