package game

import (
	"errors"
	"testing"
	"time"
)

func TestPrintRespectsCapacity(t *testing.T) {
	s := NewState(time.Unix(1, 0))
	s.CashOnHand = s.CashCapacity
	if err := s.PrintOneDollar(time.Unix(2, 0)); !errors.Is(err, ErrCapacityFull) {
		t.Fatalf("expected capacity error, got %v", err)
	}
	if s.CashOnHand != InitialCashCapacity {
		t.Fatalf("cash changed on failed print: %d", s.CashOnHand)
	}
	if s.Risk != 1 {
		t.Fatalf("overflow should increase risk, got %d", s.Risk)
	}
}

func TestEmailUnlocksAfterTenDollarsPrinted(t *testing.T) {
	s := NewState(time.Unix(1, 0))
	for i := 0; i < 10; i++ {
		if err := s.PrintOneDollar(time.Unix(int64(i+2), 0)); err != nil {
			t.Fatal(err)
		}
	}
	if !s.EmailUnlocked {
		t.Fatal("email should unlock after $10 printed")
	}
	if err := s.ReadEmail(time.Unix(20, 0)); err != nil {
		t.Fatalf("read email: %v", err)
	}
	if !s.EmailRead {
		t.Fatal("email should be marked read")
	}
}

func TestUpgradesSpendCashAndChangeCapacity(t *testing.T) {
	s := NewState(time.Unix(1, 0))
	s.CashOnHand = 10_000
	if err := s.BuyCapacityUpgrade(time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	if s.CashOnHand != 7_500 {
		t.Fatalf("expected 7500 cents, got %d", s.CashOnHand)
	}
	if s.CashCapacity != CapacityUpgradeTo {
		t.Fatalf("unexpected capacity %d", s.CashCapacity)
	}
	if err := s.BuyCapacityUpgrade(time.Unix(3, 0)); !errors.Is(err, ErrUpgradeOwned) {
		t.Fatalf("expected already owned, got %v", err)
	}
}

func TestAutomationAdvancesOnlyWhenActive(t *testing.T) {
	s := NewState(time.Unix(1, 0))
	s.CashOnHand = 10_000
	if err := s.BuyAutomation(time.Unix(2, 0)); err != nil {
		t.Fatal(err)
	}
	before := s.CashOnHand
	s.Advance(time.Unix(3, 0))
	if s.CashOnHand != before {
		t.Fatal("automation should not print on odd tick")
	}
	s.Advance(time.Unix(4, 0))
	if s.CashOnHand != before+PrintAmount {
		t.Fatal("automation should print on even tick")
	}
	s.Status = RunDead
	cash := s.CashOnHand
	s.Advance(time.Unix(5, 0))
	if s.CashOnHand != cash {
		t.Fatal("dead run must not advance")
	}
}
