# Pull Request Deployment and Physical-Table Testing

## Purpose

Every trusted pull request should become testable on the real table without an
operator checking out a branch, building the application, choosing a port,
starting a process, changing nginx, deleting data, or recording the tested
revision by hand.

The human should perform only the part that cannot be automated usefully:
interacting with and observing the physical table and companion devices. The
deployment, device entry point, scenario preparation, server maintenance, and
test evidence should be supplied automatically.

This document records the target design and the incremental work shared by this
repository and Kenobi's NixOS configuration.

## Implementation Status

The core per-PR deployment path was implemented and production-validated on
2026-08-17: trusted same-repository PRs run CI, publish the exact verified
artifact, deploy behind a stable PR hostname, and report that commit through
the health endpoint and GitHub deployment. Physical-table workflows and the
remaining acceptance criteria below are still planned. Operational procedure
and rollout evidence live in `ARKNOVA_PREVIEW_OPERATIONS.md` and
`ARKNOVA_PREVIEW_ROLLOUT.md` in Kenobi's configuration repository.

## Original Baseline

The following observations were verified on 2026-08-16 and are included to
make the proposed changes distinguishable from infrastructure that already
exists.

### Application repository

The application already has the right runtime boundary for previews:

- one Go process serves the static Svelte client, API, content pack, health
  endpoint, and WebSocket endpoint from one origin;
- the canonical data is one append-only JSONL action log per game;
- SQLite is a disposable projection that is rebuilt at startup;
- the production-like Playwright harness starts the real Go binary and uses
  isolated table and companion browser contexts;
- the synthetic content pack is safe to serve from an Internet-accessible test
  deployment; and
- the local manual-test supervisor already proves that browser and server
  processes can be coordinated while retaining game data.

The missing pieces are packaging and orchestration. The repository currently
has no GitHub Actions workflows, Actions secrets, environments, required
checks, branch rulesets, or deployable Nix package. Pull request 5 is open with
no status checks.

### Kenobi

Kenobi is now a suitable declarative host on which to add preview support:

- it runs x86-64 NixOS and its active configuration records revision
  `aa7a3b28552f449cf3c78c1b0ec6429c25a0e265` from the clean
  `kenobi-cleanup` branch;
- Nix builds are sandboxed;
- Cloudflare DNS and ACME credentials are encrypted with sops-nix and scoped to
  the `morpheum.dev` zone;
- nginx is the only public HTTP application boundary;
- application services use loopback listeners, immutable releases, persistent
  state directories, and hardened systemd units; and
- the firewall exposes only SSH, HTTP, HTTPS, and the intentional DikuMUD port.

There is no container runtime, self-hosted Actions runner, preview router, or
Ark Nova service. None is needed except the preview-specific router and service
management described below.

The existing Flows deployment establishes useful conventions: immutable
releases under `/opt/<application>/releases/<revision>`, a `current` symlink,
loopback services, nginx proxying, startup checks, and rollback-aware NixOS
activation. Ark Nova should follow those conventions. The fixed-revision,
root-over-SSH Flows script should not be copied directly for arbitrary PRs.

## Target User Experience

For an explicitly trusted same-repository pull request:

1. A push starts automated verification.
2. The exact verified commit is packaged for x86-64 Linux.
3. A trusted deployment workflow installs the artifact on Kenobi.
4. Kenobi health-checks the new process and switches routing atomically.
5. GitHub shows a successful deployment and the canonical URL:
   `https://pr-5.arknova.morpheum.dev/table`.
6. A physical table configured once with
   `https://table.arknova.morpheum.dev` opens the newest successful preview on
   startup, or allows another open preview to be selected.
7. Companion QR codes point to the same canonical PR hostname as the table.
8. A table-only operator drawer can create a fresh test epoch, restart the
   server, rebuild SQLite from JSONL, and submit the physical-test result.
9. Closing the PR removes it from routing automatically and later expires its
   releases, state, and logs.

No preview should ever be identified only as "latest." The UI, GitHub
deployment, logs, and physical-test report must all display the full commit SHA.

