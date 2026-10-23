package game

import (
	"errors"
	"fmt"
	"time"
)

const (
	InitialCashCapacity int64 = 100_000 // $1,000.00
	PrintAmount         int64 = 100     // $1.00
	CapacityUpgradeCost int64 = 2_500   // $25.00
	CapacityUpgradeTo   int64 = 250_000 // $2,500.00
	AutomationCost      int64 = 5_000   // $50.00
)

var (
	ErrCapacityFull = errors.New("cash-on-hand capacity reached")
	ErrInsufficient = errors.New("not enough cash on hand")
	ErrUpgradeOwned = errors.New("upgrade already purchased")
	ErrNoEmail      = errors.New("no new email")
)

type RunStatus string

const (
	RunFederal RunStatus = "federal"
	RunSandbox RunStatus = "sandbox"
	RunDead    RunStatus = "dead"
	RunRetired RunStatus = "retired"
)

type Tab string

const (
	TabPrint Tab = "PRINT"
	TabStore Tab = "STORE"
	TabEmail Tab = "EMAIL"
)

type Event struct {
	AtTick int64     `json:"at_tick"`
	Kind   string    `json:"kind"`
	Text   string    `json:"text"`
	At     time.Time `json:"at,omitempty"`
}

type UpgradeState struct {
	ExpandedCashCapacity bool `json:"expanded_cash_capacity"`
	PrintAutomation      bool `json:"print_automation"`
}

type State struct {
	SchemaVersion int          `json:"schema_version"`
	RunID         string       `json:"run_id"`
	Status        RunStatus    `json:"status"`
	CurrentTab    Tab          `json:"current_tab"`
	CashOnHand    int64        `json:"cash_on_hand_cents"`
	CashCapacity  int64        `json:"cash_capacity_cents"`
	TotalPrinted  int64        `json:"total_printed_cents"`
	Risk          int          `json:"risk"`
	Tick          int64        `json:"tick"`
	EmailUnlocked bool         `json:"email_unlocked"`
	EmailRead     bool         `json:"email_read"`
	Upgrades      UpgradeState `json:"upgrades"`
	Events        []Event      `json:"events"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

func NewState(now time.Time) State {
	return State{
		SchemaVersion: 1,
		RunID:         fmt.Sprintf("DSC-%d", now.UnixNano()),
		Status:        RunFederal,
		CurrentTab:    TabPrint,
		CashCapacity:  InitialCashCapacity,
		CreatedAt:     now,
		UpdatedAt:     now,
		Events: []Event{{
			AtTick: 0,
			Kind:   "discovery",
			Text:   "The count is off by one dollar. You check it again. The number does not change.",
			At:     now,
		}},
	}
}

func (s *State) PrintOneDollar(now time.Time) error {
	if s.Status != RunFederal && s.Status != RunSandbox {
		return errors.New("this run is no longer active")
	}
	if s.CashOnHand+PrintAmount > s.CashCapacity {
		s.Risk = clamp(s.Risk+1, 0, 100)
		s.addEvent("overflow_attempt", "The drawer is full. You leave the print command alone.", now)
		s.UpdatedAt = now
		return ErrCapacityFull
	}
	s.CashOnHand += PrintAmount
	s.TotalPrinted += PrintAmount
	if s.TotalPrinted >= 1_000 && !s.EmailUnlocked {
		s.EmailUnlocked = true
		s.addEvent("email_unlocked", "A new message has arrived in your internal inbox.", now)
	}
	// The first few dollars are quiet. Accumulation slowly makes the anomaly harder to ignore.
	if s.TotalPrinted >= 5_000 && s.TotalPrinted%2_500 == 0 {
		s.Risk = clamp(s.Risk+1, 0, 100)
	}
	s.UpdatedAt = now
	return nil
}

func (s *State) BuyCapacityUpgrade(now time.Time) error {
	if s.Upgrades.ExpandedCashCapacity {
		return ErrUpgradeOwned
	}
	if s.CashOnHand < CapacityUpgradeCost {
		return ErrInsufficient
	}
	s.CashOnHand -= CapacityUpgradeCost
	s.CashCapacity = CapacityUpgradeTo
	s.Upgrades.ExpandedCashCapacity = true
	s.addEvent("upgrade", "You replace the shallow drawer with a lockable cash box.", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) BuyAutomation(now time.Time) error {
	if s.Upgrades.PrintAutomation {
		return ErrUpgradeOwned
	}
	if s.CashOnHand < AutomationCost {
		return ErrInsufficient
	}
	s.CashOnHand -= AutomationCost
	s.Upgrades.PrintAutomation = true
	s.addEvent("upgrade", "A quiet modification lets the machine run without your hand on the button.", now)
	s.UpdatedAt = now
	return nil
}

// Advance advances one simulation second. It is called only by the active app loop.
func (s *State) Advance(now time.Time) {
	if s.Status != RunFederal && s.Status != RunSandbox {
		return
	}
	s.Tick++
	if s.Upgrades.PrintAutomation && s.Tick%2 == 0 && s.CashOnHand+PrintAmount <= s.CashCapacity {
		s.CashOnHand += PrintAmount
		s.TotalPrinted += PrintAmount
	}
	if s.Tick%30 == 0 {
		s.Risk = clamp(s.Risk+1, 0, 100)
	}
	s.UpdatedAt = now
}

func (s *State) OpenTab(tab Tab) error {
	if tab == TabEmail && !s.EmailUnlocked {
		return errors.New("EMAIL is not available yet")
	}
	if tab != TabPrint && tab != TabStore && tab != TabEmail {
		return errors.New("unknown tab")
	}
	s.CurrentTab = tab
	return nil
}

func (s *State) ReadEmail(now time.Time) error {
	if !s.EmailUnlocked || s.EmailRead {
		return ErrNoEmail
	}
	s.EmailRead = true
	s.addEvent("email_read", "IT OPERATIONS — Unusual print reconciliation. Can you come by before end of day?", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) addEvent(kind, text string, now time.Time) {
	s.Events = append(s.Events, Event{AtTick: s.Tick, Kind: kind, Text: text, At: now})
}

func clamp(n, min, max int) int {
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
