package game

import (
	"errors"
	"testing"
	"time"
)

func TestExploitOnlyQueuesDuringWindowAndCashArrivesAtBatchCompletion(t *testing.T) {
	now := time.Unix(1, 0)
	s := NewState(now)
	if err := s.QueueExploit(now); !errors.Is(err, ErrWindowClosed) {
		t.Fatalf("expected closed window, got %v", err)
	}
	for i := 0; i < ExploitWindowStart; i++ { s.Advance(now.Add(time.Duration(i+1)*time.Second)) }
	if !s.Batch.WindowOpen { t.Fatal("expected exploit window to be open") }
	before := s.CashOnHand
	if err := s.QueueExploit(now); err != nil { t.Fatalf("queue exploit: %v", err) }
	if s.CashOnHand != before { t.Fatal("cash must not arrive when instruction is queued") }
	for s.Batch.Number == 1 { s.Advance(now.Add(time.Duration(s.Tick+1)*time.Second)) }
	if s.CashOnHand != before+PrintAmount { t.Fatalf("expected cash after batch completion, got %d",s.CashOnHand) }
	if s.TotalPrinted != PrintAmount { t.Fatalf("expected total printed to increase at completion, got %d",s.TotalPrinted) }
}

func TestMissedWindowDoesNotQueueMoney(t *testing.T) {
	s := NewState(time.Unix(1,0))
	for i:=0;i<BatchCycleTicks;i++ { s.Advance(time.Unix(int64(i+2),0)) }
	if s.CashOnHand != 0 || s.TotalPrinted != 0 { t.Fatalf("missed window should yield no cash: cash=%d total=%d",s.CashOnHand,s.TotalPrinted) }
	if s.Batch.Number != 2 { t.Fatalf("expected next batch, got %d",s.Batch.Number) }
}

func TestCapacityUpgradeSpendsCashAndChangesCapacity(t *testing.T) {
	s:=NewState(time.Unix(1,0)); s.CashOnHand=10_000
	if err:=s.BuyCapacityUpgrade(time.Unix(2,0));err!=nil { t.Fatal(err) }
	if s.CashOnHand!=7_500 { t.Fatalf("expected 7500 cents, got %d",s.CashOnHand) }
	if s.CashCapacity!=CapacityUpgradeTo { t.Fatalf("unexpected capacity %d",s.CashCapacity) }
	if err:=s.BuyCapacityUpgrade(time.Unix(3,0));!errors.Is(err,ErrUpgradeOwned) { t.Fatalf("expected already owned, got %v",err) }
}

func TestAutomationQueuesButDoesNotInstantlyPay(t *testing.T) {
	s:=NewState(time.Unix(1,0)); s.CashOnHand=10_000
	if err:=s.BuyAutomation(time.Unix(2,0));err!=nil { t.Fatal(err) }
	before:=s.CashOnHand
	for i:=0;i<ExploitWindowStart;i++ { s.Advance(time.Unix(int64(i+3),0)) }
	if s.Batch.QueuedDollars!=1 { t.Fatalf("automation should queue one instruction, got %d",s.Batch.QueuedDollars) }
	if s.CashOnHand!=before { t.Fatal("automation must not pay before batch completion") }
	for s.Batch.Number==1 { s.Advance(time.Unix(int64(s.Tick+10),0)) }
	if s.CashOnHand!=before+PrintAmount { t.Fatalf("expected cash on completion, got %d",s.CashOnHand) }
}

func TestMessageReplyArrivesAfterActiveTicksAndCharacterRemembers(t *testing.T) {
	now:=time.Unix(1,0)
	s:=NewState(now)
	s.EmailUnlocked=true
	s.addITMessage("Print reconciliation discrepancy","One count does not line up.",[]string{"Routine variance.","Let's review records.","I'll get back to you."})
	if err:=s.SelectMessage(s.Messages[0].ID,now);err!=nil { t.Fatal(err) }
	if err:=s.ReplyToSelected(0,now);err!=nil { t.Fatal(err) }
	delay:=s.PendingReplyTicks
	if delay<3 || delay>9 { t.Fatalf("unexpected reply delay %d",delay) }
	if len(s.Messages)!=1 { t.Fatal("reply should not arrive instantly") }
	for i:=0;i<delay-1;i++ { s.Advance(now.Add(time.Duration(i+1)*time.Second)) }
	if len(s.Messages)!=1 { t.Fatal("reply arrived too early") }
	s.Advance(now.Add(time.Duration(delay)*time.Second))
	if len(s.Messages)!=2 { t.Fatalf("expected delayed reply, got %d messages",len(s.Messages)) }
	if s.ITSuspicion!=1 { t.Fatalf("character should remember dismissive reply, suspicion=%d",s.ITSuspicion) }
}

func TestSimulationDoesNotAdvanceAfterRunEnds(t *testing.T) {
	s:=NewState(time.Unix(1,0))
	s.Status=RunDead
	s.Advance(time.Unix(2,0))
	if s.Tick!=0 { t.Fatalf("dead run advanced to tick %d",s.Tick) }
}
