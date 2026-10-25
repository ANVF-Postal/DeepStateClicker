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

func TestDiscrepancyMessageWaitsForThresholdAndRandomActiveDelay(t *testing.T) {
	now := time.Unix(1, 0)
	s := NewState(now)
	s.TotalPrinted = 900
	for i := 0; i < 20; i++ {
		s.Advance(now.Add(time.Duration(i+1) * time.Second))
	}
	if s.EmailUnlocked || s.PendingFirstMessageTicks != 0 {
		t.Fatal("message should not be scheduled before the discrepancy threshold")
	}

	s.TotalPrinted = 1_000
	s.Advance(now.Add(21 * time.Second))
	if s.EmailUnlocked {
		t.Fatal("message should not arrive immediately when the threshold is crossed")
	}
	if s.PendingFirstMessageTicks < 11 || s.PendingFirstMessageTicks > 44 {
		t.Fatalf("expected randomized active-time countdown, got %d", s.PendingFirstMessageTicks)
	}
	initialDelay := s.PendingFirstMessageTicks
	for i := 0; i < initialDelay-1; i++ {
		s.Advance(now.Add(time.Duration(22+i) * time.Second))
		if s.EmailUnlocked {
			t.Fatal("message arrived before its countdown completed")
		}
	}
	s.Advance(now.Add(time.Duration(22+initialDelay) * time.Second))
	if !s.EmailUnlocked || len(s.Messages) != 1 {
		t.Fatalf("expected discrepancy message after countdown, unlocked=%v messages=%d", s.EmailUnlocked, len(s.Messages))
	}
	if s.Batch.Progress == 0 {
		t.Fatal("message timing should not require a print-cycle boundary")
	}
}

func TestSimulationDoesNotAdvanceAfterRunEnds(t *testing.T) {
	s:=NewState(time.Unix(1,0))
	s.Status=RunDead
	s.Advance(time.Unix(2,0))
	if s.Tick!=0 { t.Fatalf("dead run advanced to tick %d",s.Tick) }
}


func TestNetWorthOnlyCountsCashUntilAssetsExist(t *testing.T) {
	now := time.Unix(1, 0)
	s := NewState(now)
	s.CashOnHand = 725
	s.TotalPrinted = 10_000
	s.Advance(now.Add(time.Second))
	if s.EstimatedNetWorth != s.CashOnHand {
		t.Fatalf("without assets, net worth should equal cash on hand: worth=%d cash=%d", s.EstimatedNetWorth, s.CashOnHand)
	}
}

func TestWindowCalibrationAddsOnlyOneTick(t *testing.T) {
	now := time.Unix(1, 0)
	s := NewState(now)
	s.Upgrades.WindowCalibration = true
	for i := 0; i < ExploitWindowStart; i++ {
		s.Advance(now.Add(time.Duration(i+1) * time.Second))
	}
	if !s.Batch.WindowOpen {
		t.Fatal("calibrated opportunity should still open at the normal start")
	}
	s.Advance(now.Add(time.Duration(ExploitWindowStart+1) * time.Second))
	if !s.Batch.WindowOpen {
		t.Fatal("calibration should extend the opportunity by one tick")
	}
	s.Advance(now.Add(time.Duration(ExploitWindowStart+2) * time.Second))
	if !s.Batch.WindowOpen {
		t.Fatal("the normal final tick should remain open")
	}
	s.Advance(now.Add(time.Duration(ExploitWindowStart+3) * time.Second))
	if !s.Batch.WindowOpen {
		t.Fatal("the single calibrated extra tick should remain open")
	}
	s.Advance(now.Add(time.Duration(ExploitWindowStart+4) * time.Second))
	if s.Batch.WindowOpen {
		t.Fatal("calibration should close after exactly one extra tick")
	}
}


func TestOverflowDoesNotBlockInjectionAndRaisesRisk(t *testing.T) {
	now := time.Unix(1, 0)
	s := NewState(now)
	s.CashOnHand = s.CashCapacity
	s.Batch.Progress = BatchCycleTicks - 1
	s.Batch.WindowOpen = true
	if err := s.QueueExploit(now); err != nil {
		t.Fatalf("overflow should not block an otherwise valid injection: %v", err)
	}
	s.Advance(now.Add(time.Second))
	if s.CashOnHand != s.CashCapacity+PrintAmount {
		t.Fatalf("expected cash to exceed capacity, got cash=%d capacity=%d", s.CashOnHand, s.CashCapacity)
	}
	if s.Risk < 1 {
		t.Fatalf("overflow should increase risk, got %d", s.Risk)
	}
	if !s.StorageUnlocked {
		t.Fatal("storage should unlock when capital reaches capacity")
	}
}

func TestLockboxesIncreaseCapacityAndClutterReducesEfficiency(t *testing.T) {
	s := NewState(time.Unix(1, 0))
	s.CashOnHand = 100_000
	initialCapacity := s.CashCapacity
	for i := 0; i < 3; i++ {
		if err := s.BuyLockbox(time.Unix(int64(i+2), 0)); err != nil { t.Fatal(err) }
	}
	if s.CashCapacity != initialCapacity+3*LockboxCapacity {
		t.Fatalf("first three lockboxes should provide full capacity, got %d", s.CashCapacity)
	}
	if s.Risk != 0 { t.Fatalf("ordinary lockboxes should not yet add clutter risk, got %d", s.Risk) }
	if err := s.BuyLockbox(time.Unix(5, 0)); err != nil { t.Fatal(err) }
	if s.CashCapacity != initialCapacity+3*LockboxCapacity+350_000 {
		t.Fatalf("additional lockbox should have diminished capacity, got %d", s.CashCapacity)
	}
	if s.Risk != 1 { t.Fatalf("clutter should add risk, got %d", s.Risk) }
}

func TestRemovedModuleCanBeReinstalledWithoutLosingRevision(t *testing.T) {
	now := time.Unix(1, 0)
	s := NewState(now)
	s.CashOnHand = 10_000
	if err := s.BuyOrUpgradeModule("window_expansion", now); err != nil { t.Fatal(err) }
	if err := s.RemoveModule("window_expansion", now); err != nil { t.Fatal(err) }
	cashAfterPurchase := s.CashOnHand
	if err := s.BuyOrUpgradeModule("window_expansion", now); err != nil { t.Fatal(err) }
	m := s.Module("window_expansion")
	if m == nil || !m.Installed || m.Level != 1 {
		t.Fatalf("expected same revision to be reinstalled, got %#v", m)
	}
	if s.CashOnHand != cashAfterPurchase {
		t.Fatalf("reinstalling should not charge for another revision: before=%d after=%d", cashAfterPurchase, s.CashOnHand)
	}
}
