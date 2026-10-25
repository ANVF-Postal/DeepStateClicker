package game

import (
	"errors"
	"fmt"
	"time"
)

const (
	InitialCashCapacity int64 = 100_000 // $1,000.00
	PrintAmount         int64 = 100     // $1.00
	CapacityUpgradeCost int64 = 2_500   // $25.00 (legacy single-box upgrade)
	CapacityUpgradeTo   int64 = 250_000 // $2,500.00 (legacy save compatibility)
	LockboxCost         int64 = 2_500   // $25.00
	LockboxCapacity     int64 = 500_000 // $5,000.00
	AutomationCost      int64 = 5_000   // $50.00

	BatchCycleTicks = 60
	ExploitWindowStart = 8
	ExploitWindowEnd = 10
)

var (
	ErrCapacityFull = errors.New("cash-on-hand capacity reached")
	ErrInsufficient = errors.New("not enough cash on hand")
	ErrUpgradeOwned = errors.New("upgrade already purchased")
	ErrNoEmail      = errors.New("no message selected")
	ErrWindowClosed = errors.New("the queue has moved on")
	ErrNoPendingReply = errors.New("there is no reply to send")
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
	TabStorage Tab = "STORAGE"
	TabMessages Tab = "MESSAGES"
	TabEmail Tab = TabMessages // compatibility with the early prototype
)

type Event struct {
	AtTick int64 `json:"at_tick"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	At     time.Time `json:"at,omitempty"`
}

type Message struct {
	ID string `json:"id"`
	Sender string `json:"sender"`
	Address string `json:"address"`
	To string `json:"to"`
	Subject string `json:"subject"`
	Body string `json:"body"`
	AtTick int64 `json:"at_tick"`
	Read bool `json:"read"`
	Kind string `json:"kind"`
	Choices []string `json:"choices,omitempty"`
}

type BatchState struct {
	Number int64 `json:"number"`
	Progress int `json:"progress_ticks"`
	QueuedDollars int `json:"queued_dollars"`
	Completed int64 `json:"completed_batches"`
	LastOutcome string `json:"last_outcome"`
	WindowOpen bool `json:"window_open"`
	WindowMissed bool `json:"window_missed"`
}

type UpgradeState struct {
	ExpandedCashCapacity bool `json:"expanded_cash_capacity"`
	PrintAutomation bool `json:"print_automation"`
	WindowCalibration bool `json:"window_calibration"`
}

type ModuleState struct {
	ID string `json:"id"`
	Level int `json:"level"`
	Installed bool `json:"installed"`
	Jammed bool `json:"jammed,omitempty"`
}

type ModuleSpec struct {
	ID string
	Name string
	Description string
	Color string
	Cost int64
	MaxLevel int
	Suspicion int
}

var ModuleCatalog = []ModuleSpec{
	{ID:"window_expansion", Name:"WINDOW EXPANSION", Description:"Extends the injection opportunity by one active second per revision.", Color:"#267C6C", Cost:2500, MaxLevel:3, Suspicion:1},
	{ID:"queue_calibration", Name:"QUEUE CALIBRATION", Description:"Makes the brief opportunity easier to recognize.", Color:"#315E85", Cost:1500, MaxLevel:3, Suspicion:1},
	{ID:"auto_injector", Name:"AUTOMATIC INJECTOR", Description:"Queues an injection automatically during an available window.", Color:"#6C4B91", Cost:5000, MaxLevel:3, Suspicion:3},
	{ID:"obfuscation", Name:"SIGNAL OBFUSCATOR", Description:"A future module intended to reduce scrutiny of installed hardware.", Color:"#8A6D2F", Cost:12000, MaxLevel:3, Suspicion:0},
}

func NewModuleState() []ModuleState {
	return []ModuleState{{ID:"legacy_controller", Level:1, Installed:true, Jammed:true}}
}

type State struct {
	SchemaVersion int `json:"schema_version"`
	RunID string `json:"run_id"`
	Status RunStatus `json:"status"`
	CurrentTab Tab `json:"current_tab"`
	CashOnHand int64 `json:"cash_on_hand_cents"`
	CashCapacity int64 `json:"cash_capacity_cents"`
	StorageUnlocked bool `json:"storage_unlocked,omitempty"`
	LockboxCount int `json:"lockbox_count,omitempty"`
	TotalPrinted int64 `json:"total_printed_cents"`
	EstimatedNetWorth int64 `json:"estimated_net_worth_cents"`
	Risk int `json:"risk"`
	Tick int64 `json:"tick"`
	EmailUnlocked bool `json:"email_unlocked"`
	EmailRead bool `json:"email_read"`
	Messages []Message `json:"messages,omitempty"`
	SelectedMessageID string `json:"selected_message_id,omitempty"`
	PendingReplyTicks int `json:"pending_reply_ticks,omitempty"`
	PendingFirstMessageTicks int `json:"pending_first_message_ticks,omitempty"`
	ITPatience int `json:"it_patience"`
	ITSuspicion int `json:"it_suspicion"`
	ITReplyCount int `json:"it_reply_count"`
	Batch BatchState `json:"batch"`
	Upgrades UpgradeState `json:"upgrades"`
	Modules []ModuleState `json:"modules,omitempty"`
	NewsIndex int `json:"news_index"`
	Events []Event `json:"events"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	RandomState uint64 `json:"random_state"`
}

