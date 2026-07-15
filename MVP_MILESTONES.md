# MVP Milestones

## Purpose

This roadmap delivers Ark Nova as a sequence of complete tracer bullets. Every
increment crosses the real system from a player's touch, through the Go server
and canonical JSONL action log, through reducer-built SQLite projections, and
back to the table and companion browsers.

There are no milestones for “the backend,” “the rules library,” “the database,”
“the component system,” or another horizontal layer. Supporting code is created
only when a user-visible scenario needs it, and only to the depth that scenario
requires.

This document follows [TECHNICAL_ARCHITECTURE.md](./TECHNICAL_ARCHITECTURE.md),
[E2E_GUIDE.md](./E2E_GUIDE.md), and [RULES_SUMMARY.md](./RULES_SUMMARY.md).

## MVP Outcome

The MVP is complete when one to four people can sit around the physical table,
open stable companion URLs, and finish a base-game session using placeholder or
properly licensed content. It must support:

- setup, deterministic card order, private hands, and the public display;
- all five action cards on their basic and upgraded sides;
- zoo-map construction, animals, sponsors, association tasks, and projects;
- tracks, bonuses, breaks, income, final turns, and scoring;
- the base game's required card abilities and map behavior;
- server and browser restart recovery from `actions.jsonl`;
- consensual undo with branch-dependent new hidden information;
- table interaction from every supported edge;
- one to four players, including the base solo flow;
- deterministic replay and viewer-correct projections.

Extracted game media is not part of the deliverable. Development and CI use
synthetic content. Production media is integrated only after licensing is
resolved. Synthetic and licensed content packs are interchangeable: every
corresponding asset must have the same relative path and filename, media type,
and intrinsic dimensions. Application code and layouts must behave identically
when the configured pack changes; they must not contain pack-specific paths or
size overrides.

## Rules for Every Increment

An increment is complete only when all of the following are true:

1. A player can perform the new behavior through the real UI.
2. The accepted intent is visible as one action in the real `actions.jsonl`.
3. Restarting from that file recreates the demonstrated state.
4. The table and every affected companion show the correct projection.
5. A focused Playwright tracer-bullet scenario passes with zero-pixel baseline
   screenshots and a generated scenario README.
6. Reducer and replay tests cover the rules exercised by the scenario.
7. A human completes the documented manual test on the target table and at least
   one real companion device.
8. The increment works from a clean checkout through the Nix environment and CI.
9. No disabled, speculative, or unreachable subsystem was added “for later.”
10. Documentation describes the behavior that actually exists.
11. Every added synthetic asset has the licensed-pack-relative path, filename,
    media type, and intrinsic dimensions defined by the content contract.

E2E tests prove the visible journey. They do not excuse missing reducer,
determinism, projection, or persistence tests. Conversely, unit tests do not make
an increment complete when no player can exercise it.

## Verification Artifacts

Each increment adds or updates:

```text
web/tests/e2e/NNN-scenario/
  NNN-scenario.spec.ts
  README.md
  screenshots/

docs/manual/NNN-scenario.md
```

The generated E2E README records automated steps and screenshots. The manual
test document records:

- hardware and browser versions;
- server commit and content version;
- table URL and companion device classes;
- the exact actions performed;
- observed result and any defects;
- tester and date.

Manual evidence is concise, but it is never replaced by “looks good locally.”

## Increment 001: Start a Game on Two Devices

### Player outcome

A host opens the configured server URL on the table, creates a two-player game,
and sees a game code plus one QR code per player. Scanning player 1's QR code on
a phone opens `/play?gameid=CODE&player=1` and shows that player's empty private
area. Both surfaces show the same game revision.

### Thin implementation slice

- Go server serves the built `/table` and `/play` web clients.
- The table creates a game through one real command.
- `GameConfigured` is appended to `games/CODE/actions.jsonl` and `fsync`ed.
- The reducer produces the smallest useful game projection.
- SQLite stores that projection and its JSONL byte cursor.
- The table renders stable companion QR URLs.
- A WebSocket pushes revisioned table and player projections.
- Restarting the server rebuilds the same game screen from JSONL.

