# dircon-bridge — Agent Notes

Relays BLE fitness devices to Zwift over DIRCON. See README.md for what and
why.

## Commands

- Tests: `go test ./...`.
- Run: `go run . -v [address...]`. Needs BlueZ and a real device.
- Lint: `prek run --all-files`. It is the same set CI runs.

## Layout

- `main.go` — the CLI: flags, device discovery, one bridge per device.
- `internal/dircon/` — DIRCON packet and UUID encoding. Pure; unit tested.
- `internal/bluez/` — a small BlueZ D-Bus client (godbus): discovery,
  connect, GATT read/write/notify, and the PropertiesChanged dispatch.
- `internal/bridge/` — per-device bridge: TCP sessions, mDNS registration
  while connected, write coalescing.
- `internal/version/` — ldflags-stamped version.

## Conventions

- The mDNS service type `_wahoo-fitness-tnp._tcp` is longer than RFC 6763
  allows; brutella/dnssd doesn't check. Don't switch to a library that does.
- The protocol layout follows qdomyos-zwift's
  `src/devices/dircon/dirconpacket.cpp`.
- Tests go in `package <pkg>_test`. Struct fields: wide first, `bool` last.
