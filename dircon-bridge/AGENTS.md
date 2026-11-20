# dircon-bridge — Agent Notes

Relays BLE fitness devices to Zwift over DIRCON. See README.md for what and
why.

## Commands

- Tests: `go test ./...`.
- Run: `go run . -v [address...]`. Needs BlueZ and a real device.
- Lint: `prek run --all-files`. It is the same set CI runs.

## Layout

- `main.go` — the CLI: flags, scanning for devices (`find`), one bridge per
  device, and shutdown order (bridges withdraw their mDNS records before the
  responder stops).
- `internal/dircon/` — DIRCON packet and UUID encoding. Pure; unit tested.
- `internal/bluez/` — a small BlueZ D-Bus client (godbus): discovery,
  connect, GATT read/write/notify, and the signal dispatch. Needs a real
  bus, so only address parsing is unit tested.
- `internal/bridge/` — per-device bridge: TCP sessions, mDNS registration
  while connected, write serialization. It talks to the device through the
  `Device` and `Characteristic` interfaces (`FromBlueZ` adapts BlueZ), so the
  tests drive it with fakes over real TCP.
- `internal/version/` — ldflags-stamped version.

## Conventions

- The mDNS service type `_wahoo-fitness-tnp._tcp` is longer than RFC 6763
  allows; brutella/dnssd doesn't check. Don't switch to a library that does.
- The protocol layout follows qdomyos-zwift's
  `src/devices/dircon/dirconpacket.cpp`.
- BlueZ signals are delivered on one goroutine shared by every device;
  callbacks from it (notifications, drops) must never block. Sessions queue
  their sends for this reason.
- Tests go in `package <pkg>_test`. Struct fields: wide first, `bool` last.
