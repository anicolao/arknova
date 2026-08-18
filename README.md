# Ark Nova Digital Tabletop

An in-person digital implementation of Ark Nova designed for a large horizontal
touchscreen with players seated around every edge. The shared table presents the
public board and supports direct manipulation; each player uses a phone or tablet
for private cards, choices, and information.

The project is built around one architectural rule:

> The durable game record is an immutable sequence of player actions and
> intentions. Current state, consequences, available choices, animation, and UI
> are derived by replaying that sequence.

The log records what a player chose—not the state changes the rules engine
calculated afterward. For example, it records `BuildRequested` with a tile and
hex placement, not `MoneyReduced`, `TilePlaced`, and `BonusGranted` events.

## Product Shape

### Shared tabletop

- Public zoo maps, association board, card display, tracks, and supply.
- Touch interaction from all four edges.
- A neutral shared presentation: private hands never appear on the table.
- Seat-aware orientation for labels, controls, prompts, and animations.
- Clear indication of the active player, pending decisions, and legal targets.

### Personal device

- Private hand and hidden choices.
- Detailed card inspection without occupying shared space.
- Confirmation of consequential actions initiated on the table.
- Private prompts, optional effects, and multi-step decisions.
- Reconnection to the same seat without exposing information to other players.

The tabletop should remain playable as a social board-game surface. Phones are
companions for secrecy and detail, not isolated copies of the whole game.

## Core Principles

- **Action-only history:** Persist accepted player input, never derived output.
- **Deterministic replay:** The same log, ruleset, and content version always
  produce the same state.
- **Server authority:** The server validates and appends actions. Clients submit
  intent and render projections.
- **Privacy by projection:** The authoritative state may contain hidden data;
  each device receives only the view permitted for its seat.
- **Rules as code:** Card effects are structured rules, not parsed display text.
- **Explainability:** Any visible value can be traced to prior player actions and
  rule applications.
- **Local-first sessions:** A game should survive device refreshes, temporary
  Wi-Fi loss, restarting the tabletop display, and reconnecting to the separate
  game server.
- **Versioned games:** Saved sessions remain replayable after software and card
  data evolve.

See [DESIGN_OVERVIEW.md](./DESIGN_OVERVIEW.md) for the product architecture,
[TECHNICAL_ARCHITECTURE.md](./TECHNICAL_ARCHITECTURE.md) for the proposed
implementation architecture, [E2E_GUIDE.md](./E2E_GUIDE.md) for the full-stack
testing contract, [MVP_MILESTONES.md](./MVP_MILESTONES.md) for the vertical
development roadmap, and [RULES_SUMMARY.md](./RULES_SUMMARY.md) for the
base-game rules reference.

## Status

Increment 001 is merged and has passed its physical table-and-phone test.
Increment 002 is implemented on its focused branch: a pinned synthetic content
pack is deterministically dealt into a public display and private hands, players
can alternate the X-token action through five ordered action cards, and deleting
SQLite still recovers the exact state from the canonical JSONL actions. Its
physical-device acceptance remains outstanding.

## Development Environment

The repository uses a Nix flake and supports nix-darwin:

```console
nix develop
nix flake check
```

Build and run the current application with:

```console
nix develop --command sh -c 'cd web && bun install --frozen-lockfile && bun run build'
nix develop --command go run ./cmd/arknova \
  -listen 0.0.0.0:8080 \
  -data ./data \
  -web ./web/build \
  -content ./content/synthetic \
  -public-url http://YOUR-SERVER-NAME:8080
```

For iterative testing on a physical table and phones, use the automatically
reloading Vite and Go environment documented in
[MANUAL_TESTING.md](./MANUAL_TESTING.md).

Open `/table` at the configured server URL. Run all Go and browser checks with:

```console
nix develop --command go test ./...
nix develop --command sh -c 'cd web && bun run check:precommit && bun run test:e2e'
```

## Preview Release Package

The deployable preview is a deterministic x86-64 Linux release containing the
static Go server, built web client, and source-derived `build.json`:

```console
nix build .#packages.x86_64-linux.arknova-release
file result/bin/arknova
cat result/build.json
```

CI wraps that release in `release.tar.gz` plus a run-specific
`deployment.json`; the latter records the PR, workflow run, full commit SHA,
payload size, and SHA-256 digest. The packaging command requires those values as
environment variables and refuses a non-static or non-x86-64 server.

After the Kenobi host module and DNS have been reviewed and activated, set the
repository secret `KENOBI_PREVIEW_SSH_KEY` to the dedicated forced-command SSH
private key and the repository variable `KENOBI_PREVIEW_ENABLED` to `true`.
Successful CI runs for same-repository pull requests then deploy to
`https://pr-<number>.arknova.morpheum.dev`. The trusted default-branch workflow
re-checks the current PR SHA and verifies the complete artifact envelope before
deployment. Closing a PR removes its route, and a daily reconciliation workflow
cleans up any missed closures. Keep the enable variable unset until the host,
wildcard certificate, DNS, and key have all been tested.

The committed `web/bun.nix` is the offline Nix dependency cache description
derived from `web/bun.lock`. Regenerate it whenever the Bun lock changes:

```console
nix develop --command bun2nix --lock-file web/bun.lock --output-file web/bun.nix
```

All changes are pushed on focused branches and opened as draft pull requests for
initial review. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## Local Asset Inspection

The installed official Steam game uses Unity Addressables. For local research,
the extractor can export readable textures from a macOS Steam installation:

```console
nix run .#extract-steam-assets -- \
  --group all \
  --output ./steam-assets
```

Targeted groups include `cards`, `playmats`, `maps`, and `icons`. Extracted
directories are git-ignored and must remain local.

An older utility can inspect a locally available Tabletop Simulator Workshop
JSON:

```console
nix run . -- --json /path/to/mod.json --output ./tts-assets
```

## Intellectual Property

Ark Nova, its rules expression, artwork, card text, branding, and other assets
belong to their respective rights holders. Purchasing a physical or digital copy
does not grant redistribution or public-performance rights. Extracted assets are
for local technical evaluation only and must not be committed, distributed, or
served by a deployed application without an appropriate license.

The architecture should keep rules data and media replaceable so development can
use placeholders and licensed production content can be integrated later.

## License

The original source code and documentation in this repository are licensed
under the [GNU General Public License v3.0](./LICENSE). This license does not
apply to Ark Nova artwork, card text, branding, or other third-party material.
