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
| Canonical store | SQLite in WAL mode | Atomic expected-revision appends, durable local operation, branch queries, straightforward backup |
| Projection store | Separate disposable SQLite database | Fast reads without making projections authoritative |
| Web clients | SvelteKit, Svelte 5, TypeScript | Shared with the sibling apps; strong touch-oriented component model and Playwright support |
| Package manager | Bun with a committed lockfile | Fast, deterministic frontend scripts and test orchestration |
| Client transport | HTTP commands plus WebSocket projection stream | Clear mutation boundary and efficient revisioned updates |
| Schemas | Versioned JSON wire types generated from or checked against Go definitions | Inspectable game archives and cross-language compatibility |
| E2E | Playwright Chromium | Multi-context testing, deterministic screenshots, mobile/table viewports |
| Environment | Nix flake | Reproducible Go, Bun, Node, SQLite, and browser dependencies on nix-darwin and CI |

This is a local-first distributed web application. A dedicated server device
runs the Go process and owns storage, rules, authentication, and versioned static
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
  eventstore/                     # append-only SQLite log and branch metadata
  projections/                    # public/private read model builders
  sessions/                       # seat pairing, credentials, and connection lifecycle
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

## 3. Runtime Topology

```text
┌──────────────────────────┐       HTTPS/WSS       ┌──────────────────────────┐
│ Dedicated table device   │<--------------------->│ Dedicated server device  │
│ /table public client     │       local LAN       │ Go session server        │
│ no canonical storage     │                       │ canonical event store    │
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

In production the server binds to an explicitly configured LAN interface and
advertises its service on the local network. Development may bind to loopback.
Host-header validation, origin checks, TLS, and role/seat-scoped credentials
apply even on a trusted home network.

### 3.1 Discovery and client bootstrap

The server advertises a versioned service using mDNS/DNS-SD, for example
`_arknova._tcp`. The table can also accept a manually entered server URL or scan
an administrator bootstrap QR code. Discovery identifies candidates; it does not
establish trust.

On first connection, an administrator authorizes the table device and the server
issues a revocable `table` role credential. The table then loads the exact web
client build served by that server. Serving clients from the server keeps the UI,
projection schema, content pack, and protocol versions aligned without
coordinated application-store updates. A separately installed shell or PWA must
still reload versioned assets from the selected server before joining a game.

The table displays the connected server name, certificate identity, health, and
projection revision in an administrator-accessible status area. It never silently
switches to another discovered server during a session.

### 3.2 Device and network failure semantics

- If the table disconnects, the server remains authoritative and durable. Normal
  gameplay pauses; companions do not advance the game without the shared table.
- The disconnected table keeps its last public projection visibly marked stale
  and disables action submission. It must not simulate later state locally.
- After table refresh, reboot, or Wi-Fi recovery, it authenticates again and
  requests a fresh full projection before accepting input.
- If one companion disconnects, the table remains usable for public inspection;
  play pauses only when that seat owes a private decision.
- If the server disconnects or restarts, every client enters a read-only
  reconnecting state. No client elects itself server and no offline actions are
  queued for later append.
- Server recovery replays the canonical store and publishes a fresh projection.
  Clients discard obsolete drafts whose expected revision or branch no longer
  matches.

## 4. Canonical Action Store

### 4.1 Storage model

The canonical SQLite database contains append-only input and lineage metadata:

```sql
create table games (
  game_id text primary key,
  created_at_ms integer not null,
  active_branch_id text not null
);

create table branches (
  branch_id text primary key,
  game_id text not null,
  parent_branch_id text,
  parent_revision integer,
  branch_commitment blob not null,
  created_at_ms integer not null
);

create table input_events (
  branch_id text not null,
  revision integer not null,
  event_id text not null unique,
  client_action_id text not null,
  actor_kind text not null,
  actor_id text not null,
  event_type text not null,
  schema_version integer not null,
  payload_json blob not null,
  recorded_at_ms integer not null,
  previous_hash blob not null,
  event_hash blob not null,
  primary key (branch_id, revision),
  unique (branch_id, actor_id, client_action_id)
);
```

The exact schema may evolve, but these invariants do not:

- a branch revision is contiguous and unique;
- rows are never updated or deleted during normal operation;
- `client_action_id` makes retries idempotent;
- every row commits to its predecessor through a hash chain;
- actor identity and event schema version are explicit;
- timestamps are audit metadata and never rules inputs.

Database permissions and triggers should reject updates and deletes outside an
explicit archive-maintenance tool. Backups copy the canonical database plus the
pinned content packs required to replay it.

### 4.2 Append transaction

For every submitted action:

1. Authenticate the session and seat.
2. Begin an immediate SQLite transaction.
3. Read the branch head and compare `expectedRevision`.
4. Return the existing result for a repeated `clientActionId`.
5. Load a verified snapshot or replay to the branch head.
6. Decode and validate the input against current state.
7. Append the input row and updated hash-chain head.
8. Commit before acknowledging success.
9. Reduce and publish projections for the new revision.

No `MoneyChanged`, `CardDrawn`, `TurnAdvanced`, or other computed consequence is
written. Rejected requests belong only in bounded operational logs.

### 4.3 Branches and undo

An accepted undo creates lineage metadata rather than truncating history. The
new branch references the agreed parent revision and commits to the original
lineage's `UndoProposed` and acceptance actions. Its commitment participates in
all later random derivation.

Post-rewind actions are not copied. Players play forward from the reconstructed
state, and hidden information after the branch point is newly and
deterministically selected for that branch.

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

Projection rows in the disposable SQLite database are caches. Each row records
its branch, revision, projection schema version, and viewer class. The server may
rebuild the entire database from the canonical log. Startup verifies the cached
head and rebuilds on mismatch.

Snapshots of authoritative state are also disposable. A snapshot records its
state schema, branch, revision, and source event hash. It is accepted only after
that hash matches the canonical chain.

## 7. API and Realtime Protocol

### 7.1 Commands

Mutation endpoints express human intent:

```http
POST /api/games/{gameId}/actions
Content-Type: application/json
Authorization: Bearer <seat credential>

