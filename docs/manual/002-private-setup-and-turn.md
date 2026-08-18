# Manual Test 002 — Private Setup and Turn

## Setup

- Server commit: record before testing
- Content pack and version: `content/synthetic`, `synthetic-v1`
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

Use a fresh game. Existing Increment 001 games remain replayable as their
original empty projections and are not silently upgraded into dealt games.

## Procedure

1. Open `/table`, start a two-player game, and connect two physical companions.
2. Confirm the table shows six public synthetic cards and five action-card slots
   for each player.
3. From both supported player edges, confirm the action names, strengths, active
   player, and X-token counts are readable.
4. Confirm each companion shows three private cards and compare the devices:
   neither hand may appear on the table or the other companion.
5. On the table, have player 1 select Animals at strength 3 to take an X-token.
6. Confirm Animals moves to strength 1, Cards and Build shift right, player 1 has
   one X-token, player 2 becomes active, and every device reaches revision 2.
7. Continue alternating X-token turns for at least two complete rounds and
   confirm only the active player's controls are enabled.
8. Stop the server, delete only `data/manual/projections.sqlite`, restart, and
   refresh every browser.
9. Confirm the private hands, action-card order, X-tokens, history, active player,
   and revision exactly match the state before shutdown.

## Result

- Status: **not yet executed on physical devices**
- Observations/defects: record during testing
