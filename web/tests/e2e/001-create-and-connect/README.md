# 001 — Create and Connect

This tracer bullet proves the first playable system boundary: a host creates a
two-player game on the table, both companions join through their stable player
URLs, and the same state returns after the Go server restarts.

## Walkthrough

1. Open `/table` and choose **Start two-player game**.
2. Confirm game code `WILD`, revision 1, and two companion QR codes.
3. Confirm the test-only diagnostic view contains exactly one canonical
   `GameConfigured` action.
4. Open player 1 and player 2 URLs in isolated mobile browser contexts.
5. Confirm each device identifies its seat and shows an empty private hand at
   revision 1.
6. Stop and restart the real server without changing its data directory.
7. Reload all three devices and confirm they recover game `WILD` at revision 1.

The screenshots in this directory are generated from these assertions with
synthetic UI content only.
