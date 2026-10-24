# Deep State Clicker — Prototype 0.3

A terminal-based idle/management game prototype built in Go with Bubble Tea and Lip Gloss. The current prototype centers on a batch-based printing exploit, a responsive internal terminal, and the first structured email conversation.

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

- The printer queue runs continuously while the game is open. Each batch currently takes about one active minute, with the exploit opportunity early in the cycle.
- A small visual opportunity appears during each batch. Press `x`, `p`, Space, or click the illuminated `INJECT` control to queue an extra dollar instruction.
- The instruction does not pay immediately. Cash is added only when that batch completes.
- Miss the window and the ordinary batch completes without an extra instruction.
- The Store contains cash capacity, queue calibration, and exploit automation upgrades.
- After enough extra cash has been printed, a message arrives from IT Operations. Open MESSAGES to read the inbox and choose a reply. Replies arrive after a randomized number of active game seconds; the timer pauses when the application is closed.
- The world bulletin scrolls along the bottom and its headlines grow more serious as the run continues.

Cash on hand is spendable liquidity. Estimated net worth is a separate, speculative figure and cannot be spent directly. The simulation does not progress offline.

## Controls

**Menu**
- Up/down or `j`/`k` — navigate
- Enter — select
- `n` — new operation
- `l` — load autosave
- `q` or Esc — quit

**In game**
- `x`, `p`, or Space — inject during the visible exploit window
- `s` — open Store
- `m` or `e` — open Messages when unlocked
- `1` / `2` / `3` — buy the selected Store upgrade or choose a message reply
- `i` — leave the selected message unanswered
- Up/down or `j`/`k` — select an inbox message
- `b` or Esc — return to PRINT
- `v` — save
- `h` or `?` — help
- `q` — save and quit

Mouse clicks can select menu options, switch tabs, trigger the visible exploit control, select messages, and choose visible reply actions. Layout and hit regions share the same calculated terminal geometry; wide terminals show an inbox and reading pane side by side, while narrower terminals stack them.

## Architecture

- `internal/game`: batch lifecycle, exploit window, economy, structured mail, character state, news progression, and deterministic state-held random number generation
- `internal/save`: versioned JSON save envelope and atomic save writes
- `internal/terminal`: Bubble Tea input loop, responsive layout, mouse hit regions, and Lip Gloss presentation
- `cmd/deepstate`: application wiring

See `docs/PROTOTYPE.md` for the current design contract and deliberate non-goals.
