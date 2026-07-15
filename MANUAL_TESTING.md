# Manual Testing

Manual testing uses one stable browser origin backed by two automatically
refreshing development processes:

- Vite serves the table and companion web clients, including browser hot reload.
- The supervisor rebuilds and restarts the Go server whenever `cmd/` or
  `internal/` Go source changes.
- Vite proxies `/api`, `/healthz`, and `/ws` to Go, so every device uses the same
  table URL and companion QR origin.
- Game data persists under `data/manual/` across Go restarts.

## Start the shared environment

From a clean, up-to-date test branch or commit, install the pinned dependencies:

```sh
nix develop --command sh -c 'cd web && bun install --frozen-lockfile'
```

Determine a hostname or LAN address that the table and phones can reach. Then
start both development servers with that public origin:

```sh
ARKNOVA_DEV_ORIGIN=http://YOUR-MAC.local:5173 \
  nix develop --command bun run scripts/dev.ts
```

The command prints the checked-out Git commit, whether the worktree is dirty,
the table URL, backend URL, and persistent data directory. Open the printed
`/table` URL on the table device. QR codes use `ARKNOVA_DEV_ORIGIN`, so scanned
companion URLs point back to Vite rather than the loopback-only Go process.

The default backend is `http://127.0.0.1:8081`. Override it only when that port
is unavailable:

```sh
ARKNOVA_DEV_ORIGIN=http://YOUR-MAC.local:5173 \
ARKNOVA_DEV_API_ORIGIN=http://127.0.0.1:9081 \
  nix develop --command bun run scripts/dev.ts
```

## Test a change

1. Record the printed commit and dirty/clean status in the scenario's manual
   test document.
2. Perform the scenario on the physical table and companion devices.
3. Edit web code as needed; Vite updates connected browsers automatically.
4. Edit Go code as needed; the watcher rebuilds and restarts Go while preserving
   `data/manual/`. Refresh a browser after the server reports it is listening.
5. Repeat the scenario from its first step after every fix. Partial retesting is
   diagnostic evidence, not a completed manual run.
6. Run the automated checks before recording a pass.
7. Record devices, browsers, observations, defects, tester, date, and the final
   commit. A pass against uncommitted changes must be repeated after committing.

Stop both processes together with `Ctrl-C`. Delete `data/manual/` only when the
scenario explicitly requires a fresh store; otherwise retained data is useful
for restart and recovery testing.
