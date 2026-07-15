# Manual Test 001 — Create and Connect

## Setup

- Server commit: record before testing
- Server device and OS: record before testing
- Table device, browser, and viewport: record before testing
- Companion devices and browsers: record before testing
- Server URL: record before testing
- Tester and date: record after testing

Start the shared manual-testing environment as described in
[MANUAL_TESTING.md](../../MANUAL_TESTING.md):

```sh
ARKNOVA_DEV_ORIGIN=http://SERVER-NAME:5173 \
  nix develop --command bun run scripts/dev.ts
```

## Procedure

1. On the separate table device, open `http://SERVER-NAME:5173/table`.
2. Start a two-player game and record the displayed game code and revision.
3. Confirm two distinct, readable QR codes appear.
4. Scan player 1's QR code with one physical phone and player 2's with another.
5. Confirm each phone displays the correct player number, game code, empty
   private area, and the same revision as the table.
6. Refresh all three browsers and confirm the same views return.
7. Trigger a Go restart by saving a Go source file without changing its behavior,
   wait for the watcher to report that the server is listening, then refresh all
   three browsers.
8. Confirm the game returns without reconfiguration or credentials.

## Result

- Status: **not yet executed on physical devices**
- Observations/defects: record during testing