This is not permission to build a generic game framework, authentication system,
or reusable QR library. Implement only the path needed to create and reopen this
game.

### Automated tracer bullet

`001-create-and-connect`:

1. Open the table from the Go server.
2. Create a deterministic two-player game.
3. Verify one JSONL action exists through the test diagnostics endpoint.
4. Open both stable companion URLs in isolated browser contexts.
5. Verify the table and companions show the same game code and revision.
6. Restart the Go server and verify all three views recover.

### Manual test

Run the server on its intended device, open the table UI on the separate
touchscreen device, scan both QR codes with two physical phones, refresh every
browser, then restart the server. Confirm the game returns without reconfiguration
or credentials.

### Done when

The full round trip is demonstrable on physical devices, not merely through API
calls, and the generated E2E walkthrough shows every surface.

## Increment 002: Deal Private Hands and Take a Turn

### Player outcome

Starting a game visibly creates the public card display and private starting
hands from a deterministic synthetic deck. The active player chooses one of five
action cards on the table. The selected card moves to strength 1, intervening
cards shift, and the turn passes to the next player.

The first action has a deliberately small effect: choose “take an X-token” so
the complete action-card movement and turn loop can be proven before adding one
of the five large actions.

### Thin implementation slice

- A tiny synthetic content pack with stable card IDs and placeholder art.
- Deterministic initial draws derived from seed and action ancestry.
- Viewer-specific table and hand projections.
- Five action-card slots and the X-token alternative.
- Turn ownership and action-card shift rules.
- Public action history derived from accepted intents.

### Automated tracer bullet

`002-private-setup-and-turn`:

1. Create the game and open two companions.
2. Verify the table shows the public display but no private card names.
3. Verify each companion sees only its own synthetic hand.
4. Player 1 takes an X-token using an action card at strength 3.
5. Verify the card shifts to slot 1, the token appears, and player 2 becomes
   active on all surfaces.
6. Delete SQLite, restart, and verify the same hands and turn state replay.

### Manual test

On the physical table, inspect every action slot from both player edges. On each
phone, compare the hand with the other phone and table. Take alternating X-token
turns and confirm the shared state remains understandable without looking at the
server console.

### Done when

Two people can alternate indefinitely, private hands remain off the normal table
view, and replay reproduces the exact deal.

## Increment 003: Draw Cards from the Public Display

### Player outcome

A player selects the Cards action, chooses a legal basic-side draw based on its
strength and reputation range, receives cards privately, and sees the public
display close gaps and refill. The action card shifts and the next turn begins.

### Thin implementation slice

- Basic Cards action choices for strengths 1–5.
- Deck, display, hand, discard, and reputation-range rules needed by the action.
- Multi-step pending decisions when a player must choose or discard.
- Private companion prompts and public “waiting for player” state.
- Deterministic replacement-card selection.

### Automated tracer bullet

`003-cards-action`:

1. Player 1 selects Cards on the table.
2. The phone displays only legal draw choices.
3. Player 1 takes a card within reputation range and resolves any discard.
4. Verify the table never shows the private result.
5. Verify the display shifts/refills and all surfaces reach the next revision.
6. Rebuild projections and verify the same hidden result.

### Manual test

Exercise every basic Cards strength using a short fixture game. Attempt one
out-of-range display selection and confirm the UI explains why it is unavailable.
Refresh during a discard prompt and complete it after reconnecting.

### Done when

The first real Ark Nova action is fully playable through public and private
surfaces, including interruption and recovery mid-action.

## Increment 004: Build an Enclosure on the Zoo Map

### Player outcome

A player selects Build, chooses a standard enclosure size, previews rotations on
their zoo map, places it adjacent to an existing building, pays its cost, and
receives a covered placement bonus.

### Thin implementation slice

- Canonical hex coordinates and rotation independent of screen pixels.
- The small set of map spaces needed by the scenario: buildable, rock, water,
  placement bonus, and occupied.
- Basic Build legality, adjacency, size, and cost.
- Table placement preview plus confirm/cancel.
- Computed explanation showing strength, cost, and bonus.

