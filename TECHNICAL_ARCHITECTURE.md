# Technical Architecture

## Status

This document proposes the implementation architecture for the Ark Nova digital
tabletop described in [DESIGN_OVERVIEW.md](./DESIGN_OVERVIEW.md). It turns the
logical constraints in that document into concrete technology, process, and
module choices.

The proposal incorporates practices proven in the sibling `food` and
`photostore` applications:

- SvelteKit clients with Playwright as the user-visible contract;
- a Nix-defined development and CI environment;
- generated E2E walkthroughs backed by committed zero-tolerance screenshots;
- an authoritative local service with deterministic fixtures;
- append-only history and rebuildable projections;
- small tracer-bullet scenarios that exercise real system boundaries.

It deliberately strengthens their patterns where this game's requirements are
stricter: the rules kernel is pure, the canonical log stores only accepted human
input, private projections are allow-listed, and replay determinism is tested as
a protocol compatibility guarantee.

## 1. Selected Technology

| Area | Choice | Rationale |
| --- | --- | --- |
| Rules and server | Go | Explicit data flow, good concurrency and networking, fast deterministic tests, simple local deployment |
| Canonical store | One append-only JSONL file per game | Human-readable actions, simple backup, and deterministic replay |
| Projection store | Disposable SQLite database | Fast reads and idempotency indexes without making projections authoritative |
| Web clients | SvelteKit, Svelte 5, TypeScript | Shared with the sibling apps; strong touch-oriented component model and Playwright support |
| Package manager | Bun with a committed lockfile | Fast, deterministic frontend scripts and test orchestration |
| Client transport | HTTP commands plus WebSocket projection stream | Clear mutation boundary and efficient revisioned updates |
| Schemas | Versioned JSON wire types generated from or checked against Go definitions | Inspectable game archives and cross-language compatibility |
| E2E | Playwright Chromium | Multi-context testing, deterministic screenshots, mobile/table viewports |
| Environment | Nix flake | Reproducible Go, Bun, Node, SQLite, and browser dependencies on nix-darwin and CI |

This is a local-first distributed web application. A dedicated server device
runs the Go process and owns storage, rules, game/player routing, and versioned static
web assets. The large tabletop is a separate physical device running only the
table client. Companions are additional network clients. All connect to the
server over the local network; no cloud service is required to play.

## 2. Repository Layout

```text
cmd/arknova/
  main.go                         # server, replay, inspect, and archive commands

internal/
  content/                        # versioned card/map definitions and validation
  game/                           # immutable domain state and value types
  events/                         # input-event schemas and version upgrades
  rules/                          # pure validation, reduction, effects, legal moves
  random/                         # versioned ancestry-keyed deterministic PRF
  eventlog/                       # JSONL append, locking, fsync, and replay cursor
  projections/                    # public/private read model builders
  sessions/                       # game/player routing and connection lifecycle
  server/                         # HTTP/WebSocket adapters; no game rules
  archive/                        # portable game export/import

web/
  src/lib/api/                    # typed command client and projection stream
  src/lib/components/             # shared presentation components
  src/lib/table/                  # shared-table interaction and orientation
  src/lib/companion/              # private hand, prompts, and confirmation
  src/routes/table/               # tabletop entry point
  src/routes/play/                # companion entry point
  src/routes/admin/               # explicit local recovery tools
  tests/e2e/                      # multi-surface Playwright scenarios
  scripts/test-server.ts          # isolated deterministic full-stack harness
  scripts/check-e2e-rules.ts      # enforced E2E policy

testdata/
  content/                        # tiny synthetic cards/maps, safe to commit
  replay/                         # golden input logs and state digests

TECHNICAL_ARCHITECTURE.md
DESIGN_OVERVIEW.md
E2E_GUIDE.md
RULES_SUMMARY.md
```

Go `internal` packages prevent accidental public API growth. Dependencies point
inward: adapters may import the rules kernel; the rules kernel imports neither
storage, networking, UI, clocks, filesystem, SQL, nor media packages.

