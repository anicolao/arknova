# End-to-End Testing Guide

## Purpose

Playwright E2E scenarios are the primary proof that players can use the digital
tabletop correctly across its shared and private surfaces. They exercise the
real Go server, canonical event store, replay engine, projections, WebSocket
updates, SvelteKit clients, and persistence boundary.

E2E tests do not replace rules-kernel, replay, event-store, or privacy unit tests.
They prove the user-visible contract: a player's action can travel through the
whole system and produce the correct public and private experiences.

This guide adapts the strongest practices from the `food` and `photostore`
applications: zero-pixel visual baselines, one unified step API, generated
walkthrough documentation, deterministic isolated fixtures, short observable
waits, static enforcement, and CI parity.

## 1. Non-Negotiable Principles

- Use Chromium through Playwright.
- Use the real application stack; do not mock the rules server or projection
  protocol in full-stack scenarios.
- Give every scenario a fresh temporary game store and synthetic content pack.
- Fix all seeds, clocks, IDs, content versions, locale, timezone, and worker
  counts.
- Compare screenshots with zero differing pixels.
- Never use `waitForTimeout`, sleeps, or polling intervals as synchronization.
- No individual assertion or explicit wait may exceed 2000 ms.
- Treat the table and all companions as one scenario, not independent tests.
- Capture every meaningful user-visible state through the unified step helper.
- Generate each scenario README from the same steps that assert screenshots.
- Never place real Ark Nova artwork or extracted assets in fixtures, screenshots,
  traces, reports, or CI artifacts.
- Test private-information absence as aggressively as visible correctness.

## 2. What Belongs in E2E

E2E scenarios cover complete player journeys and device coordination:

- creating a game and pairing companions;
- selecting private cards on a phone and public targets on the table;
- submitting and confirming actions;
- seeing seat-specific prompts without leaking them to other devices;
- reconnecting at an exact revision;
- restoring a game after server restart;
- proposing and accepting an undo branch;
- replaying from the branch with different hidden information;
- responsive layout, edge orientation, and touch interaction;
- accessible names, focus, error messages, and recovery paths.

Do not enumerate every card combination through the browser. Exhaustive rules
matrices, geometry edge cases, randomness properties, and replay compatibility
belong in fast Go tests. Add an E2E scenario when a behavior crosses a runtime or
human-interaction boundary.

## 3. Directory Layout

```text
web/tests/e2e/
  helpers/
    fixtures.ts
    multi-surface-step-helper.ts
    pairing.ts
    privacy.ts
  001-create-and-pair/
    001-create-and-pair.spec.ts
    README.md
    screenshots/
      000-game-created--table.png
      000-game-created--seat-1.png
  002-private-card-public-target/
    002-private-card-public-target.spec.ts
    README.md
    screenshots/
  003-reconnect-and-restore/
    003-reconnect-and-restore.spec.ts
    README.md
    screenshots/
  004-undo-hidden-information/
    004-undo-hidden-information.spec.ts
    README.md
    screenshots/
```

Each numbered directory owns one tracer-bullet journey, its generated README,
and committed authoritative screenshots. Do not hand-edit a generated scenario
README; change the scenario metadata or steps and regenerate it.

Prefer several focused scenarios over one ever-growing test. A scenario may have
many steps when they form one indivisible cross-device journey.

## 4. Full-Stack Test Harness

`web/scripts/test-server.ts` owns the complete lifecycle for one Playwright run:

1. Create a temporary root outside the repository.
2. Write a tiny synthetic content pack with stable IDs and placeholder media.
3. Start the real Go server with an empty canonical store.
4. Start Vite on a strict loopback port and proxy `/api` and WebSockets.
5. Wait for explicit health and replay-ready endpoints.
6. Forward child output with component prefixes.
7. Terminate all children and delete the temporary root on exit.

Use environment variables reserved for tests:

```text
ARKNOVA_E2E=1
ARKNOVA_ALLOW_TEST_CONTROLS=1
ARKNOVA_DATA_DIR=<temporary path>
ARKNOVA_CONTENT_PACK=<synthetic pack path>
ARKNOVA_FIXED_NOW_MS=1710504000000
ARKNOVA_DETERMINISTIC_IDS=1
ARKNOVA_GAME_SEED=<fixed seed>
ARKNOVA_RULESET_VERSION=e2e-v1
ARKNOVA_RNG_VERSION=e2e-v1
ARKNOVA_PROJECTION_WORKERS=1
ARKNOVA_UI_BUILD_HASH=e2e-build
```