### Automated tracer bullet

`004-build-enclosure`:

1. Select Build at a known strength.
2. Preview an invalid disconnected placement and verify it cannot be confirmed.
3. Rotate and confirm a legal three-space enclosure.
4. Verify money, occupied hexes, and placement bonus on all relevant views.
5. Verify JSONL contains the requested tile, anchor, and rotation—not computed
   money or bonus events.
6. Replay and compare the zoo-map projection pixel for pixel.

### Manual test

Place, rotate, cancel, and re-place tiles using fingers from every supported
table edge. Confirm targets are large enough and canonical map coordinates remain
correct when controls are visually rotated.

### Done when

Physical touch placement is reliable and the same requested placement replays to
the same map without storing computed consequences.

## Increment 005: Play an Animal from Phone to Table

### Player outcome

A player selects an animal privately on their phone, selects a legal empty
enclosure publicly on the table, reviews the combined action, and confirms. The
animal cost, enclosure occupation, appeal, icons, and one representative
immediate ability resolve visibly.

### Thin implementation slice

- Basic Animals action and combined strength requirement.
- Animal prerequisites, cost, enclosure size, and rock/water adjacency needed by
  the scenario.
- Cross-device draft interaction linking private card and public target.
- Occupied enclosure state, appeal track, and persistent icons.
- One synthetic immediate animal ability that uses the same typed effect path as
  later content.

### Automated tracer bullet

`005-play-animal`:

1. Select Animals on the table.
2. Select a synthetic animal on player 1's phone.
3. Verify the table names the acting seat without exposing the selected card.
4. Reject one enclosure that fails a printed adjacency requirement.
5. Confirm a legal enclosure and verify cost, appeal, occupation, and ability.
6. Verify player 2's companion never renders player 1's unrelated hand.

### Manual test

Run the phone-to-table handoff on both phone and tablet companions. Cancel at
each step, reconnect halfway through, and confirm that only a final confirmation
appends the animal-play intent.

### Done when

The defining private-to-public interaction feels coherent on real hardware and
survives cancellation and reconnects.

## Increment 006: Sponsors, Income, and the First Break

### Player outcome

A player can play a sponsor from their phone or use Sponsors to advance the
break. Reaching the end of the break track resolves hand limits, display cleanup,
worker return, and income. Play then continues with the correct next player.

### Thin implementation slice

- Basic Sponsors action and one immediate, one persistent, and one income
  sponsor effect.
- Break-track advancement and interruption after the current effect.
- Hand-limit prompt, display refresh, income calculation, and reset.
- Appeal income plus one kiosk or uncovered-map income source.
- A readable break summary on the table and private discard prompt on phones.

### Automated tracer bullet

`006-sponsors-and-break`:

1. Play a synthetic income sponsor.
2. Use Sponsors' alternative to trigger a break.
3. Resolve a private discard-to-limit prompt.
4. Verify display cleanup, income sources, worker return, and next player.
5. Verify only the sponsor/break player intents are logged; the computed break
   sequence is absent from JSONL.
6. Restart during the pending discard and complete the break after recovery.

### Manual test

Play through two breaks with different appeal and kiosk income. Have one player
refresh their phone during hand-limit selection. Confirm the table explains why
each player received their income.

### Done when

The game has a sustainable multi-turn economy and breaks require no manual
bookkeeping outside the UI.

## Increment 007: Association Work and Conservation

### Player outcome

A player sends an association worker to gain reputation, take a partner zoo or
university, or support a base conservation project. Supporting a project places
the player's token, awards conservation, and unlocks a conservation-track bonus.

### Thin implementation slice

- Basic Association task strengths and worker occupancy.
- Reputation, partner zoo, university, and conservation project projections.
- Project eligibility and support levels.
- One conservation bonus choice that pauses for companion input.
- Partner-zoo discount feeding back into the already playable Animals scenario.

### Automated tracer bullet

`007-association-and-conservation`:

