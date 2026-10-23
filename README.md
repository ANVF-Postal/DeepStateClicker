# Deep State Clicker — Prototype 0.1

A small, dependency-free Go terminal prototype. The initial build deliberately uses the Go standard library only so it can compile and be tested in a restricted/offline environment. The simulation and persistence layers are independent of the terminal presentation, so a current Bubble Tea v2 interface can be introduced later without rewriting game rules.

## Requirements

- Go 1.23 or newer
- A terminal with a UTF-8-capable font (optional; the prototype is mostly plain text)

## Run

```sh
go run ./cmd/deepstate
```

## Test

```sh
go test ./...
go vet ./...
```

## Controls

- `p` — print $1
- `s` — open Store
- `e` — open Email when unlocked
- `b` — back to the main screen
- `h` — help
- `save` — save the current run
- `q` — quit (the game autosaves)

The world simulation advances only while the process is running. There is no offline progression.

## Architecture

- `internal/game`: deterministic game state and rules; no terminal or disk dependencies
- `internal/save`: versioned JSON save envelope and atomic save writes
- `internal/terminal`: input loop and text rendering only
- `cmd/deepstate`: application wiring

See `docs/PROTOTYPE.md` for the current scope and deliberate non-goals.
