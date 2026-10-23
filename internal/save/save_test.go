package save

import (
	"path/filepath"
	"testing"
	"time"

	"deepstateclicker/internal/game"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saves", "save.json")
	original := game.NewState(time.Unix(123, 0))
	original.CashOnHand = 12345
	if err := Write(path, original); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.CashOnHand != original.CashOnHand {
		t.Fatalf("cash mismatch: got %d", got.CashOnHand)
	}
	if got.Status != game.RunFederal {
		t.Fatalf("status mismatch: got %q", got.Status)
	}
}