"Same repository" is necessary but is not, by itself, a trust decision. The
initial policy may treat branches pushed by the sole repository administrator
as trusted. If write access expands, deployment must additionally require a
base-branch-controlled approval label or protected GitHub Environment approval.
Regardless of author trust, the preview process is arbitrary pre-merge code and
must be isolated from Kenobi's other services, private network, and credentials.

## Target Architecture

```text
GitHub pull_request workflow (untrusted, no deployment secret)
    |
    +-- Go tests
    +-- Svelte checks and production build
    +-- multi-device Playwright tests
    +-- x86-64 Linux release and deployment envelope
    |
    v
GitHub workflow_run deployment (trusted workflow from main)
    |
    +-- verify successful run, repository, PR, and head SHA
    +-- transfer the exact artifact using a restricted SSH identity
    v
Kenobi preview deployment helper
    |
    +-- immutable release: /opt/arknova-previews/releases/<sha>
    +-- deployment state:  /var/lib/arknova-previews/pr-<number>/<epoch>
    +-- blue/green unit:   arknova-preview@<deployment-id>.service
    +-- route registry:    pr-<number> -> active deployment
    v
nginx wildcard TLS virtual host -> preview router -> Go application
```

GitHub-hosted runners should perform verification and packaging. A
self-hosted runner would let PR jobs consume resources on an Internet-facing
home server and would enlarge the trusted computing base without improving the
physical test.

## Hostnames, DNS, and TLS

Add these records to the existing Cloudflare-managed zone:

- `arknova.morpheum.dev` for the preview index;
- `table.arknova.morpheum.dev` for the permanent tabletop launcher; and
- `*.arknova.morpheum.dev` for canonical PR origins.

Add `arknova.morpheum.dev` to Kenobi's existing dynamic-DNS service. Prefer
static CNAME records from `table.arknova.morpheum.dev` and
`*.arknova.morpheum.dev` to that dynamic name, subject to Cloudflare's wildcard
record validation, so only one address record needs dynamic maintenance. Add an
ACME certificate for `arknova.morpheum.dev` with
`*.arknova.morpheum.dev` as an extra name, using the existing restricted
Cloudflare DNS credential. This is a one-time NixOS configuration change; a PR
deployment must never modify DNS or request a certificate.

Use a subdomain per PR instead of a path prefix. The client intentionally uses
root-relative `/api`, `/content`, `/ws`, `/table`, and `/play` paths, and the
WebSocket origin check expects the browser and server to share a host. A
canonical subdomain preserves those contracts and keeps local storage isolated
between PRs.

Nginx should have one static wildcard virtual host. It should preserve `Host`,
forward WebSocket upgrades, apply request-body and game-creation rate limits,
and proxy only to a loopback preview router. The firewall remains unchanged;
preview ports must never be added to `allowedTCPPorts`.

## Preview Router and Table Launcher

Nginx cannot safely consume configuration fragments written by a deployment
user. Such fragments could proxy unrelated loopback services or weaken the
public boundary. Instead, add a small infrastructure-owned preview router with
a narrow registry:

```text
pr-5.arknova.morpheum.dev -> <active-deployment-id>
<deployment-id>           -> systemd-activated listener
```

Only the root-owned deployment helper may update the registry. The router must:

- accept only `pr-<positive integer>.arknova.morpheum.dev` hosts;
- return 404 for an absent or expired PR;
- support HTTP streaming and WebSocket upgrades;
- expose no arbitrary upstream selection API;
- reload an atomically replaced registry without restarting nginx; and
- provide an index containing PR number, title, SHA, deployment time, and
  health, but no application data.

The router must also support an internal-only candidate probe. Deployment first
starts a new deployment ID without changing the public PR route, checks the
candidate through that restricted probe, atomically changes the registry, and
then checks the public HTTPS origin. If the public check fails, it restores the
previous registry entry before stopping either process.

