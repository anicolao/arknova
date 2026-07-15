# Manual Test 001 — Create and Connect

## Setup

- Server commit: record before testing
- Server device and OS: record before testing
- Table device, browser, and viewport: record before testing
- Companion devices and browsers: record before testing
- Server URL: record before testing
- Tester and date: record after testing

Run the production build on the intended server device:

```sh
nix develop --command sh -c 'cd web && bun install --frozen-lockfile && bun run build'
nix develop --command go run ./cmd/arknova -listen 0.0.0.0:8080 -data ./data -web ./web/build -public-url http://SERVER-NAME:8080
```

## Procedure

1. On the separate table device, open `http://SERVER-NAME:8080/table`.
2. Start a two-player game and record the displayed game code and revision.
3. Confirm two distinct, readable QR codes appear.
4. Scan player 1's QR code with one physical phone and player 2's with another.
5. Confirm each phone displays the correct player number, game code, empty
   private area, and the same revision as the table.
6. Refresh all three browsers and confirm the same views return.
7. Stop the server process, start it again with the same data directory, then
   refresh all three browsers.
8. Confirm the game returns without reconfiguration or credentials.

## Result

- Status: **not yet executed on physical devices**
- Observations/defects: record during testing