The production binary must refuse deterministic-ID, fixed-clock, fixture-content,
or reset controls unless `ARKNOVA_E2E=1` and explicit test controls are enabled.
CI must never connect to a developer's existing game directory.

Initially, run scenarios serially with one worker. Parallelism is allowed only
after the harness provides a completely isolated server, ports, data directory,
seed, and browser storage partition per worker.

## 5. Browser and Device Model

One Playwright test creates multiple browser contexts:

- `table`: shared display at the canonical table viewport;
- `seat1` through `seat4`: isolated companion contexts at canonical phone or
  tablet viewports;
- optional `admin`: isolated recovery context.

Contexts must not share cookies, local storage, caches, or credentials. A private
projection leak can be hidden if multiple seats use pages in one context.

Initial canonical projects:

| Project | Viewport | Purpose |
| --- | --- | --- |
| table-1080p | 1920×1080, scale 1 | Primary shared-table visual baseline |
| companion-phone | 393×852, scale 1 | Primary private-device baseline |
| companion-tablet | 820×1180, scale 1 | Targeted tablet layout scenarios |

Do not multiply every scenario across every viewport. Run the complete gameplay
journeys at the primary table and phone sizes, then add focused responsive
scenarios for 4K tables, tablets, and each seat-edge orientation.

## 6. Playwright Configuration

The intended baseline configuration is:

```ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: Boolean(process.env.CI),
  reporter: [['html', { open: 'never' }]],
  timeout: 60_000,
  expect: {
    timeout: 2_000,
    toHaveScreenshot: { maxDiffPixels: 0 }
  },
  use: {
    baseURL: 'http://127.0.0.1:5174',
    trace: 'retain-on-failure',
    timezoneId: 'America/Toronto',
    locale: 'en-CA',
    colorScheme: 'light',
    reducedMotion: 'reduce',
    deviceScaleFactor: 1,
    serviceWorkers: 'block',
    launchOptions: {
      args: [
        '--disable-gpu',
        '--disable-dev-shm-usage',
        '--disable-font-subpixel-positioning',
        '--disable-lcd-text',
        '--font-render-hinting=none',
        '--force-device-scale-factor=1',
        '--use-gl=swiftshader'
      ]
    }
  },
  snapshotPathTemplate:
    '{testDir}/{testFileDir}/screenshots/{arg}.png',
  webServer: {
    command: 'bun run test-server',
    url: 'http://127.0.0.1:5174/api/test/ready',
    reuseExistingServer: false
  }
});
```

CI and baseline updates must use the same pinned Chromium build and fonts from
the Nix environment. Do not regenerate authoritative images using a different
browser revision or operating system.

## 7. Unified Multi-Surface Step Pattern

Tests use one `MultiSurfaceStepHelper` for assertions, screenshots, console
output, and generated documentation. Tests never manage counters, filenames, or
README Markdown directly.

Example:

```ts
const journey = new MultiSurfaceStepHelper(
  { table, seat1, seat2 },
  testInfo
);

journey.setMetadata(
  'Private card, public enclosure',
  'Seat 1 plays a private animal card onto a public table target.'
);

await journey.step('animal-selected', {
  description: 'Seat 1 selected an animal while the table awaits a target.',
  capture: ['table', 'seat1', 'seat2'],
  verifications: [
    {
      spec: 'The table identifies seat 1 without showing the card',
      check: async ({ table }) => {
        await expect(table.getByTestId('pending-seat')).toHaveText('Seat 1');
        await expect(table.getByText('Synthetic Ibex')).toHaveCount(0);
      }
    },
    {
      spec: 'Seat 1 sees the selected private card',
      check: async ({ seat1 }) => {
        await expect(seat1.getByRole('button', { name: 'Synthetic Ibex' }))
          .toHaveAttribute('aria-pressed', 'true');
      }
    },
    {
      spec: 'Seat 2 cannot see seat 1 private card',
      check: async ({ seat2 }) => {
        await expect(seat2.getByText('Synthetic Ibex')).toHaveCount(0);
      }
    }
  ]
});

journey.generateDocs();
```

