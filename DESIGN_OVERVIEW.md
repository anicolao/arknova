# Design Overview

## 1. Purpose and Scope

The system runs an in-person Ark Nova session across two classes of device:

1. A large, horizontal, multi-touch display showing shared public state.
2. One personal phone or tablet per player showing private state and prompts.

The design supports one to four players, reconnectable sessions, deterministic
save/replay, auditable rules, and a UI usable from every edge of a physical
table. Online matchmaking and remote play are not initial requirements, though
the protocol should not prevent them.

This document describes logical boundaries. It intentionally does not prescribe
a frontend framework, transport library, or database yet.

## 2. Non-Negotiable Event Model

### 2.1 The log contains input, not consequences

The canonical record is an append-only stream of accepted player actions and
intentions. A log entry answers:

> Who asked to do what, with which explicit choices, at which game revision?

It does not store the changes calculated by the rules engine.

Example input event:

```json
{
  "eventId": "01J...",
  "gameId": "game-123",
  "revision": 84,
  "actor": { "kind": "player", "seat": 2 },
  "type": "BuildRequested",
  "payload": {
    "actionCardInstanceId": "action-build-p2",
    "xTokensSpent": 1,
    "placements": [
      { "building": "standard-enclosure-3", "anchor": "F7", "rotation": 2 }
    ]
  },
  "clientActionId": "phone-2:193",
  "expectedRevision": 83
}
```

Replay derives all consequences: effective strength, legality, money spent,
occupied hexes, covered placement bonuses, action-card movement, newly available
choices, and the next player.

Events that must **not** appear include `MoneyChanged`, `AppealAdvanced`,
`CardDrawn`, `BreakStarted`, or `TurnAdvanced`. Those are computed facts.

### 2.2 Accepted actions only

Malformed, unauthorized, stale, or illegal requests are rejected and are not
part of the game log. They may appear in operational logs or metrics, which are
separate from the canonical game history.

The append transaction is:

1. Load or replay state at the current revision.
2. Authenticate the actor and verify seat ownership.
3. Check `expectedRevision` and `clientActionId`.
4. Validate the proposed action against current legal choices.
5. Append the input event atomically.
6. Replay or incrementally reduce it into a new state.
7. Publish new projections to connected devices.

`clientActionId` makes retries idempotent. `expectedRevision` prevents two
devices from silently acting on different states.

### 2.3 Multi-step actions

Ark Nova frequently requires choices during resolution. The reducer may enter a
derived pending-decision state, but it does not invent the answer.

For example:

```text
AnimalsActionChosen
  -> derived state requests an animal
AnimalPlayRequested
  -> derived state requests an enclosure when multiple are legal
EnclosureChosen
  -> derived state requests an optional ability target
AbilityTargetChosen
  -> derived state completes the action and advances the turn
```

Each entry is a real player decision. The pending prompts and intermediate game
state are derived from the preceding entries.

An action may bundle choices that the player made atomically, such as a building
type, orientation, and location. It should not bundle future decisions merely to
reduce event count.

### 2.4 Administrative actions

Session-management actions use explicit human actors and the same immutable
stream where they affect gameplay. Examples include `GameConfigured`,
`SeatClaimed`, `PlayerReplaced`, `UndoProposed`, and `UndoAccepted`.

Background processes must not fabricate gameplay actions. A timer, animation,
network reconnect, or projection rebuild never creates a canonical event.

## 3. Deterministic Replay

The state function is conceptually:

```text
state = replay(rulesetVersion, contentVersion, initialConfiguration, inputEvents)
```

Given the same inputs, replay must be independent of wall-clock time, machine,
locale, hash-map iteration order, and client behavior.

### 3.1 Randomness

Random results are computed, not logged. A game configuration therefore commits
to deterministic randomness inputs:

- a seed;
- a named pseudorandom algorithm and version;
- a stable rule for deriving independent streams or draw positions;
- the hash of the complete action ancestry leading to the random operation.

Each shuffle, draw, or random selection is derived from those inputs and the
currently eligible pool. `DrawCardsRequested` does not identify which cards were
drawn; replay determines them. Randomness must depend on all prior accepted
actions in the current lineage, not merely on a global seed, a fixed deck
permutation, or a draw counter. A stable construction is to derive each random
value from:

```text
random-key = PRF(game-seed, rules-version, lineage-action-hash, random-purpose)
```

`lineage-action-hash` commits to every preceding action in order.
`random-purpose` identifies the operation and its ordinal within the resolving
action, such as initial-hand card 3, replacement-display card 1, or randomized
setup choice. Selection is made from a canonically ordered list of the remaining
eligible items. Domain separation prevents one type of random operation from
consuming or perturbing another by accident.

This construction has two required properties:

1. Replaying the same lineage reproduces exactly the same hidden information.
2. A new undo branch has different action ancestry and therefore produces new
   hidden information after its branch point, even if players subsequently make
   the same choices they made before the undo.

For casual local games, the host may include a random seed in the human-initiated
`GameConfigured` action. For verifiable fairness, use commit/reveal:

1. Each player submits `RandomnessCommitted` with a hash.
2. Each player submits `RandomnessRevealed` with their secret.
3. Replay validates the commitments and deterministically combines the secrets.

No server-selected hidden seed is required. A recovery policy must be defined
for a player who commits but refuses to reveal before play begins.

### 3.2 Time

Wall-clock timestamps are metadata, not rules inputs. Initial implementation
should avoid timed gameplay. If timers are introduced, a player or administrator
must submit the action that resolves an expiry; replay cannot depend on when it
happens to run.

### 3.3 Versioning

`GameConfigured` pins:

- ruleset identifier and semantic version;
- card/content pack version;
- map pack and enabled expansions;
- RNG algorithm version;
- player count and scenario options.

Old reducers and content packs remain available for old logs. A migration creates
a new game/log lineage; it never silently changes the meaning of an existing
history.

Golden replay tests should store representative input logs and assert a stable
state digest at selected revisions.

## 4. State, Rules, and Effects

### 4.1 Immutable game state

The reducer returns a new immutable state for each accepted input. State includes:

- turn and active-seat information;
- action-card order and upgraded sides;
- player resources and tracks;
- zoo-map occupancy and adjacency indexes;
- remaining deck contents, discard pile, display, and private hands;
- cards and persistent effects in each zoo;
- association workers, tasks, universities, partner zoos, and projects;
- pending decisions and their legal options;
- break and end-game progress;
- deterministic RNG cursor/state.

Snapshots may cache this computed state for performance. They are disposable,
versioned artifacts—not canonical records—and can always be rebuilt from the log.

### 4.2 Command validation

Every event type has a validator operating on the pre-event state. Validation
checks turn ownership, phase, prerequisites, affordability, target legality,
placement geometry, action strength, and visibility constraints.

The server should expose derived legal actions so clients can guide interaction,
but server validation remains authoritative. A client being unable to display a
choice is never proof that the choice is illegal.

### 4.3 Effect engine

Card and map effects are structured data or typed code. They are not inferred
from localized card text. Effects are resolved through a deterministic queue with:

- source card or board location;
- triggering condition;
- mandatory or optional status;
- legal target query;
- cost and consequence functions;
- explicit ordering rules;
- references to the pinned rules/content version.

The queue itself is derived. When human input is needed, resolution pauses and
produces a pending decision. A corresponding input event resumes it.

### 4.4 Explainability

The engine should produce an ephemeral explanation trace during replay:

```text
BuildRequested at revision 84
  action strength: slot 4 + 1 X-token = 5
  cost: 3 hexes × 2 = 6
  placement bonus at F8: +1 reputation
  Build action moves to slot 1
```

This trace is computed and need not enter the canonical log. It supports an
action history UI, debugging, rules disputes, and accessibility narration.

## 5. Privacy and Projections

### 5.1 The canonical log is server-private

An action-only log can still contain secrets: choosing a discard, selecting from
a private hand, or revealing a committed random value. Clients must not receive
the full canonical log or authoritative state.

After each revision, the projection layer creates:

- a **public tabletop projection**;
- one **private projection per seat**;
- an **observer projection**, if spectators are supported;
- an **administrative projection** for local recovery tools.

A private projection contains public state plus only that seat's hand, prompts,
and permitted hidden history. Projection DTOs must be allow-listed; do not
serialize authoritative objects and remove a few fields afterward.

### 5.2 Hidden information in history

Public history entries are generated from the authoritative input plus replay
context. They can say “Player 2 drew two cards” without identifying those cards.
A player's own history may show the identities. Previously hidden information
may become public when the rules reveal it.

### 5.3 Transport security

Personal devices pair to a seat using a short-lived QR code or numeric code shown
on the table. The server issues a seat-scoped credential. Local TLS is desirable;
at minimum, session tokens must be unguessable, revocable, and never embedded in
URLs or logs.

The tabletop has no privileged access to private projections merely because it
runs on the host computer. An explicit administrator mode must be visibly entered
for recovery or debugging.

## 6. System Boundaries

```text
                  immutable accepted input
Table UI  ─┐       events                 ┌─ Event Store
           ├─> Session API -> Validator ──┤
Phone UIs ─┘                              └─ Snapshot Cache
                          │
                          v
                 Deterministic Reducer
                          │
                 Authoritative State
                          │
                  Projection Builder
                    ┌─────┴─────┐
                    v           v
              Public Table   Seat-Private Views
```

Suggested modules:

- **content:** versioned cards, maps, icons, localization keys, and media IDs;
- **rules:** event schemas, validators, reducer, effects, scoring, and legal moves;
- **event-store:** atomic append, stream reads, integrity hashes, and snapshots;
- **session-server:** authentication, seat pairing, commands, and subscriptions;
- **projections:** public/private DTOs and redacted history;
- **table-client:** multi-touch public interface and seat-relative presentation;
- **companion-client:** private hand, prompts, inspection, and confirmation;
- **tools:** replay inspector, deterministic log editor for tests, and diagnostics.

The rules module must not depend on UI, networking, wall-clock time, storage, or
media files.