var NewsHeadlines = []string{
	"WASHINGTON: Budget committee postpones routine hearing",
	"NEW YORK: Regional manufacturers report a modest rise in input costs",
	"CHICAGO: Transit authority approves maintenance funding",
	"MARKETS: Treasury yields edge higher in quiet trading",
	"ECONOMY: Small businesses report longer waits for credit",
	"CONSUMER PRICES: Household essentials rise for a third consecutive month",
	"SUPPLY WATCH: Several regional distributors report intermittent shortages",
	"PUBLIC SERVICES: Municipal budgets face renewed pressure from rising costs",
	"MARKETS: Currency volatility spreads beyond emerging markets",
	"WASHINGTON: Emergency session called as confidence in price stability erodes",
	"REPORT: Families delay prescriptions and utility payments as costs climb",
	"GLOBAL OUTLOOK: Several governments announce extraordinary economic measures",
}

func NewState(now time.Time) State {
	s := State{
		SchemaVersion: 1,
		RunID: fmt.Sprintf("DSC-%d", now.UnixNano()),
		Status: RunFederal,
		CurrentTab: TabPrint,
		CashCapacity: InitialCashCapacity,
		ITPatience: 3,
		RandomState: uint64(now.UnixNano()) ^ 0x9e3779b97f4a7c15,
		CreatedAt: now,
		UpdatedAt: now,
		Batch: BatchState{Number: 1, Progress: 0, LastOutcome: "Routine batch cycle active."},
		Modules: NewModuleState(),
		Events: []Event{{AtTick: 0, Kind: "discovery", Text: "The discrepancy is real. The exploit is already in place; the queue is moving.", At: now}},
	}
	if s.RandomState == 0 { s.RandomState = 1 }
	return s
}

// Normalize supplies defaults when loading saves created before the queue/inbox update.
func (s *State) Normalize() {
	if s.CurrentTab == "EMAIL" { s.CurrentTab = TabMessages }
	if s.CurrentTab == "" { s.CurrentTab = TabPrint }
	if s.CashCapacity == 0 { s.CashCapacity = InitialCashCapacity }
	if s.LockboxCount < 0 { s.LockboxCount = 0 }
	if s.Batch.Number == 0 {
		s.Batch.Number = s.TotalPrinted/PrintAmount + 1
		s.Batch.Completed = s.Batch.Number - 1
	}
	if s.ITPatience == 0 && len(s.Messages) == 0 { s.ITPatience = 3 }
	if s.RandomState == 0 { s.RandomState = 0x9e3779b97f4a7c15 }
	if len(s.Modules) == 0 { s.Modules = NewModuleState() }
	for i := range s.Modules {
		if s.Modules[i].ID == "queue_calibration" && s.Modules[i].Level > 0 { s.Upgrades.WindowCalibration = true }
		if s.Modules[i].ID == "auto_injector" && s.Modules[i].Level > 0 { s.Upgrades.PrintAutomation = true }
	}
	if s.Upgrades.WindowCalibration && moduleLevel(s, "queue_calibration") == 0 { s.Modules = append(s.Modules, ModuleState{ID:"queue_calibration", Level:1, Installed:true}) }
	if s.Upgrades.PrintAutomation && moduleLevel(s, "auto_injector") == 0 { s.Modules = append(s.Modules, ModuleState{ID:"auto_injector", Level:1, Installed:true}) }
	if len(s.Messages) == 0 && s.EmailUnlocked {
		s.addITMessage("Print reconciliation discrepancy", "Hi,\n\nI'm reviewing the reconciliation from the last print cycle. One count doesn't line up with the expected record. Could you take a look with me before end of day?\n\n— IT Operations", []string{"It may be a routine variance. I'll review the records.", "I can help review it. Which batch are you looking at?", "Leave it with me; I'll get back to you."})
		if s.EmailRead {
			if m := s.SelectedMessage(); m != nil { m.Read = true }
		}
	}
}