{
  "type": "BuildRequested",
  "schemaVersion": 1,
  "expectedRevision": 83,
  "clientActionId": "device-id:193",
  "payload": { ... }
}
```

Success returns the accepted revision and event ID, not a list of consequences.
Conflicts return the current revision. Validation errors use stable machine codes
plus safe user-facing messages and never expose hidden legal options.

Session and administrative actions have separate endpoints and explicit roles.
All mutating requests require JSON, origin validation, and authentication.

### 7.2 Projection stream

After pairing, each client opens a WebSocket authorized for exactly one viewer.
Messages contain:

- game, branch, and revision;
- projection schema version;
- a full projection or a safe delta;
- a monotonic stream sequence for gap detection.

On any gap, branch change, or incompatible schema, the client requests a fresh
full projection. The protocol favors correctness over clever incremental state.
Backpressure is bounded; a slow client is disconnected and reconnects from a
fresh projection rather than accumulating unbounded updates.

## 8. Pairing and Session Security

The authorized table requests a short-lived pairing nonce from the separate
server and displays a QR code containing the server origin, certificate
fingerprint, game ID, and nonce. A player chooses or is assigned a seat on the
public table, confirms on the companion, and receives a revocable seat
credential directly from the server. The table never proxies or learns the
companion credential.

Requirements:

- pairing codes expire and cannot be replayed;
- a credential grants only one game and seat;
- replacing a device visibly revokes the previous credential;
- credentials are stored outside URLs and diagnostic output;
- private projections are never cached by a shared service worker;
- the admin UI requires a deliberate physical-table confirmation;
- game archives containing hidden history are treated as private data.

TLS on the local network uses a stable server identity with a documented trust
bootstrap. The administrator bootstrap and player pairing screens show a short
certificate fingerprint so a discovered device cannot impersonate the game
server. Development may use loopback HTTP only.

## 9. Web Client Architecture

One SvelteKit application serves distinct table and companion shells while
sharing types, cards, dialogs, and projection transport.

Client state is divided into:

- **server projection:** replaced or patched only by revisioned stream messages;
- **ephemeral UI state:** inspection, zoom, hover, drag preview, and orientation;
- **draft intent:** unsubmitted choices spanning table and companion;
- **connection state:** pairing, revision, reconnect, and resync status.

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
  canonical.sqlite
  projections.sqlite
  snapshots/
  content/
  archives/
```

On server startup it:

1. checks canonical database integrity and hash chains;
2. verifies required versioned content is present;
3. validates or discards snapshots;
4. verifies the projection head and rebuilds if necessary;
5. starts LAN discovery and resumes serving only after a consistent state is
   available.

Canonical appends are acknowledged only after durable commit. Projection failure
after commit is recoverable by replay and must not roll back accepted history.
The table device is replaceable without restoring data: authorize a replacement,
load the client from the server, and rebuild its entire view from a fresh
projection.

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
event payloads, credentials, private card IDs, and full projections by default.

Useful metrics include append latency, replay duration, snapshot hits, projection
rebuild time, WebSocket reconnects, dropped updates, rejected stale revisions,
and active sessions. Metrics never influence rules.

Every accepted action can produce an ephemeral explanation trace from the pure
kernel. Traces are available in local diagnostics and the player-facing action
history after projection-safe redaction.

## 12. Testing Boundaries

No one test layer is the source of truth for everything:

- pure unit and property tests prove rules and invariants;
- golden replay tests prove deterministic compatibility;
- event-store tests prove atomicity, idempotency, branching, and recovery;
- projection tests prove privacy and viewer-specific contracts;
- API tests prove authentication and protocol behavior;
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
5. Projection and snapshot deletion cannot lose canonical gameplay.
6. Clients cannot calculate or submit computed consequences.
7. Event appends use expected revision and idempotency keys.
8. Every saved game pins all replay-relevant versions.
9. Undo creates a branch and new action ancestry; history is never truncated.
10. Local and CI commands use the same Nix-defined toolchain.
11. Production game media is replaceable and is never imported into source
    control without confirmed rights.
