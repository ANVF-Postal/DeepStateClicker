# Prototype 0.1 — Design Contract

## Player fantasy
A Federal Reserve employee discovers a printing anomaly. The interface initially feels like a plain internal workstation. The player can print a few extra physical dollars, but their ability to hold, move, and conceal cash is limited. Choices leave history; money can buy time, not erase consequences.

## Prototype loop
1. Left-anchored title/menu and short intro sequence.
2. PRINT screen: cash-on-hand counter, $1 print action, $1,000 starting capacity.
3. STORE: one capacity upgrade and one automation upgrade.
4. EMAIL: first IT employee message, unlocked by early play.
5. Active-only simulation ticks, a simple risk score, and a persistent event log.
6. Versioned save/load plumbing, with run status separated from ordinary game state.

## Architecture rules
- The simulation must not depend on terminal UI, input, or save-file details.
- Game rules are expressed as methods over explicit state and are covered by unit tests.
- All money is integer cents; no floating-point currency.
- Cash-on-hand capacity is separate from future storage locations and assets.
- Save data is versioned. Save writes use a temporary file plus rename.
- Federal eligibility is explicit state. Death and retirement will be modeled as irreversible transitions; this prototype only lays down the status fields and save contract.
- No offline progress. Ticks occur only while the application is running.
- Randomness must eventually be injected/seeded so story and simulation tests remain reproducible.

## Future-compatible event model
Major interventions (including geopolitical crises) should later be authored as game events with prerequisites, cost, uncertain outcomes, duration, and persistent consequences. The prototype does not implement real-world operational instructions or detailed mechanics for staging attacks. Its event system is intended to support abstract fictional consequences and moral stakes.

## Deliberate non-goals for 0.1
No crypto, casino, warehouses, assassination resolution, world map, audio, custom bitmap art, external services, or sophisticated anti-tamper save system. Those features should be designed after the core loop and state transitions are tested.