### Content-pack asset contract

Synthetic and licensed content packs implement the same filesystem contract.
For every corresponding resource, both packs must provide the same relative
path and filename, media type, and intrinsic size (for example, identical pixel
dimensions for raster images). Selecting a pack changes only the configured
content root; URLs, manifests, application code, CSS, and tests must not rewrite
paths or apply pack-specific dimensions. Missing or mismatched resources fail
content-pack validation before a game starts.

## 3. Runtime Topology

```text
┌──────────────────────────┐       HTTPS/WSS       ┌──────────────────────────┐
│ Dedicated table device   │<--------------------->│ Dedicated server device  │
│ /table public client     │       local LAN       │ Go session server        │
│ no canonical storage     │                       │ actions.jsonl            │
└──────────────────────────┘                       │ projection cache/store   │
                                                  │ pure replay kernel       │
┌──────────────────────────┐                       └──────────────────────────┘
│ Companion devices        │                            ^              ^
│ /play seat-private       │----------------------------+              |
└──────────────────────────┘                                           |
                                                                       |
┌──────────────────────────┐                                           |
│ Admin browser            │-------------------------------------------+
└──────────────────────────┘
```

One process on the server device is authoritative for a game. The table is not a
fallback server and holds no canonical database. Clients never run an
authoritative reducer and never append directly to storage. A client may use
local optimistic presentation, but it must reconcile to the next server
projection and cannot expose optimistic hidden information.

In production the server binds to a configured LAN interface and port.
Host-header validation, origin checks, and TLS when configured apply at the HTTP
boundary. Development may bind to loopback.

### 3.1 Configured server URL and client bootstrap

The table is configured with the server device's stable hostname or IP address,
port, and scheme. For example:

```text
http://arknova-server:8080/table
https://gameserver.example.home:8443/table
```

The table opens that URL directly. There is no service discovery, candidate
selection, or administrator discovery handshake. If the configured server is
unreachable, the table shows a connection/configuration error and allows an
administrator to correct the name, port, or scheme.

The Go server serves the exact table and companion web-client build used with
its protocol, projection schema, and content pack. The table therefore needs
only a browser or kiosk shell and a configured server URL; it does not require a
separately coordinated application release.

The table displays the configured server URL, health, and projection revision in
an administrator-accessible status area. It never changes server during a game
unless an administrator explicitly changes the configuration and reconnects.

### 3.2 Device and network failure semantics

- If the table disconnects, the server remains authoritative and durable. Normal
  gameplay pauses; companions do not advance the game without the shared table.
- The disconnected table keeps its last public projection visibly marked stale
  and disables action submission. It must not simulate later state locally.
- After table refresh, reboot, or Wi-Fi recovery, it opens the same configured
  URL and requests a fresh full projection before accepting input.
- If one companion disconnects, the table remains usable for public inspection;
  play pauses only when that seat owes a private decision.
- If the server disconnects or restarts, every client enters a read-only
  reconnecting state. No client elects itself server and no offline actions are
  queued for later append.
- Server recovery replays the canonical action file and publishes a fresh projection.
  Clients discard obsolete drafts whose expected revision or branch no longer
  matches.

## 4. Canonical Action Log

### 4.1 File format

Each game has one canonical file:

```text
games/GAME_ID/actions.jsonl
```

Every complete line is one accepted human action encoded as a JSON object:

```json
{"actionId":"01J...","clientActionId":"table-7:193","actor":{"kind":"player","seat":2},"type":"BuildRequested","schemaVersion":1,"recordedAtMs":1710504000000,"payload":{"actionCardInstanceId":"action-build-p2","xTokensSpent":1,"placements":[{"building":"standard-enclosure-3","anchor":"F7","rotation":2}]}}
```

