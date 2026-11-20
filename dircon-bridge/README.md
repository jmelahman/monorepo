# dircon-bridge

Relays Bluetooth LE fitness devices, such as smart trainers and heart rate
monitors, to Zwift over Wahoo Direct Connect (DIRCON).

It is for setups where Zwift can't use Bluetooth itself. The main case is
Zwift under Wine: Wine stubs out the Windows Runtime Bluetooth LE APIs, so
Zwift logs `BLE_LOAD_FAIL` and finds nothing. Zwift's Direct Connect support
works over the network instead, so this bridge connects to the devices through
BlueZ and offers each one to Zwift as a Direct Connect device.

## Install

```sh
go install github.com/jmelahman/dircon-bridge@latest
```

Or download a Linux binary from the
[releases](https://github.com/jmelahman/dircon-bridge/releases).

## Use

Wake the devices (pedal the trainer, wear the strap), then run:

```sh
dircon-bridge
```

With no arguments, it scans for a trainer (Fitness Machine or Cycling Power
service) and a heart rate monitor, and bridges each as it is found. Once it has
one, it looks for the other for three more scans and then stops, since
scanning disturbs the connected device. If it finds more than one of a kind,
it lists them and you pass the addresses you want:

```sh
dircon-bridge D6:F8:D9:7F:99:4B F5:B4:81:3D:07:26
```

In Zwift's pairing screen, the devices appear with a Wi-Fi icon. The first
device listens on TCP port 36866, the next on 36867, and so on, in the order
they are given or found (`-port`
changes the first). Zwift must be able to reach those ports and see the mDNS
announcements, so a containerized Zwift needs host networking.

Each device is announced only while it is connected. If its Bluetooth link
drops, the announcement is withdrawn and Zwift's connections are closed. The
bridge reconnects and announces it again on its own.

`-v` logs every message relayed. Stop the bridge with Ctrl-C or SIGTERM; it
disconnects the devices on the way out so they advertise again.

## Requirements

- Linux with BlueZ and its D-Bus API (the default on desktop distributions).
- Permission to use BlueZ over the system bus, which desktop users normally
  have.

## How it works

DIRCON is GATT over TCP: Zwift asks for the services and characteristics,
reads, writes, and turns on notifications, and the bridge does each of those
on the device through BlueZ. Two device quirks are handled along the way:

- Fitness Machine control points reject writes until their indications are on,
  but Zwift never asks for them over DIRCON. The bridge turns them on before
  the first write.
- Zwift resends a write it hasn't heard back about after about a second, and
  BlueZ rejects a write while another is outstanding. The bridge serializes
  writes, and answers a resend of a write that just succeeded without writing
  it again.