1. Take a partner zoo through Association.
2. On a later turn, play an eligible animal and verify its discount.
3. Support a base project and choose a conservation bonus privately.
4. Verify worker occupancy, project token, tracks, and bonus on every surface.
5. Trigger a break and verify the worker becomes available again.

### Manual test

Exercise each basic task, including a visibly occupied task and an ineligible
project level. Confirm partner zoos and universities are legible on companions
without crowding the public board.

### Done when

Association choices participate in later actions rather than existing as an
isolated board demo.

## Increment 008: Upgrade Actions and Complete the Core Loop

### Player outcome

Conservation bonuses upgrade action cards. Players can use upgraded Cards,
Build, Animals, Association, and Sponsors actions, including multi-card,
multi-building, and multi-task choices where allowed. Special enclosures and
representative upgraded effects work through the existing interfaces.

### Thin implementation slice

- Upgrade selection and flipped action-card presentation.
- Upgraded forms of all five actions.
- Petting zoo and reptile house capacity rules.
- Multiple placements/tasks/cards with clear remaining-strength feedback.
- Hand limit increase from upgraded Cards.

### Automated tracer bullet

`008-upgraded-actions` is a short deterministic game that earns upgrades and
then demonstrates one materially upgraded behavior from every action card. Each
step asserts the result on the table and relevant companion and replays from
JSONL at the end.

### Manual test

Two players use every upgraded action at least once in a continuous game. Record
any point where the interface fails to explain combined strength, ordering, or
remaining choices.

### Done when

All five action cards are complete enough to support an uninterrupted core game
without developer controls.

## Increment 009: End the Game and Score It

### Player outcome

Appeal and conservation crossing triggers the end game, every other player gets
one final turn, final-scoring cards resolve, and the table presents the score and
tie-break explanation.

### Thin implementation slice

- End trigger and final-turn ownership.
- Private final-scoring cards and representative scoring conditions.
- Main-board appeal/conservation score conversion.
- Tie-break rules and an explainable score breakdown.
- Completed-game state that rejects further normal actions but remains replayable.

### Automated tracer bullet

`009-final-turns-and-scoring`:

1. Begin from a short action-log prelude near the end threshold.
2. Trigger the crossing with a visible conservation action.
3. Verify every eligible opponent receives exactly one final turn.
4. Reveal and score synthetic final cards.
5. Verify winner, negative-score handling, and tie-break breakdown.
6. Restart and verify the completed-game screen is identical.

### Manual test

Finish a two-player accelerated game and independently calculate the final score
from the visible state. Compare every line of the digital breakdown and attempt
one action after completion.

### Done when

A game has a rules-correct, understandable, and durable conclusion.

## Increment 010: Complete Base Content by Playable Mechanic

### Player outcome

Players can complete a normal base game with every required card, map, bonus,
and ability supported. Content is added in vertical mechanic groups, and each
group is demonstrated inside a playable scenario—not loaded into a catalog for
future use.

### Content slices

Implement one independently shippable slice at a time:

1. remaining immediate and persistent animal abilities;
2. remaining sponsor timing and income abilities;
3. conservation projects played from hand;
4. releasing animals and moving into special enclosures, including current
   enclosure-selection errata;
5. venom, constriction, multiplier, and interaction tokens;
6. remaining map bonuses, edge bonuses, and asymmetric maps;
7. remaining final-scoring conditions;
8. card-specific glossary exceptions.

Each slice adds a focused E2E tracer bullet using synthetic analogues plus
table-driven reducer tests for every production definition in that mechanic
group. A card definition is not “implemented” until its behavior is reachable
through a real player journey.

### Automated tracer bullets

Each numbered content slice owns a focused scenario that starts from a short
replayed prelude, performs the mechanic through the table and companion UI, and
verifies the resulting public/private projections. A final
`010-complete-base-game` scenario plays an accelerated but uninterrupted game
using at least one definition from every slice and reaches final scoring without
test-only state mutation.

### Manual test

After each slice, play a targeted fixture game without test controls. At the end
of the increment, complete at least two full base games with different maps and
record every unsupported card or manual workaround. The increment remains open
until that list is empty.

### Done when

