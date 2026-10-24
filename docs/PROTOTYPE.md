# Prototype 0.3 — The Queue and the Inbox

## Player fantasy

A Federal Reserve employee has discovered an anomaly in a printing process. The exploit is already set up. The player does not start the machine; they watch its ordinary batch queue and occasionally intervene when a brief opportunity appears. Every dollar leaves a mark.

## Core loop

1. The title screen leads to a short preamble establishing the anomaly and the already-prepared exploit.
2. A batch progresses through a roughly one-minute queue cycle while the application is open. The exploit window appears early in the cycle.
3. A brief visual opportunity appears inside that cycle. The player may queue one or more extra dollar instructions while the window is open.
4. The batch completes later. Only then does the cash-on-hand balance and total printed amount change.
5. A missed opportunity is not a failure screen; the ordinary queue continues and another batch follows.
6. Store upgrades expand physical cash capacity, make the opportunity easier to recognize, or eventually automate the exploit.
7. The MESSAGES tab unlocks an IT Operations thread. Structured messages have sender, address, subject, timestamp, body, read status, and explicit reply choices.
8. Replies have randomized delays measured in active simulation ticks. The character's patience and suspicion persist and influence later wording.
9. The world-news ticker progresses from routine economic coverage to more serious signs of economic stress.

## Interface contract

- The PRINT screen is an operational dashboard, not a standalone clicker button.
- Cash on hand, cash capacity, total printed, and estimated net worth are distinct fields. Estimated net worth is not spendable.
- A single shared layout calculation determines terminal dimensions and the corresponding mouse hit regions.
- Wide terminals place the inbox and reading pane side by side. Narrow terminals stack them.
- The news ticker is a single clipped line at the bottom.
- Tabs draw attention only when something needs the player's attention. Ordinary unread messages pulse amber; urgent threat states can pulse red.
- The exploit window is communicated visually, not through explanatory tutorial text.
- The queue keeps moving while the player reads messages. Timers pause when the application is closed.

## Simulation architecture

- The game model owns the batch phase, progress, queued instructions, completion, money, risk, character state, and message delays.
- The terminal layer only renders state and dispatches player actions.
- Random state is stored in the game state so that saves preserve the random sequence and tests can start from a known seed.
- All money uses integer cents.
- Save fields are additive for compatibility with the early prototype. Old saves are normalized when loaded.
- No offline progression.

## Deliberate non-goals for 0.3

No warehouses, crypto, casino, world map, sound effects, custom bitmap artwork, geopolitical intervention system, assassination resolution, or sophisticated anti-tamper save system. The current IT thread is a foundation for more complex branching conversations, not the final narrative. The preamble text is placeholder copy until the author supplies the definitive story and fade timing.
