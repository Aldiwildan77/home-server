# ESP32 agent — scope, not code (yet)

This is a plan. Nothing here has been compiled or run against real hardware —
I don't have a TinyGo toolchain or an ESP32 in this environment, so I'm not
writing firmware I can't verify. Read this, decide if it's worth building,
and I'll write the actual source once you can test it against a real board.

## Why this can't just be a build target

`backend/` is standard Go. `GOOS=linux GOARCH=arm64 go build` works because Go
ships a real toolchain for those targets. ESP32's chip (Xtensa, or RISC-V on
newer variants) **isn't a Go target at all** — there's no `GOOS=esp32`.

The only way to run Go-like code on ESP32 is [TinyGo](https://tinygo.org/),
which compiles a *subset* of Go. It does not support:

- `os/exec` — no shell, no processes. The backend's whole discovery mechanism
  (`ip neigh`, ping-sweep) is impossible on ESP32.
- `database/sql` / `modernc.org/sqlite` — no filesystem/threading model this
  needs. Not that it matters: ESP32 would always be a leaf, and leaves never
  ran an inventory DB anyway.
- A full `net/http` server — TinyGo's HTTP support is minimal. Not needed
  either: leaves never expose the HTTP API, only `listen_http_addr`-enabled
  nodes do.

What TinyGo *can* plausibly do (untested by me, needs confirming on your
board): `net.UDPConn` over its WiFi stack, `encoding/json`, `crypto/hmac`,
`crypto/sha256`. Those are the only pieces the wire protocol actually needs.

## What a real ESP32 agent would be

Much smaller than `backend/`'s Node — it only ever plays leaf:

- No discovery. It can't run `ip neigh`. Its device list is a short
  hardcoded table (MAC + IP pairs) baked in at compile time, since these
  boards are usually wired to one or two specific devices anyway.
- No relaying. It gossips its own tiny device list up to one configured
  parent (never accepts arbitrary peers, never floods to multiple neighbors)
  and listens for `wake` messages addressed to a MAC it owns.
- Same wire protocol as the backend, so it can join the existing mesh: same
  JSON `udpMessage` shape, same `HMAC-SHA256(psk, node_id)` per-node key
  derivation, same `Hop`/`Sig`/`Timestamp` fields. This is the one part that
  has to match exactly, or the rest of the mesh will just drop its packets
  as unverified (working as designed).
- WiFi credentials and the psk/device table are compile-time constants (a
  small `config.go` you edit and reflash per device) — no config file, no
  filesystem dependency, matches how embedded firmware is normally shipped.

## Before I write any of this

1. Install TinyGo, confirm `tinygo build -target=esp32 ...` works at all on
   your specific board with a trivial "blink" program.
2. Confirm TinyGo's `net` package on your board can open a UDP socket and
   receive broadcast traffic — that's the actual unknown here, not the Go
   syntax.

Once both of those work on real hardware, tell me and I'll write the actual
firmware against your confirmed setup instead of guessing at API surfaces
that might not exist on your target.
