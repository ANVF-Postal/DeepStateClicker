package save

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"deepstateclicker/internal/game"
)

const CurrentVersion = 1

type envelope struct {
	FormatVersion int        `json:"format_version"`
	State         game.State `json:"state"`
}

func Write(path string, state game.State) error {
	if path == "" {
		return errors.New("save path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create save directory: %w", err)
	}
	data, err := json.MarshalIndent(envelope{FormatVersion: CurrentVersion, State: state}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode save: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temporary save: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit save: %w", err)
	}
	return nil
}

func Read(path string) (game.State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return game.State{}, fmt.Errorf("read save: %w", err)
	}
	var e envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return game.State{}, fmt.Errorf("decode save: %w", err)
	}
	if e.FormatVersion != CurrentVersion {
		return game.State{}, fmt.Errorf("unsupported save format %d (supported: %d)", e.FormatVersion, CurrentVersion)
	}
	if e.State.SchemaVersion != 1 {
		return game.State{}, fmt.Errorf("unsupported state schema %d", e.State.SchemaVersion)
	}
	if e.State.CashOnHand < 0 || e.State.CashCapacity < 0 || e.State.TotalPrinted < 0 {
		return game.State{}, errors.New("save contains invalid negative currency")
	}
	return e.State, nil
}