The table launcher should be a small page served by the router at
`table.arknova.morpheum.dev`. It selects the newest healthy preview when the
table starts and presents all other healthy previews. It must not switch a
running game merely because another PR deploys. A new deployment may show a
notification, but switching an active table requires an explicit selection or
the next launcher reload.

The launcher can place the canonical PR table in a full-screen iframe. That
keeps the kiosk on a permanent URL while the application still sees its
canonical PR origin, so generated companion URLs and browser storage remain
correct. The application must explicitly allow framing by the trusted launcher
origin and deny unrelated framing.

## Artifact Contract

Add a reproducible x86-64 Linux package to this repository's flake. The
SHA-addressed release payload must contain only deterministic source outputs:

```text
bin/arknova
web/index.html and hashed static assets
content/synthetic/...
build.json
```

`build.json` must include only source-derived fields:

- repository name;
- complete commit SHA;
- Go, Bun, and content-pack versions; and
- artifact format version.

CI wraps that release in a deployment envelope containing:

- pull request number;
- source run ID and run attempt;
- head repository;
- complete commit SHA;
- packaging timestamp; and
- a digest of the deterministic release payload.

The envelope is not part of the release identity. This keeps
`releases/<sha>` reusable if the same commit is the head of more than one pull
request and avoids calling run-specific bytes reproducible.

Build the Go executable with `CGO_ENABLED=0` and fail packaging if it is not a
static x86-64 Linux executable. Build the web client with the committed Bun
lockfile. Do not include `steam-assets`, local data, Playwright traces,
credentials, or any unlicensed content.

The application should gain a read-only build-information endpoint. Its
response must come from the installed `build.json` and should be included in
`/healthz`. Readiness already has a useful property: replay finishes before the
HTTP listener starts. Deployment should nevertheless verify the reported SHA,
load `/table`, create a game, establish a WebSocket, and load synthetic content
through the internal candidate probe before switching the route. It then repeats
a non-destructive smoke check through the canonical public origin.

## Kenobi Service Model

Add a NixOS module to the Kenobi configuration rather than installing ad hoc
units. It should declare:

- the wildcard ACME certificate and nginx virtual hosts;
- `arknova-preview-router.service`;
- the `arknova-preview@.service` template;
- a non-wheel, non-interactive `arknova-deploy` SSH identity;
- the forced-command deployment helper; and
- state, release, log, and runtime directories with explicit ownership.

Each application instance runs pre-merge code and therefore needs a stronger
boundary than the trusted Flows and Dikuclient services:

- a systemd-owned listener passed through socket activation;
- `PrivateNetwork=true`, with no process-created network sockets or access to
  Kenobi loopback, LAN, or Internet services;
- `DynamicUser=true` where compatible with the persistent state layout;
- `NoNewPrivileges=true`;
- `PrivateTmp=true`;
- `PrivateDevices=true`;
- `ProtectHome=true`;
- `ProtectSystem=strict`;
- an empty capability bounding set and restrictive syscall/address-family
  policy compatible with Go, SQLite, and the inherited listener;
- a narrowly writable `StateDirectory`;
- automatic restart on failure;
- startup and readiness checks;
- `MemoryMax`, `TasksMax`, and CPU limits; and
- journald fields identifying PR and SHA.

There is currently no swap on Kenobi, so memory limits are required even though
the machine presently has ample free memory. Limit the number of simultaneously
active previews and expire closed PRs promptly.

Use blue/green deployments. A deployment ID identifies a PR, full SHA, and
unique slot or nonce. The candidate gets a distinct socket and unit, so a failed
start or health check cannot interrupt the active deployment. After an atomic
route switch and public check, gracefully drain and stop the former unit. Keep
enough metadata to reverse that switch during the retention period.

The service should run with normal random IDs and production behavior. Do not
set `ARKNOVA_E2E`, `ARKNOVA_ALLOW_TEST_CONTROLS`, deterministic-ID, fixed-clock,
or fixed-seed variables on an Internet-accessible preview.

