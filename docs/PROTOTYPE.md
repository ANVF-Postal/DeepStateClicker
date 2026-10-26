# Operations Console Prototype — MAIN, Modules, and Storage

## Player fantasy

A Federal Reserve employee discovers an anomaly in a printing process. The exploit is already prepared. The player watches an ordinary batch queue and intervenes during a brief opportunity. The terminal begins as an orderly government interface; as the operation grows, the system reports increasingly uncomfortable consequences.

## Core loop

1. A batch progresses through a roughly one-minute queue cycle while the application is open.
2. A brief opportunity appears early in the cycle. The player can queue extra printing instructions; cash arrives when the batch completes.
3. Capacity is a safety threshold, not a hard cutoff. Cash above capacity remains available but increases risk.
4. MAIN / PRINT OPERATIONS displays the capital panel and printing controls.
5. UPGRADES purchases and improves printer modules. Each module occupies a logical bay while installed; revision upgrades improve the module rather than creating duplicate copies.
6. HARDWARE RACK uses bordered panes: the module catalogue on the left and installed hardware on the right. Both panes scroll with keyboard or mouse wheel, and the rack shows a scrollbar indicator and an empty bay marker.
7. Acquiring or reinstalling a module transfers it to the rack immediately, then starts a randomized firmware sequence capped at about 13 seconds under normal visible animation. Each boot selects 4–8 unique status messages from a larger pool, with each phase lasting 0.7–1.6 seconds. Transitions nudge the progress bar, which can also pause, creep, or surge within a phase. Effects remain disabled until the sequence completes and displays a brief green `ACTIVATED!` confirmation. Booting pauses when the application is closed.
8. MAIN displays persistent system UPTIME as `HHH:MM:SS`. It is saved across app exits and is intended to reset only after an actual system restart, not when the game is closed.
7. A legacy controller begins jammed in the rack. Removing it requires a paid specialist removal, representing the cost of doing the job discreetly and safely.
8. STORAGE unlocks as capital approaches apartment capacity. Lockboxes add capacity, but clutter reduces the efficiency of additional boxes and adds exposure.
9. The MESSAGES tab unlocks an IT Operations thread. Structured messages support explicit reply choices, delayed responses, and persistent character reactions.
10. The news bulletin flashes, then scrolls each headline across the terminal. Its subject matter grows more serious as the run continues.

## Interface contract

- The main terminal uses labelled windows: a title interrupts the top border, following the style of traditional terminal file managers.
- CAPITAL is a distinct financial panel. It separates capital on hand, capacity, estimated net worth, total printed, and capacity utilization.
- Capacity utilization is green below 80%, amber from 80% to below 100%, and indicates overflow at 100% and above. The overflow indicator may blink, but the text remains readable.
- MAIN has PRINT OPERATIONS, UPGRADES, and HARDWARE RACK internal views. STORAGE and MESSAGES are separate top-level tabs when unlocked.
- Module colours remain consistent between the catalogue, installed rack, and module details. Text labels accompany colour so meaning does not depend on colour alone.
- The rack has no hard-coded maximum number of bays. The UI must scroll its available-module and installed-module panes as those collections grow.
- A single shared layout calculation informs terminal dimensions and mouse hit regions.
- Wide terminals use two-pane layouts. Narrow terminals use compact layouts.
- The exploit opportunity is communicated visually, not through tutorial text.
- The simulation advances only while the application is open; there is no offline progression.

## Economy and storage

- All money uses integer cents.
- Estimated net worth is speculative and is not spendable cash.
- Cash capacity is not a hard limit. Each dollar of new overflow adds risk, capped by the risk scale.
- The apartment is the first physical storage problem. Lockboxes increase capacity, but excessive clutter wastes space and attracts attention.
- The current storage pass models total apartment capacity and lockbox clutter. Individual container balances, manual transfers into larger containers, property space, off-site warehouses, and cash transport remain future work.
- Future storage containers should reward consolidation without making one large container universally safest. Capacity, apartment space, exposure, and retrieval friction should be distinct trade-offs.
- Warehouses will hold assets and cash that are not immediately on hand; retrieving them will require transport and time.

## Module progression

- Module revisions improve the installed module's effects.
- Removing a module disables its effect. Reinstalling a removed module preserves its revision and requires a fresh initialization sequence.
- The first automatic injector is deliberately simple and conspicuous. More sophisticated timing variation and obfuscation should be introduced later, once automation itself creates meaningful scrutiny.
- Inspection events may confiscate installed modules if the player fails to prepare. Warnings and deadlines should be readable and fair.
- Module suspicion and rack-wide conspicuousness are future extensions; the current prototype establishes the data model and UI.

## Simulation architecture

- The game model owns batch progress, queued instructions, completion, money, risk, module state, storage capacity, character state, and message delays.
- The terminal layer renders state and dispatches player actions.
- Random state is stored in game state so saves preserve the random sequence and tests can use known seeds.
- Save fields are additive for compatibility with early prototypes. Old saves are normalized when loaded.
- No offline progression.

## Deliberate non-goals for this prototype

No warehouses, crypto, casino, world map, sound effects, custom bitmap artwork, geopolitical intervention system, or sophisticated anti-tamper save system. Storage container-to-container transfers and personal-property upgrades are planned for a later pass.
