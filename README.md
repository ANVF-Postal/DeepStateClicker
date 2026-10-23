# Deep State Clicker — Prototype 0.2

A terminal-based idle/management game prototype built in Go. The interface now uses Bubble Tea for a full-screen keyboard-driven TUI and Lip Gloss for layout and styling. The simulation and save layers remain separate from presentation.

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

## Controls

**Menu**
- Up/down or `j`/`k` — navigate
- Enter — select
- `n` — new operation
- `l` — load autosave
- `q` or Esc — quit

**In game**
- `p` or Space — print $1
- `s` — open Store
- `e` — open Email when unlocked
- `b` or Esc — return to PRINT
- `1` / `2` — buy the selected Store upgrade
- `v` — save
- `h` or `?` — help
- `q` — save and quit

The app uses the terminal's alternate screen and redraws in place. The simulation advances only while the app is open; there is no offline progression.

## Architecture

- `internal/game`: game state and rules; no terminal or disk dependencies
- `internal/save`: versioned JSON save envelope and atomic save writes
- `internal/terminal`: Bubble Tea model and Lip Gloss views
- `cmd/deepstate`: application wiring

See `docs/PROTOTYPE.md` for the current scope and deliberate non-goals.