## 7. Physical Table Interaction

### 7.1 Coordinate systems

Game coordinates are canonical and independent of seating. Each client converts
them into screen coordinates. Rotating a label or control for a seat never
changes zoo-map geometry or event payloads.

Every touch begins in a seat zone inferred from the nearest table edge. Controls
that affect a player must either belong to that seat's edge or require companion
confirmation. Ambiguous center-table touches use the active player's context.

### 7.2 Interaction ownership

Only the active player can initiate normal game actions, but all players may
inspect public cards and boards concurrently. Inspection overlays are local UI
state and never become log events.

Recommended division:

- Table: spatial placement, public target selection, track inspection.
- Companion: hand selection, private discards, secret choices, detailed reading.
- Either with confirmation: irreversible public action when no hidden data is
  involved.

When an action crosses devices, use a shared interaction ID. Example: a player
selects an enclosure on the table, selects an animal on their phone, previews the
combined action on both, then confirms once. Only the final accepted intent is
appended.

### 7.3 Orientation and accessibility

- Keep core board orientation stable; rotate peripheral controls toward seats.
- Never rely only on color for player identity or legal targets.
- Provide zoomed card inspection on companions and optional table magnification.
- Use generous touch targets and tolerate imprecise finger placement.
- Announce pending private decisions publicly without revealing their content.
- Keep animations cancellable and derive their end state from the projection.

Animations are presentation only. Skipping or replaying one cannot affect rules.

## 8. Networking, Reconnects, and Persistence

The host machine runs the authoritative server and durable event store. Clients
communicate using request/response for action submission and a revisioned stream
for projection updates.

On reconnect, a client presents its seat credential and last seen revision. The
server sends either projection deltas or a fresh projection. A client never needs
the canonical log to recover.

Every accepted event is durably committed before success is returned. The event
store should support:

- append with expected revision;
- unique event and client-action IDs;
- per-entry schema version;
- integrity hashing or hash chaining;
- periodic disposable snapshots;
- export/import of a complete versioned game archive;
- explicit branching for development and replay tools.

Loss of a companion must not corrupt a session. A seat can be re-paired from the
table through a visible administrative action.

## 9. Undo and Correction

The canonical stream is never edited or truncated in place.

For casual play, undo is modeled as a human request that creates a new branch
from an earlier revision. The UI discloses the proposed rewind point, the actions
that will be abandoned, and whether any hidden information was revealed after
that point. Every player must explicitly agree.

`UndoProposed` and each player's response are human input events in the original
lineage. Once all required players accept, a new branch is created from the
agreed revision. Its ancestry includes a deterministic commitment to the undo
proposal and acceptance events. This branch commitment distinguishes it from the
abandoned history and becomes part of subsequent random derivation.

Actions after the rewind point are **not copied automatically** into the new
branch. The game returns to the derived state at that point and the players must
play forward again, submitting new actions and responding to newly derived
prompts. Agreeing to an undo therefore means agreeing to replay that portion of
the game, not merely deleting one action and retaining its later consequences.

If the rewind is before hidden information was revealed—for example, before a
card draw—the corresponding random operation on the new branch produces a
different result. The previously seen card does not become the replacement
branch's card. This prevents an undo from acting as a free preview of a
deterministic deck. The new result is still deterministic: replaying the new
branch, including its undo ancestry, always reveals the same replacement card.

The old lineage is retained for audit and debugging, but its post-rewind state
has no effect on the new branch. Competitive modes may disable undo entirely.

Administrative correction should prefer an explicit rules-supported player
action. Arbitrary state editing breaks the action-only invariant and is excluded
from normal sessions.

## 10. Testing Strategy

### Reducer tests

- Unit-test each event against legal and illegal states.
- Property-test resource invariants, deck uniqueness, map occupancy, and action
  ordering.
- Replay every test from revision zero rather than trusting incremental state.
- Assert that incremental reduction equals full replay.

### Privacy tests

- Snapshot every projection by viewer role.
- Assert private card IDs and option details never enter public payloads.
- Test history redaction before and after card revelation.
- Treat logs, errors, analytics, and crash reports as possible leakage paths.

### Determinism tests

- Run golden logs on every supported platform.
- Compare canonical state digests at checkpoints.
- Randomize collection insertion order in tests where possible.
- Prohibit ambient randomness, current time, locale-sensitive parsing, and
  floating-point calculations in rules code.

### Interaction tests

- Simulate stale revisions, duplicate submissions, disconnects, and retries.
- Test simultaneous public inspection during active-player input.
- Test handoff between table placement and companion confirmation.
- Validate every seat orientation and common screen aspect ratio.

## 11. Open Decisions

- Implementation language for the deterministic rules kernel.
- Durable event-store format and game archive format.
- Web versus native shell for the tabletop and companions.
- Exact confirmation policy for table-originated actions.
- Commit/reveal recovery policy and whether casual play needs it.
- Scope of undo in local, solo, and competitive modes.
- Licensing path for Ark Nova rules expression, artwork, text, and branding.
- Whether expansion support is designed into the first content schema or added
  only after the base-game engine is stable.