Before adding automated deployment, add graceful signal handling to the Go
entry point. `SIGTERM` should stop accepting requests, close WebSockets, finish
or reject in-flight actions, close SQLite, and exit within the systemd stop
timeout. This makes supervisor-driven restart a meaningful persistence test
rather than an uncontrolled process kill.

## Restricted Deployment Interface

Create a dedicated SSH key for GitHub Actions and store only its private half as
the repository secret `KENOBI_PREVIEW_SSH_KEY`. Pin Kenobi's SSH host key in a
checked, reviewed `known_hosts` file; it is public identity material, not a
secret. Add the public key only to
`arknova-deploy`, with a forced command and with PTY, forwarding, agent
forwarding, and arbitrary shell execution disabled.

The SSH forced command runs unprivileged and may invoke exactly one root-owned
helper through an exact passwordless sudo rule. The helper, its arguments, and
its standard input are the complete privilege boundary: the deploy user receives
no general `systemctl`, filesystem, or shell permission.

The forced command should accept only these operations:

```text
deploy <pr-number> <40-hex-sha> <artifact-size>   # artifact on stdin
restart <pr-number> <40-hex-sha>
rebuild-projection <pr-number> <40-hex-sha>
fresh-epoch <pr-number> <40-hex-sha>
remove <pr-number>
status <pr-number>
```

The helper must parse arguments without shell evaluation, cap artifact size,
verify the envelope, payload digest, and `build.json`, reject path traversal and
symlinks in the archive, and refuse metadata whose PR or SHA differs from the
command. It may invoke only the corresponding systemd instance and may update
only that PR's release, state, and route records.

The helper installs to a partial directory, changes ownership and modes,
renames it atomically to `/opt/arknova-previews/releases/<sha>`, allocates a new
deployment ID and listener, starts the candidate backend, and performs internal
health checks. It atomically switches the route only after success, verifies the
public route, and rolls the registry back on failure. The previous healthy
backend remains running until that completes. Complete release directories are
immutable and are never overwritten.

Do not use the current personal `anicolao` or emergency `root` SSH access from
Actions. Both cross the wheel/root trust boundary documented in Kenobi's
operations policy.

## GitHub Workflows

### `ci.yml`

Trigger on `pull_request` and run without secrets with read-only repository
permissions:

1. check formatting and the E2E static policy;
2. run all Go tests;
3. run Svelte checks and the production build;
4. run the canonical Playwright suite;
5. run a Linux portability job;
6. build the deployment artifact; and
7. upload reports on failure and the deployment artifact on success.

Pin every third-party Action by full commit SHA. Use the Nix lock and Bun lock
as the toolchain and dependency authority. The visual-baseline job should stay
on the platform defined by `E2E_GUIDE.md`; the deployment package is a separate
x86-64 Linux job.

### `deploy-preview.yml`

Use `workflow_run` so the workflow containing the SSH operation always comes
from the default branch rather than from the PR. It must:

1. accept only a successful completed `ci.yml` run;
2. resolve the associated open PR and exact tested head SHA;
3. verify that the head repository is `anicolao/arknova`;
4. download only that run's artifact;
5. verify its deployment envelope, payload digest, and `build.json` before
   transfer;
6. deploy with concurrency group `preview-pr-<number>`;
7. after acquiring concurrency, query the PR again and skip the run unless its
   current head is still the tested SHA;
8. test the canonical public HTTPS URL; and
9. create or update a GitHub Deployment/environment URL and one sticky PR
   comment.

The current-head check is mandatory even with concurrency: an older slow run
can otherwise finish after a newer run and roll the preview backward.

Never use `pull_request_target` to check out or execute PR code. Fork PRs should
receive normal unprivileged CI but no preview. Supporting a fork requires an
explicit trust decision and a base-branch-controlled mechanism; it cannot be
both safe and entirely automatic.

`workflow_run` only operates after this workflow exists on the default branch.
Land the first version disabled behind a base-branch repository variable, merge
it, enable it only after Kenobi is ready, and validate the complete lifecycle on
a subsequent canary PR. Do not run a secret-bearing copy from its own PR branch.