func (s *State) PrintOneDollar(now time.Time) error {
	// Kept as a compatibility wrapper for early callers. New printing is batch-based.
	if !s.Batch.WindowOpen { return ErrWindowClosed }
	return s.QueueExploit(now)
}

func (s *State) QueueExploit(now time.Time) error {
	if s.Status != RunFederal && s.Status != RunSandbox { return errors.New("this run is no longer active") }
	if !s.Batch.WindowOpen { return ErrWindowClosed }
	s.Batch.QueuedDollars++
	s.Batch.LastOutcome = "An extra print instruction is queued for the active batch."
	s.UpdatedAt = now
	return nil
}

func moduleLevel(s *State, id string) int {
	for _, m := range s.Modules { if m.ID == id && m.Installed && !m.Jammed { return m.Level } }
	return 0
}

func (s *State) Module(id string) *ModuleState {
	for i := range s.Modules { if s.Modules[i].ID == id { return &s.Modules[i] } }
	return nil
}

func (s *State) BuyOrUpgradeModule(id string, now time.Time) error {
	var spec *ModuleSpec
	for i := range ModuleCatalog { if ModuleCatalog[i].ID == id { spec = &ModuleCatalog[i]; break } }
	if spec == nil { return errors.New("unknown module") }
	m := s.Module(id)
	if m == nil {
		s.Modules = append(s.Modules, ModuleState{ID:id})
		m = &s.Modules[len(s.Modules)-1]
	}
	if m.Jammed { return errors.New("this module is jammed and must be removed before it can be serviced") }
	if m.Level >= spec.MaxLevel { return ErrUpgradeOwned }
	cost := spec.Cost * int64(m.Level+1)
	if s.CashOnHand < cost { return ErrInsufficient }
	s.CashOnHand -= cost
	m.Level++
	m.Installed = true
	if id == "queue_calibration" { s.Upgrades.WindowCalibration = true }
	if id == "auto_injector" { s.Upgrades.PrintAutomation = true }
	s.addEvent("module_upgrade", spec.Name+" advanced to revision "+fmt.Sprint(m.Level)+".", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) RemoveModule(id string, now time.Time) error {
	m := s.Module(id)
	if m == nil || !m.Installed { return errors.New("module is not installed") }
	if m.Jammed { return errors.New("module is jammed; safe removal requires a specialist") }
	m.Installed = false
	if id == "queue_calibration" { s.Upgrades.WindowCalibration = false }
	if id == "auto_injector" { s.Upgrades.PrintAutomation = false }
	s.addEvent("module_removed", m.ID+" removed from the hardware rack.", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) RemoveJammedLegacyModule(now time.Time) error {
	m := s.Module("legacy_controller")
	if m == nil || !m.Installed { return errors.New("no legacy module is installed") }
	const removalCost int64 = 5000
	if s.CashOnHand < removalCost { return ErrInsufficient }
	s.CashOnHand -= removalCost
	m.Installed = false
	m.Jammed = false
	s.addEvent("module_removed", "A specialist carefully removes the seized legacy controller without damaging the printer.", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) BuyLockbox(now time.Time) error {
	if s.CashOnHand < LockboxCost { return ErrInsufficient }
	s.CashOnHand -= LockboxCost
	s.LockboxCount++
	added := LockboxCapacity
	if s.LockboxCount > 3 {
		added = 350_000 // Clutter makes each additional loose box less space-efficient.
		s.Risk = clamp(s.Risk+1,0,100)
	}
	s.CashCapacity += added
	s.StorageUnlocked = true
	s.addEvent("storage", fmt.Sprintf("Lockbox %02d added to the apartment. Capacity increased by $%s.",s.LockboxCount,moneyString(added)),now)
	s.UpdatedAt = now
	return nil
}

func (s *State) BuyCapacityUpgrade(now time.Time) error {
	if s.Upgrades.ExpandedCashCapacity { return ErrUpgradeOwned }
	if s.CashOnHand < CapacityUpgradeCost { return ErrInsufficient }
	s.CashOnHand -= CapacityUpgradeCost
	s.CashCapacity = CapacityUpgradeTo
	s.Upgrades.ExpandedCashCapacity = true
	s.addEvent("upgrade", "You replace the shallow drawer with a lockable cash box.", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) BuyAutomation(now time.Time) error {
	if s.Upgrades.PrintAutomation { return ErrUpgradeOwned }
	if s.CashOnHand < AutomationCost { return ErrInsufficient }
	s.CashOnHand -= AutomationCost
	s.Upgrades.PrintAutomation = true
	s.addEvent("upgrade", "A quiet modification lets the exploit queue itself during future opportunities.", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) BuyWindowCalibration(now time.Time) error {
	if s.Upgrades.WindowCalibration { return ErrUpgradeOwned }
	if s.CashOnHand < 1_500 { return ErrInsufficient }
	s.CashOnHand -= 1_500
	s.Upgrades.WindowCalibration = true
	s.addEvent("upgrade", "A subtle calibration makes the queue's brief opportunity easier to recognize.", now)
	s.UpdatedAt = now
	return nil
}

// Advance progresses only while the application is open. Each call represents one active second.
func (s *State) Advance(now time.Time) {
	if s.Status != RunFederal && s.Status != RunSandbox { return }
	s.Tick++
	s.Batch.Progress++
	s.Batch.WindowOpen = false
	windowStart, windowEnd := ExploitWindowStart, ExploitWindowEnd
	if level := moduleLevel(s, "window_expansion"); level > 0 { windowEnd += level }
	if level := moduleLevel(s, "queue_calibration"); level > 0 { windowEnd++ }
	if s.Upgrades.WindowCalibration && moduleLevel(s, "queue_calibration") == 0 { windowEnd++ }
	if s.Batch.Progress >= windowStart && s.Batch.Progress <= windowEnd {
		s.Batch.WindowOpen = true
		if (moduleLevel(s, "auto_injector") > 0 || s.Upgrades.PrintAutomation) && s.Batch.QueuedDollars == 0 && s.CashOnHand+PrintAmount <= s.CashCapacity {
			s.Batch.QueuedDollars = 1
			s.Batch.LastOutcome = "The calibrated exploit queued an instruction automatically."
		}
	}
	if s.Batch.Progress == windowEnd+1 && s.Batch.QueuedDollars == 0 && !s.Batch.WindowMissed {
		s.Batch.WindowMissed = true
		s.Batch.LastOutcome = "The queue moved on. No instruction was sent."
	}
	if s.Batch.Progress >= BatchCycleTicks {
		if s.Batch.QueuedDollars > 0 {
			amount := int64(s.Batch.QueuedDollars) * PrintAmount
			beforeOverflow := maxInt64(0,s.CashOnHand-s.CashCapacity)
			s.CashOnHand += amount
			s.TotalPrinted += amount
			afterOverflow := maxInt64(0,s.CashOnHand-s.CashCapacity)
			newOverflow := afterOverflow-beforeOverflow
			if newOverflow > 0 {
				riskAdded := int((newOverflow+99)/100) // Each excess dollar adds exposure, capped by the risk scale.
				s.Risk = clamp(s.Risk+max(1,riskAdded),0,100)
				s.Batch.LastOutcome = fmt.Sprintf("Batch %05d completed. $%s entered capital; capacity exceeded by $%s.",s.Batch.Number,moneyString(amount),moneyString(afterOverflow))
				s.addEvent("overflow",s.Batch.LastOutcome,now)
			} else {
				s.Batch.LastOutcome = fmt.Sprintf("Batch %05d completed. $%s entered capital on hand.", s.Batch.Number, moneyString(amount))
				s.addEvent("batch_complete", s.Batch.LastOutcome, now)
			}
		} else {
			s.Batch.LastOutcome = fmt.Sprintf("Batch %05d completed. No extra instruction was sent.", s.Batch.Number)
		}
		s.Batch.Completed++
		s.Batch.Number++
		s.Batch.Progress = 0
		s.Batch.QueuedDollars = 0
		s.Batch.WindowOpen = false
		s.Batch.WindowMissed = false
	}
	// The first discrepancy becomes visible only after enough unrecorded cash
	// has actually cleared a batch. Delivery is then delayed by a seeded,
	// active-time countdown rather than tied to the next batch boundary.
	if s.TotalPrinted >= 1_000 && !s.EmailUnlocked && s.PendingFirstMessageTicks == 0 {
		s.PendingFirstMessageTicks = 12 + s.randomInt(34) // 12–45 active seconds
	}
	if s.PendingFirstMessageTicks > 0 {
		s.PendingFirstMessageTicks--
		if s.PendingFirstMessageTicks == 0 && !s.EmailUnlocked && s.TotalPrinted >= 1_000 {
			s.EmailUnlocked = true
			s.addITMessage("Print reconciliation discrepancy", "Hi,\n\nI'm reviewing the reconciliation from the last print cycle. One count doesn't line up with the expected record. Could you take a look with me before end of day?\n\n— IT Operations", []string{"It may be a routine variance. I'll review the records.", "I can help review it. Which batch are you looking at?", "Leave it with me; I'll get back to you."})
			s.addEvent("message_received", "A new message has arrived in your internal inbox.", now)
		}
	}
	if s.CashCapacity > 0 && s.CashOnHand*100 >= s.CashCapacity*80 { s.StorageUnlocked = true }
	if s.Tick%30 == 0 { s.Risk = clamp(s.Risk+1, 0, 100) }
	s.advanceMail(now)
	// Net worth is an estimate, not spendable cash.
	// No assets exist yet, so net worth is exactly the cash currently held.
	s.EstimatedNetWorth = s.CashOnHand
	if s.Tick%75 == 0 && s.NewsIndex < len(NewsHeadlines)-1 { s.NewsIndex++ }
	s.UpdatedAt = now
}

func (s *State) OpenTab(tab Tab) error {
	if tab == TabMessages && !s.EmailUnlocked { return errors.New("MESSAGES is not available yet") }
	if tab == TabStorage && !s.StorageUnlocked { return errors.New("STORAGE is not available yet") }
	if tab != TabPrint && tab != TabStore && tab != TabStorage && tab != TabMessages { return errors.New("unknown tab") }
	s.CurrentTab = tab
	return nil
}

func (s *State) ReadEmail(now time.Time) error {
	m := s.SelectedMessage()
	if m == nil { return ErrNoEmail }
	m.Read = true
	s.EmailRead = true
	s.addEvent("message_read", "Opened message: "+m.Subject, now)
	s.UpdatedAt = now
	return nil
}

func (s *State) SelectedMessage() *Message {
	if len(s.Messages) == 0 { return nil }
	for i := range s.Messages {
		if s.Messages[i].ID == s.SelectedMessageID { return &s.Messages[i] }
	}
	return &s.Messages[len(s.Messages)-1]
}

func (s *State) SelectMessage(id string, now time.Time) error {
	for i := range s.Messages {
		if s.Messages[i].ID == id {
			s.SelectedMessageID = id
			s.Messages[i].Read = true
			s.UpdatedAt = now
			return nil
		}
	}
	return ErrNoEmail
}

func (s *State) ReplyToSelected(choice int, now time.Time) error {
	m := s.SelectedMessage()
	if m == nil || m.Kind != "it" || len(m.Choices) == 0 { return ErrNoPendingReply }
	if choice < 0 || choice >= len(m.Choices) { return errors.New("invalid reply choice") }
	if s.PendingReplyTicks > 0 { return errors.New("a reply is already pending") }
	reply := m.Choices[choice]
	s.ITReplyCount++
	switch choice {
	case 0:
		s.ITPatience = clamp(s.ITPatience-1, 0, 5)
		s.ITSuspicion = clamp(s.ITSuspicion+1, 0, 10)
	case 1:
		s.ITSuspicion = clamp(s.ITSuspicion-1, 0, 10)
		s.ITPatience = clamp(s.ITPatience+1, 0, 5)
	case 2:
		s.ITPatience = clamp(s.ITPatience-1, 0, 5)
		s.ITSuspicion = clamp(s.ITSuspicion+2, 0, 10)
	}
	m.Body += "\n\nYOU ("+now.Format("15:04")+")\n"+reply
	m.Choices = nil
	s.PendingReplyTicks = 3 + s.randomInt(7) // 3–9 active seconds, never offline
	s.addEvent("message_sent", "You replied to IT Operations. The next message may arrive sooner than you'd like.", now)
	s.UpdatedAt = now
	return nil
}

func (s *State) IgnoreSelectedMessage(now time.Time) error {
	m := s.SelectedMessage()
	if m == nil { return ErrNoEmail }
	if m.Kind == "it" {
		s.ITPatience = clamp(s.ITPatience-1, 0, 5)
		s.ITSuspicion = clamp(s.ITSuspicion+1, 0, 10)
		s.addEvent("message_ignored", "The IT message remains unanswered. The discrepancy remains.", now)
	}
	s.UpdatedAt = now
	return nil
}

func (s *State) advanceMail(now time.Time) {
	if s.PendingReplyTicks <= 0 { return }
	s.PendingReplyTicks--
	if s.PendingReplyTicks > 0 { return }
	s.ITReplyCount++
	subject := "RE: Print reconciliation discrepancy"
	body := "I checked the batch history again. The mismatch is still there. I'd rather resolve this quietly, but I need something I can put in my notes."
	choices := []string{"I found a routine counting issue and will document it.", "Let's review the underlying records together.", "I don't have anything else to add."}
	if s.ITSuspicion >= 4 || s.ITReplyCount >= 3 {
		subject = "RE: Reconciliation — follow-up required"
		body = "I've compared the records against the original batch summaries. This isn't the first time the numbers have diverged. I need a clear explanation, and I'm keeping a copy of the discrepancy notes."
		choices = []string{"I'll help you reconcile the records.", "The process is more complicated than it looks.", "You should close this out without escalating it."}
	}
	if s.ITPatience <= 1 {
		body += "\n\nPlease respond today. I can't leave this open indefinitely."
	}
	s.addITMessage(subject, body, choices)
	s.addEvent("message_received", "A reply has arrived from IT Operations.", now)
}

func (s *State) addITMessage(subject, body string, choices []string) {
	id := fmt.Sprintf("IT-%03d", len(s.Messages)+1)
	s.Messages = append(s.Messages, Message{
		ID: id, Sender: "Daniel Mercer — IT Operations", Address: "daniel.mercer@frb.internal",
		To: "You", Subject: subject, Body: body, AtTick: s.Tick, Kind: "it", Choices: choices,
	})
	s.SelectedMessageID = id
	s.EmailUnlocked = true
}

func (s *State) randomInt(n int) int {
	if n <= 1 { return 0 }
	x := s.RandomState
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	if x == 0 { x = 0x9e3779b97f4a7c15 }
	s.RandomState = x
	return int(x % uint64(n))
}

func (s *State) addEvent(kind, text string, now time.Time) {
	s.Events = append(s.Events, Event{AtTick: s.Tick, Kind: kind, Text: text, At: now})
}

func moneyString(cents int64) string { return fmt.Sprintf("%d.%02d", cents/100, cents%100) }

func maxInt64(a,b int64) int64 { if a>b { return a }; return b }

func clamp(n, min, max int) int {
	if n < min { return min }
	if n > max { return max }
	return n
}