The file contains no SQL schema, projection rows, snapshots, computed outcomes,
or special storage records. `recordedAtMs` is audit metadata and never a rules
input. File order is action order; the zero-based line number is the action
revision. The action ID and client action ID are part of the envelope so reducers
and the disposable projection can detect duplicates.

No `MoneyChanged`, `CardDrawn`, `TurnAdvanced`, or other computed consequence is
written. Rejected requests belong only in bounded operational logs.

### 4.2 Append and projection flow

The server is the only normal writer. For every submitted action:

1. Read the game code and player number supplied by the client.
2. Acquire the game's event-log lock.
3. Compare `expectedRevision` with the current complete-line count maintained by
   the in-memory authoritative state.
4. Return the existing accepted result when `clientActionId` is already present
   in the projection's idempotency index.
5. Validate the proposed input against current state.
6. Serialize one compact JSON object followed by one newline.
7. Append the complete line to `actions.jsonl` and `fsync` the file.
8. Apply that action through the reducer inside a SQLite projection transaction.
9. Advance the projection's `next_offset` cursor to the new JSONL EOF in the same
   SQLite transaction.
10. Release the lock, acknowledge the accepted action, and publish projections.

The SQLite database stores a `projection_state` row containing the next byte
offset to reduce. On startup, the server seeks to that offset and applies every
complete remaining line. Crash behavior is intentionally simple:

- before the append: the action is absent and the client may retry;
- after append but before projection commit: the durable line is replayed;
- after projection commit: both the derived state and cursor already include it.

A trailing partial line indicates interrupted or corrupted storage and stops
startup for explicit recovery; it is never guessed or silently accepted. The
JSONL file may be copied for backup once appends are paused or while holding the
same lock. The projection database is excluded from backups because it is fully
rebuildable.

### 4.3 Undo in one linear file

Undo does not truncate or rewrite `actions.jsonl`. `UndoProposed` and each
player's acceptance are ordinary human-action lines. When the final required
acceptance is reduced, the reducer reconstructs the state at the agreed earlier
revision and begins a new logical lineage. Actions between that revision and the
accepted undo remain in the file for audit but are no longer part of current
game state.

Subsequent actions append to the same file. The new lineage's deterministic
random key includes the undo proposal and acceptance actions, so hidden
information after the rewind changes while replay of the complete JSONL remains
stable. No branch table, branch file, or mutable active-branch pointer is needed.

## 5. Pure Rules Kernel

The kernel exposes functions shaped like:

```go
type Engine interface {
    InitialState(config GameConfigured) (State, error)
    Validate(state State, input InputEvent) error
    Apply(state State, input InputEvent) (State, Trace, error)
    LegalActions(state State, viewer SeatID) LegalActionSet
    Project(state State, viewer Viewer) Projection
}
```

`Apply` is deterministic and side-effect free. It cannot read the clock, call a
random library, access SQL, iterate over unordered maps to make decisions, or
perform network requests. All collections involved in rules resolution have a
defined canonical order.

### 5.1 State and effects

State is immutable by contract. Implementations may use copy-on-write internally
after tests prove that prior revisions cannot be mutated.

Card and map behavior uses typed, versioned effect definitions. Localized text
is display data and is never parsed to decide rules. Effect resolution maintains
a derived queue. When human input is required, the state exposes a typed pending
decision and legal choices; the next accepted player input resumes resolution.

### 5.2 Random selection

Every random operation selects from a canonically ordered eligible pool using a
versioned pseudorandom function keyed by:

- the game seed or combined commit/reveal seed;
- rules and RNG versions;
- the complete lineage action hash;
- a domain-separated operation purpose and ordinal.

Random outputs are consequences and are not logged. Tests must cover repeatable
same-lineage replay, different results on an undo branch, uniqueness of drawn
cards, and stability across supported platforms.

### 5.3 Replay compatibility

Each game pins content, rules, event-schema, and RNG versions. Old implementations
remain callable by version. A new event schema may have a pure decoder/upgrader,
but existing canonical bytes and meanings never change silently.