### `remove-preview.yml`

On `pull_request.closed`, invoke `remove <pr-number>`, deactivate the GitHub
deployment, and remove the PR from the launcher. Keep its immutable release,
state, and logs for a short retention period, initially seven days.

Also run a daily base-branch-controlled reconciliation job. It should compare
Kenobi's registry with GitHub's open PRs, remove orphan routes, delete expired
state, and keep a small number of recent releases for rollback. Cleanup must be
idempotent because GitHub delivery and SSH connections can fail independently.

## State and Release Lifecycle

Use a fresh state epoch for each newly deployed commit by default. Replaying a
new event schema against data created by an earlier commit is a separate
compatibility test and should not happen accidentally.

Within one deployed SHA:

- a systemd restart preserves JSONL and SQLite;
- projection rebuild preserves JSONL and removes only SQLite;
- fresh epoch creates a new empty state directory without destroying the old
  one; and
- rollback selects the prior release and its matching state epoch.

This supports restart and projection-rebuild scenarios while ensuring that a
force-push or updated PR does not inherit surprising browser-test data. Old
epochs remain read-only until retention cleanup.

The route switch, release selection, state selection, and reported build metadata
must be one coherent transaction from the operator's perspective. A browser
must never load web assets from one SHA while sending actions to another.

## Physical-Test Operator Support

The current manual procedures mix valuable human observations with mechanical
server operations. Separate them.

Add a preview-only operator API to the infrastructure router, not to the normal
game API. Protect it with a credential stored through sops-nix and provisioned
once in the table launcher. Companion devices do not need this credential.

The table operator drawer should:

- show PR, SHA, deployment time, content version, server health, and current
  state epoch;
- discover manual scenarios shipped in `docs/manual/`;
- create a fresh epoch;
- restart the exact deployed SHA;
- rebuild its projection database from JSONL;
- show connected table/companion user agents, viewports, revisions, and
  connection states;
- record observations and defects; and
- submit pass, fail, or blocked against the exact SHA.

The router may execute only the restricted helper operations. It must not
expose shell commands, file paths, logs containing private projections, or E2E
diagnostic endpoints.

A submitted run should update a `physical-table` GitHub Check attached to the
tested commit and link to an immutable report containing:

- scenario and procedure version;
- PR and full SHA;
- table and companion browser/device metadata;
- start and finish timestamps;
- observed revisions and reconnect events;
- human observations and defects; and
- tester identity.

Storing evidence against the tested SHA avoids committing a result to the PR
and thereby changing the SHA that was tested. The curated manual document may
link to the result after merge; the check is the merge-time evidence.

Use a dedicated GitHub App for this callback, with repository access limited to
`anicolao/arknova` and only the metadata/read and Checks/write permissions it
needs. Store its private key and installation identifiers through sops-nix.
Do not place a personal access token in the launcher, browser storage, preview
application, or game data. If the GitHub App is deferred, store the result on
Kenobi and let a base-branch scheduled workflow retrieve it through the
restricted SSH interface; the browser must never hold a GitHub credential.

The application itself remains publicly reachable because it serves only
synthetic content and non-secret games. Operator endpoints remain authenticated.
Nginx and the router should cap request sizes, rate-limit game creation, cap
games and disk per preview, and return a clear capacity error rather than
allowing public requests to exhaust Kenobi.

## Repository Rules After Rollout

Once CI and deployment are proven, add a GitHub ruleset for `main` requiring:

- `ci/go`;
- `ci/web`;
- `ci/e2e`;
- `ci/linux-package`;
- `preview/kenobi`; and
- `physical-table` when runtime or player-facing paths require a physical test.

The `physical-table` workflow should report success as "not required" for
changes that cannot affect runtime behavior, rather than leaving a required
check pending forever. Initially classify changes to `cmd/`, `internal/`,
`content/`, `web/src/`, and manual scenario definitions as requiring physical
verification; refine the policy from real usage.

Draft PRs may deploy and collect physical evidence, but only a ready PR with all
required checks may merge.