All base definitions pass content validation, every exceptional mechanic has a
visible scenario, and full games require no developer intervention or rule
substitution.

## Increment 011: Undo Hidden Information and Recover Devices

### Player outcome

Players unanimously rewind an action, replay from the agreed point, and receive
new hidden information when the old lineage had revealed it. The server, table,
or any companion may restart without losing the active game.

### Thin implementation slice

- Undo proposal, affected-action summary, and unanimous response UI.
- Reducer-managed logical lineage in the same `actions.jsonl`.
- Ancestry-dependent random selection after rewind.
- Rejection of stale drafts and projection refresh after lineage change.
- Clear recovery screens for server, table, and companion disconnects.

### Automated tracer bullet

`011-undo-and-recovery`:

1. Draw and privately reveal a known synthetic card.
2. Propose and unanimously accept a rewind to before the draw.
3. Repeat the action and verify a different card is revealed.
4. Restart the server and verify the new card remains stable on replay.
5. Restart the table and companions independently.
6. Delete SQLite and verify JSONL rebuild produces the same active lineage.

### Manual test

Perform the scenario with physical devices, including unplugging the table from
the network and restarting the server process. Verify recovery instructions are
enough without opening a shell or editing files.

### Done when

Undo cannot be used as a deterministic preview, and common failures are
recoverable by players rather than developers.

## Increment 012: One to Four Players and Solo

### Player outcome

The same application supports one, two, three, and four players. Player-count
setup, break positions, seating, final turns, and projections adjust correctly.
Solo play uses its countdown and scoring flow.

### Thin implementation slice

- Player-count-specific setup and break behavior.
- Dynamic table layout and QR URLs for occupied seats.
- Three- and four-player final-turn ordering.
- Base solo setup, countdown, card-display cycling, and score target.
- Projection and touch performance with four active companions.

### Automated tracer bullets

- `012-player-counts` exercises setup, one round, a break, and end ordering for
  two, three, and four players.
- `013-solo-game` completes a short deterministic solo game through its final
  score.

### Manual test

Run one session at every player count on the physical table. For four players,
use devices on every table edge and verify simultaneous public inspection does
not interfere with the active player's action.

### Done when

No supported player count depends on hidden test setup or a layout that works
only on a developer monitor.

## Increment 013: Physical Table Acceptance

### Player outcome

Players can run a complete game on the target installation without a developer
present. Controls face the appropriate edges, touches are reliable, cards are
readable, reconnect states are clear, and the session remains responsive for a
full game's duration.

### Thin implementation slice

This increment fixes issues revealed by full physical play; it does not create a
new UI framework. Work is accepted only against a reproduced player problem:

- edge-aware orientation and ambiguous center-touch handling;
- accessible alternatives to gestures;
- touch target size and drag tolerance;
- card inspection and zoom on table and companions;
- reduced motion and stable animation completion;
- long-session memory, projection, and reconnect behavior;
- server/table startup instructions for a non-developer.

### Automated tracer bullet

`014-physical-table-acceptance` runs a representative multi-turn journey from
each table edge, uses touch input, inspects cards at table and companion sizes,
forces reconnects, and checks deterministic screenshots at the canonical table,
phone, and tablet viewports.

### Manual acceptance

Four people who did not implement the feature set up and complete a game using
only the documented operator instructions. Record duration, device failures,
mis-touches, unreadable states, rule questions, and any developer intervention.

### Done when

The external play group completes the game without developer intervention, no
critical or high-severity defect remains, and every discovered regression has an
automated scenario or a documented reason it cannot be automated.

## Increment Order and Change Policy

The increments are ordered by playable dependency, but scope may be split when a
tracer bullet becomes too large. A split must still end in visible behavior; it
cannot create “part A: backend” and “part B: frontend.”

An increment may move later when it is not required by the preceding player
journey. It may move earlier only when a real scenario is blocked. Architecture
cleanup, refactoring, performance work, and reusable abstractions travel inside
the increment whose scenario proves their need.

At every merge, `main` remains demonstrable. A partially wired layer, unreachable
rules engine, schema without a user, or component gallery without a journey is
not progress against this roadmap.