The helper must:

- log each step, specification, and pass to the terminal;
- run all verifications before screenshots;
- wait for each captured surface to report the same branch and revision;
- name images `NNN-step-id--surface.png`;
- call `expect(page).toHaveScreenshot()` for every captured surface;
- embed the surface images and checked specifications in the scenario README;
- attach branch/revision metadata to failures;
- fail if `generateDocs()` is omitted for a documented scenario.

Actions happen between steps. Verifications and screenshots happen inside steps.
Do not take decorative screenshots that assert nothing.

## 8. Synchronization Rules

Never use time as evidence that the application is ready.

Wait on observable contracts:

- `[data-testid="projection-revision"]` reaches an expected revision;
- `[data-testid="connection-state"]` becomes `connected`;
- a button becomes enabled;
- a pending-decision region appears;
- a job or replay status endpoint reports completion;
- an expected WebSocket message is reflected in the DOM.

Every assertion and explicit wait has a maximum of 2000 ms. The overall test may
remain 60 seconds to cover process startup and many short actions. If a normal UI
transition cannot expose a stable signal in two seconds, improve the app or test
harness rather than extending the wait.

Forbidden:

```ts
await page.waitForTimeout(500);
await expect(locator).toBeVisible({ timeout: 10_000 });
await page.waitForFunction(..., { timeout: 30_000 });
```

Use web-first assertions and stable readiness indicators. Animation is disabled
through reduced motion; screenshots are taken only after projection revision and
layout stability are observable.

## 9. Static E2E Policy Check

`web/scripts/check-e2e-rules.ts` runs locally and in CI. It recursively checks
Playwright config, E2E specs, helpers, and fixture scripts for:

- `waitForTimeout` or sleep helpers;
- explicit waits/assertions above 2000 ms;
- `test.only`, `describe.only`, and skipped scenarios without an allow-list;
- direct `page.screenshot()` or manually numbered screenshot filenames;
- hand-written scenario README output;
- network interception of first-party `/api` routes in full-stack scenarios;
- imports of production/extracted artwork into fixtures;
- accidental `reuseExistingServer: true` in CI configuration.

The checker is included in `check:precommit`, not merely documented:

```json
{
  "scripts": {
    "check:e2e-rules": "bun scripts/check-e2e-rules.ts",
    "check:precommit": "bun run check:e2e-rules && bun run check",
    "test:e2e": "playwright test",
    "test:e2e:update-snapshots": "playwright test --update-snapshots"
  }
}
```

## 10. Fixture Strategy

All committed fixture content is synthetic and visually obvious. Use invented
cards such as `Synthetic Ibex`, placeholder icons, geometric boards, and stable
short text. Fixtures define enough rules to exercise the same engine pathways as
real content without copying protected expression or media.

Each scenario requests a fixture world containing:

- content/rules/RNG versions;
- fixed game and branch identifiers;
- fixed player display names and seat colors;
- a fixed seed and canonical eligible-card pools;
- known action-card positions and resources;
- synthetic maps with deterministic geometry;
- optional prelude input actions replayed through the real engine.

Prefer setup through public UI actions. A scenario may use a test-only fixture
endpoint to establish a long precondition when setup is not the behavior under
test, provided that endpoint creates a canonical game by replaying declared input
actions rather than writing projection state directly.

Never seed the projection database. Never mutate a snapshot to arrange a test.
The fixture must remain reproducible after all projections and snapshots are
deleted.

## 11. Privacy Testing

Every cross-seat scenario includes negative assertions. At minimum:

- the public table does not contain private card IDs, names, or prompts;
- other seats do not contain the active seat's private data;
- public WebSocket frames do not contain private fixture markers;
- console messages and error responses do not reveal private payloads;
- screenshots, traces, generated READMEs, and HTML reports are safe to retain;
- reconnecting with a different seat credential changes the available projection;
- revoked credentials stop receiving updates.

Use unique canary strings in each seat's hidden fixture data and scan serialized
network responses and artifacts for those strings. Prefer an allow-list of the
one context permitted to observe each canary.

## 12. Visual Baseline Policy

Screenshots are committed, authoritative contracts. Every helper step captures
all surfaces materially changed by the preceding action. Zero pixels may differ.

To update a baseline:

1. Run the scenario normally and understand the failure.
2. Inspect the actual UI and Playwright diff; do not approve blindly.
3. Run `bun run test:e2e:update-snapshots -- <scenario>` inside the pinned Nix
   environment.
4. Inspect every changed PNG and generated README.
5. Re-run the same scenario without update mode.
6. Commit screenshots and README with the behavior change.

Do not mask clocks, IDs, paths, player names, or other dynamic regions. Make
their sources deterministic. Do not add CSS that exists only to hide instability.

Fonts, emoji, canvas, WebGL, and GPU output are common sources of pixel drift.
Use repository-provided fonts and deterministic SVG/DOM rendering for baseline
UI. Game media should render through stable browser paths; avoid GPU-dependent
effects in asserted screens.

## 13. Selectors and Touch Interaction

Prefer accessible roles and names:

```ts
page.getByRole('button', { name: 'Confirm build' })
page.getByRole('region', { name: 'Your hand' })
```

Use `data-testid` for exact game-state signals and spatial targets:

```html
<output data-testid="projection-revision">84</output>
<button data-testid="zoo-hex-F7" aria-label="Zoo hex F7"></button>
```

Do not select by CSS implementation classes, generated DOM IDs, full local
paths, or card text when the text is not the behavior under test.

Use real pointer/touch sequences for drag, rotate, pinch, and multi-touch paths.
Also provide and test an accessible non-gesture alternative for essential game
actions. Coordinate assertions use canonical game positions, not fragile raw
screen pixels.

## 14. Initial Scenarios

### 001 Create and pair

- Create a deterministic two-player game on the table.
- Pair two isolated companions using single-use codes.
- Verify seat identity and public/private projections.
- Verify a used or expired pairing code is rejected safely.

### 002 Private card, public target

- Seat 1 chooses a synthetic animal privately.
- The table announces only that seat 1 must choose an enclosure.
- Seat 1 selects a legal public target on the table and confirms.
- All surfaces reach the same revision.
- The public result appears while unrelated hidden cards remain private.

### 003 Reconnect and restore

- Complete several actions.
- Disconnect and recreate one companion context.
- Re-pair or restore its seat credential and verify the exact private revision.
- Restart the server without changing the temporary canonical store.
- Verify table and companions reconstruct identical projections.

### 004 Undo hidden information

- Draw and privately reveal a known synthetic card.
- Propose a rewind to before the draw.
- Verify every affected player sees and accepts the consent prompt.
- Verify a new branch starts at the agreed state.
- Replay the action and verify a different synthetic card is revealed.
- Restart/replay the new branch and verify that replacement card is stable.
- Verify the abandoned branch remains inspectable only through admin tooling.

### 005 Edge orientation and accessibility

- Exercise the same public choice from every supported table edge.
- Verify controls face the acting seat while canonical board geometry is stable.
- Complete the action using keyboard/accessibility controls without gestures.

## 15. CI Contract

The pull-request workflow runs on macOS with Nix:

```console
nix develop --command go test ./...
nix develop --command sh -c 'cd web && bun install --frozen-lockfile'
nix develop --command sh -c 'cd web && bun run check:precommit'
nix develop --command sh -c 'cd web && bun run build'
nix develop --command sh -c 'cd web && bun run test:e2e:install'
nix develop --command sh -c 'cd web && bun run test:e2e'
```

CI never runs snapshot update mode. On failure it uploads:

- Playwright HTML report;
- failed screenshots and pixel diffs;
- traces, filtered for private canary leakage;
- prefixed server/client logs;
- the synthetic input log and version metadata;
- no production media or real saved games.

Generated scenario READMEs must be clean after a successful run. CI should fail
if tests modify tracked walkthroughs, ensuring documentation and test steps stay
synchronized.

## 16. Review Checklist

Every PR that changes user-visible behavior answers:

- Which player journey changed?
- Which rules/replay tests prove the domain behavior?
- Which E2E step proves the public table behavior?
- Which companion surfaces were captured?
- Which negative assertions prove hidden information stayed private?
- Were screenshots intentionally inspected and updated?
- Does the scenario use only synthetic content?
- Are all waits observable and at most 2000 ms?
- Can the fixture be rebuilt from canonical input actions alone?
- Does server restart reproduce the same branch and projections?