## Implementation Sequence

Keep one-time host work separate from routine application deployment.

### 1. Application readiness PR

In this repository:

- add graceful shutdown;
- accept a systemd-activated listener while retaining the explicit listen
  address used by development and tests;
- expose build/readiness metadata;
- add the reproducible Linux release and deployment envelope;
- add deployment-specific unit and packaging tests; and
- add `ci.yml` without deployment credentials.

Done when a GitHub-hosted run produces an artifact that starts on an x86-64
NixOS test VM or equivalent isolated Linux test and passes the public-origin
smoke journey.

### 2. Kenobi infrastructure PR

In `anicolao/nix-darwin-config`:

- add wildcard DNS/ACME declarations;
- add the router, launcher, systemd template, deployment helper, user, and
  hardening;
- test the generated nginx and systemd configuration; and
- activate using the rollback-protected procedure in `KENOBI_OPERATIONS.md`.

This remains a reviewed NixOS activation. Routine Ark Nova PRs must not rebuild
or switch the host configuration.

### 3. Deployment workflow bootstrap PR

In this repository:

- add the base-branch-controlled deploy and cleanup workflows, initially gated
  by a disabled repository variable;
- merge them to the default branch;
- add the restricted SSH secrets only after the host endpoint is active; and
- enable preview deployment.

The bootstrap PR cannot prove its own `workflow_run` path because that workflow
is not trusted default-branch code until after merge.

### 4. Canary lifecycle

Update a harmless open application PR after the bootstrap workflow is enabled,
then:

- deploy its successful CI artifact;
- exercise update, failed health check, rollback, restart, close, and janitor
  paths; and
- publish the GitHub Deployment URL and sticky PR comment.

Done when updating a PR replaces only its preview, a failed release never
becomes public, and closing the PR removes its route without an SSH session.

### 5. Physical-test workflow PR

After basic preview deployment is stable, add the operator credential through
sops-nix, operator drawer, scenario metadata, device telemetry, immutable
report, and GitHub Check integration. Convert the shell-oriented steps in
`MANUAL_TESTING.md` and `docs/manual/` into human observations plus operator
actions.

Done when the table is configured only with the permanent launcher URL and a
tester can complete, record, restart, rebuild, and submit a scenario without
using a development machine or terminal.

### 6. Enforcement and capacity tuning

Enable the main-branch ruleset after several successful preview lifecycles.
Measure build time, process memory, disk growth, and number of concurrent open
PRs before selecting final retention and resource limits.

## Acceptance Criteria

The deployment system is complete when all of these are demonstrated:

1. Opening or updating a trusted PR deploys the exact successful CI artifact.
2. The canonical URL and `/healthz` report the same complete SHA.
3. The real table needs only its permanent launcher configuration.
4. Companion QR codes remain pinned to the selected PR origin.
5. Two PRs can run simultaneously without sharing processes, browser storage,
   game logs, SQLite, content, or ports.
6. A failed build or health check leaves the last healthy preview in service.
7. Restart and projection rebuild are available from the operator drawer and
   preserve the expected data boundary.
8. No preview backend port is reachable through the public firewall.
9. Actions cannot obtain an administrative shell or modify unrelated Kenobi
   services.
10. Fork PRs cannot access deployment credentials.
11. Closing a PR removes routing automatically, and reconciliation cleans a
    deliberately orphaned preview.
12. A physical result is attached to the exact tested commit and can gate the
    merge without adding another commit.
13. A stale successful workflow run cannot replace a newer PR preview.
14. Preview code cannot reach Kenobi's other loopback services, LAN, Internet,
    administrative interfaces, or credentials.

## Deliberate Non-Goals

- Do not deploy real or extracted Ark Nova artwork.
- Do not make Kenobi a general-purpose CI runner.
- Do not let application PRs modify Kenobi's NixOS configuration.
- Do not expose E2E deterministic controls on public previews.
- Do not preserve games implicitly across different PR commits.
- Do not automate the human judgment that physical-table testing exists to
  collect.