Golden fixtures contain input logs, public/private projection snapshots, and a
canonical state digest at selected revisions. Any intentional compatibility
change requires explicit fixture review.

## 6. Projections and Privacy

Projection code consumes authoritative state and returns allow-listed wire DTOs:

- `TableProjection`: public board, public history, turn, and pending-seat signal;
- `SeatProjection`: public state plus that seat's hand, choices, and private
  history;
- `ObserverProjection`: delayed or restricted public information if enabled;
- `AdminProjection`: explicit recovery information, never sent implicitly.

Never serialize authoritative state and redact fields afterward. Tests recursively
scan public payloads, logs, errors, traces, and Playwright artifacts for private
fixture card IDs.

Projection rows in the disposable SQLite database are caches. The database also
records its projection schema version along with a `next_offset` cursor pointing
into `actions.jsonl`. The server may delete and rebuild the entire database by
reducing the file from byte offset zero.

## 7. API and Realtime Protocol

### 7.1 Commands

Mutation endpoints express human intent:

```http
POST /api/games/{gameId}/actions
Content-Type: application/json

{
  "player": 2,
  "type": "BuildRequested",
  "schemaVersion": 1,
  "expectedRevision": 83,
  "clientActionId": "device-id:193",
  "payload": { ... }
}
```

Success returns the accepted revision and event ID, not a list of consequences.
Conflicts return the current revision. Validation errors use stable machine codes
plus clear user-facing messages.

Session and administrative actions have separate endpoints. All mutating
requests require JSON and must pass the normal game/player turn validation.

### 7.2 Projection stream

Each client opens a WebSocket using its `gameid` and `player` parameters. The
server trusts those parameters and sends that viewer's projection. Messages
contain:

- game, branch, and revision;
- projection schema version;
- a full projection or a safe delta;
- a monotonic stream sequence for gap detection.

On any gap, branch change, or incompatible schema, the client requests a fresh
full projection. The protocol favors correctness over clever incremental state.
Backpressure is bounded; a slow client is disconnected and reconnects from a
fresh projection rather than accumulating unbounded updates.

## 8. Companion URLs and Recovery

The table renders one stable companion URL per player as a QR code. Each URL
contains the configured HTTP or HTTPS origin, game code, and player number. For
example:

```text
https://gameserver.example.home:8443/play?gameid=ABCD&player=2
```

A companion scans the QR code and opens the server-hosted companion UI directly.
There is no pairing exchange, nonce, account, credential, expiration, or
revocation. The server uses `gameid` to locate the game and `player` to select the
seat projection. It still validates that submitted actions are legal for the
specified player in the current state.

If a device is lost, refreshed, or replaced, the player scans the same QR code
again. The URL may also be typed or bookmarked. This intentionally favors simple
recovery over protection against players viewing another seat's URL.

HTTPS deployments use a certificate trusted by the table and companions. HTTP
is also supported when that is appropriate for the configured local network.
The QR code always uses the same configured origin that served the table so
companions connect to the intended server without origin rewriting.

## 9. Web Client Architecture

One SvelteKit application serves distinct table and companion shells while
sharing types, cards, dialogs, and projection transport.

Client state is divided into:

- **server projection:** replaced or patched only by revisioned stream messages;
- **ephemeral UI state:** inspection, zoom, hover, drag preview, and orientation;
- **draft intent:** unsubmitted choices spanning table and companion;
- **connection state:** game/player route, revision, reconnect, and resync status.

Ephemeral inspection never becomes an event. A draft becomes one canonical input
only after final confirmation. Cross-device drafts use a server-issued
interaction ID, contain no authoritative consequence, expire safely, and can be
cancelled without changing the game.

The table's canonical geometry is independent of visual orientation. Components
receive an edge/seat transform for labels and controls. Game coordinates in event
payloads never depend on pixels, viewport size, or seat rotation.

Use accessible native controls where possible, stable roles/names for broad
interaction, and `data-testid` only for precise game state or geometry targets.

## 10. Server Appliance Operation and Recovery

The dedicated server device owns a single data directory:

```text
data/
  games/
    GAME_ID/
      actions.jsonl
  projections.sqlite
  content/
  archives/
```

On server startup it:

1. verifies every canonical JSONL file through its last complete line;
2. verifies required versioned content is present;
3. checks each projection cursor against its JSONL file size;
4. reduces any unapplied tail or rebuilds SQLite from offset zero if necessary;
5. begins serving the configured HTTP/HTTPS name and port only after a consistent
   state is available.

Canonical appends are acknowledged only after the JSONL line is flushed and
`fsync` succeeds. Projection failure after that is recoverable by replay and must
not roll back or remove the accepted line.
The table device is replaceable without restoring data: configure a replacement
with the same server URL, load the client, and rebuild its entire view from a
fresh projection.

The CLI should eventually provide:

```console
arknova serve --data ./data --listen 127.0.0.1:8080
arknova replay --archive game.arknova
arknova verify --archive game.arknova
arknova rebuild-projections --data ./data
arknova export --game GAME_ID --output game.arknova
```

## 11. Observability

Operational logs are structured and separate from gameplay history. Include
request ID, game ID, branch, revision, duration, and safe error code. Exclude
event payloads, private card IDs, and full projections by default.

Useful metrics include append latency, replay duration, JSONL tail length,
projection rebuild time, WebSocket reconnects, dropped updates, rejected stale
revisions, and active sessions. Metrics never influence rules.

Every accepted action can produce an ephemeral explanation trace from the pure
kernel. Traces are available in local diagnostics and the player-facing action
history after projection-safe redaction.

## 12. Testing Boundaries

No one test layer is the source of truth for everything:

- pure unit and property tests prove rules and invariants;
- golden replay tests prove deterministic compatibility;
- action-log tests prove locking, complete-line append, `fsync` error handling,
  cursor replay, idempotency, undo lineage, and recovery;
- projection tests prove privacy and viewer-specific contracts;
- API tests prove game/player routing and protocol behavior;
- Playwright E2E tests are the primary proof of user-visible behavior and visual
  state across the table and companion surfaces.

The full E2E policy is defined in [E2E_GUIDE.md](./E2E_GUIDE.md).

## 13. CI and Reproducibility

The Nix shell should provide Go, Bun, Node.js, SQLite, Playwright Chromium and its
runtime libraries, formatting tools, and image comparison utilities. CI invokes
the same commands developers run locally:

```console
nix develop --command go test ./...
nix develop --command sh -c 'cd web && bun install --frozen-lockfile'
nix develop --command sh -c 'cd web && bun run check:precommit'
nix develop --command sh -c 'cd web && bun run build'
nix develop --command sh -c 'cd web && bun run test:e2e'
```

CI runs on macOS to match the target nix-darwin host. Linux checks may be added
for rules portability, but visual baselines have one named canonical platform.
Failed E2E runs upload the Playwright report, trace, console output, and screenshot
diffs. Successful runs do not rewrite baselines or generated walkthroughs.

## 14. Architectural Guardrails

These rules should be enforced by package boundaries, static checks, and tests:

1. Only accepted human actions enter the canonical log.
2. Rules code performs no I/O and reads no ambient time or randomness.
3. Every rules decision has a canonical collection order.
4. Public/private wire DTOs are allow-listed and tested for leaks.
5. Deleting the projection database cannot lose canonical gameplay.
6. Clients cannot calculate or submit computed consequences.
7. Event appends use expected revision and idempotency keys.
8. Every saved game pins all replay-relevant versions.
9. Undo creates a branch and new action ancestry; history is never truncated.
10. Local and CI commands use the same Nix-defined toolchain.
11. Production game media is replaceable and is never imported into source
    control without confirmed rights.
