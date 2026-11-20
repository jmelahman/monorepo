#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "bleak>=1.0",
#     "zeroconf>=0.140",
# ]
# ///
"""Bridges a Bluetooth trainer to Zwift over Wahoo Direct Connect (DIRCON).

Zwift under Wine can't use Bluetooth: Wine's WinRT Bluetooth LE classes are
stubs, so Zwift's BLE DLL fails to load. Zwift's Direct Connect support works,
though, and DIRCON is just GATT over TCP. This connects to the trainer through
BlueZ, advertises it over mDNS as `_wahoo-fitness-tnp._tcp`, and relays
service discovery, reads, writes and notifications between the two.

Run Zwift with host networking (NETWORKING=host) so it sees the mDNS
announcement, then pair the trainer from Zwift's pairing screen.

The packet layout follows qdomyos-zwift's src/devices/dircon/dirconpacket.cpp:
a six-byte header (version, message id, sequence number, response code,
big-endian payload length) followed by 16-byte big-endian UUIDs and values.
"""

from __future__ import annotations

import argparse
import asyncio
import contextlib
import logging
import socket
import struct
from typing import TYPE_CHECKING
import uuid

from bleak import BleakClient
from bleak import BleakScanner
from zeroconf import IPVersion
from zeroconf import ServiceInfo
from zeroconf.asyncio import AsyncZeroconf

if TYPE_CHECKING:
    from bleak.backends.characteristic import BleakGATTCharacteristic

log = logging.getLogger("dircon-bridge")

HEADER = struct.Struct(">BBBBH")
UUID_LEN = 16

MSG_DISCOVER_SERVICES = 0x01
MSG_DISCOVER_CHARACTERISTICS = 0x02
MSG_READ_CHARACTERISTIC = 0x03
MSG_WRITE_CHARACTERISTIC = 0x04
MSG_ENABLE_NOTIFICATIONS = 0x05
MSG_NOTIFICATION = 0x06
MSG_UNKNOWN_07 = 0x07

RESP_SUCCESS = 0x00
RESP_UNKNOWN_MESSAGE = 0x01
RESP_UNEXPECTED_ERROR = 0x02
RESP_SERVICE_NOT_FOUND = 0x03
RESP_CHARACTERISTIC_NOT_FOUND = 0x04
RESP_OPERATION_NOT_SUPPORTED = 0x05
RESP_WRITE_FAILED = 0x06

PROP_READ = 0x01
PROP_WRITE = 0x02
PROP_NOTIFY = 0x04
PROP_INDICATE = 0x08

# Generic Access and Generic Attribute belong to the BLE link, not the trainer.
HIDDEN_SERVICES = {
    uuid.UUID("00001800-0000-1000-8000-00805f9b34fb"),
    uuid.UUID("00001801-0000-1000-8000-00805f9b34fb"),
}

# Fitness Machine and Cycling Power: what a smart trainer advertises.
TRAINER_SERVICES = {
    "00001826-0000-1000-8000-00805f9b34fb",
    "00001818-0000-1000-8000-00805f9b34fb",
}


def dircon_properties(char: BleakGATTCharacteristic) -> int:
    props = set(char.properties)
    flags = 0
    if "read" in props:
        flags |= PROP_READ
    if props & {"write", "write-without-response"}:
        flags |= PROP_WRITE
    if "notify" in props:
        flags |= PROP_NOTIFY
    if "indicate" in props:
        flags |= PROP_INDICATE
    return flags


def packet(msg_id: int, seq: int, payload: bytes = b"", code: int = RESP_SUCCESS) -> bytes:
    if code != RESP_SUCCESS:
        payload = b""
    return HEADER.pack(1, msg_id, seq & 0xFF, code, len(payload)) + payload


class Trainer:
    """The BLE side: one connection to the trainer, kept alive across drops."""

    def __init__(self, address: str) -> None:
        self.address = address
        self.name = address
        self.client: BleakClient | None = None
        self.connected = asyncio.Event()
        self.services: dict[uuid.UUID, list[BleakGATTCharacteristic]] = {}
        self.chars: dict[uuid.UUID, BleakGATTCharacteristic] = {}
        # Characteristic -> sessions that enabled notifications on it.
        self.subscribers: dict[uuid.UUID, set[Session]] = {}
        self._notifying: set[uuid.UUID] = set()
        self._write_lock = asyncio.Lock()
        self._writes: dict[tuple[str, bytes], asyncio.Task[None]] = {}

    async def write(self, char: BleakGATTCharacteristic, data: bytes) -> None:
        # BlueZ fails a write while another is outstanding, and a write with
        # response takes about a second. Zwift resends one it hasn't heard back
        # about, so let the resend share the original instead of colliding.
        key = (char.uuid, data)
        if key not in self._writes:
            self._writes[key] = asyncio.create_task(self._write(char, data))
            self._writes[key].add_done_callback(lambda _: self._writes.pop(key, None))
        await asyncio.shield(self._writes[key])

    async def _write(self, char: BleakGATTCharacteristic, data: bytes) -> None:
        async with self._write_lock:
            assert self.client is not None
            response = "write" in char.properties
            await self.client.write_gatt_char(char, data, response=response)

    async def run(self) -> None:
        while True:
            try:
                await self._connect_once()
            except Exception as e:  # noqa: BLE001 - keep retrying whatever BlueZ throws
                log.warning("trainer connection failed: %s", e)
            self.connected.clear()
            self._notifying.clear()
            await asyncio.sleep(3)

    async def _connect_once(self) -> None:
        log.info("scanning for %s (pedal or wake the trainer if it is asleep)", self.address)
        device = await BleakScanner.find_device_by_address(self.address, timeout=30)
        if device is None:
            raise RuntimeError("not advertising")
        self.name = device.name or self.address
        disconnected = asyncio.Event()
        client = BleakClient(device, disconnected_callback=lambda _: disconnected.set())
        async with client:
            self.client = client
            self.services = {}
            self.chars = {}
            for service in client.services:
                service_uuid = uuid.UUID(service.uuid)
                if service_uuid in HIDDEN_SERVICES:
                    continue
                self.services[service_uuid] = list(service.characteristics)
                for char in service.characteristics:
                    self.chars[uuid.UUID(char.uuid)] = char
            log.info(
                "connected to %s with services %s",
                self.name,
                ", ".join(str(s) for s in self.services),
            )
            for char_uuid, sessions in self.subscribers.items():
                if sessions:
                    await self._start_notify(char_uuid)
            self.connected.set()
            await disconnected.wait()
            log.warning("trainer disconnected")
        self.client = None

    async def _start_notify(self, char_uuid: uuid.UUID) -> None:
        if char_uuid in self._notifying or self.client is None:
            return

        def relay(_: BleakGATTCharacteristic, data: bytearray) -> None:
            payload = char_uuid.bytes + bytes(data)
            for session in list(self.subscribers.get(char_uuid, ())):
                session.send(packet(MSG_NOTIFICATION, 0, payload))

        await self.client.start_notify(self.chars[char_uuid], relay)
        self._notifying.add(char_uuid)

    async def subscribe(self, session: Session, char_uuid: uuid.UUID, *, enable: bool) -> None:
        sessions = self.subscribers.setdefault(char_uuid, set())
        if enable:
            sessions.add(session)
            await self._start_notify(char_uuid)
        else:
            sessions.discard(session)

    def drop(self, session: Session) -> None:
        for sessions in self.subscribers.values():
            sessions.discard(session)


class Session:
    """The DIRCON side: one TCP connection from Zwift."""

    def __init__(
        self, trainer: Trainer, reader: asyncio.StreamReader, writer: asyncio.StreamWriter
    ) -> None:
        self.trainer = trainer
        self.reader = reader
        self.writer = writer
        self.peer = writer.get_extra_info("peername")

    def send(self, data: bytes) -> None:
        if not self.writer.is_closing():
            self.writer.write(data)

    async def run(self) -> None:
        log.info("Zwift connected from %s", self.peer)
        try:
            while True:
                header = await self.reader.readexactly(HEADER.size)
                _, msg_id, seq, _, length = HEADER.unpack(header)
                body = await self.reader.readexactly(length)
                response = await self.handle(msg_id, seq, body)
                self.send(response)
                await self.writer.drain()
        except (asyncio.IncompleteReadError, ConnectionError):
            pass
        finally:
            self.trainer.drop(self)
            self.writer.close()
            log.info("Zwift disconnected from %s", self.peer)

    async def handle(self, msg_id: int, seq: int, body: bytes) -> bytes:  # noqa: C901, PLR0911, PLR0912
        trainer = self.trainer
        try:
            await asyncio.wait_for(trainer.connected.wait(), timeout=10)
        except TimeoutError:
            return packet(msg_id, seq, code=RESP_UNEXPECTED_ERROR)

        if msg_id == MSG_DISCOVER_SERVICES:
            log.debug("discover services")
            return packet(msg_id, seq, b"".join(u.bytes for u in trainer.services))

        if msg_id == MSG_UNKNOWN_07:
            return packet(msg_id, seq)

        if len(body) < UUID_LEN:
            return packet(msg_id, seq, code=RESP_UNEXPECTED_ERROR)
        target = uuid.UUID(bytes=body[:UUID_LEN])
        data = body[UUID_LEN:]

        if msg_id == MSG_DISCOVER_CHARACTERISTICS:
            chars = trainer.services.get(target)
            if chars is None:
                return packet(msg_id, seq, code=RESP_SERVICE_NOT_FOUND)
            log.debug("discover characteristics of %s", target)
            payload = target.bytes + b"".join(
                uuid.UUID(c.uuid).bytes + bytes([dircon_properties(c)]) for c in chars
            )
            return packet(msg_id, seq, payload)

        char = trainer.chars.get(target)
        if char is None or trainer.client is None:
            return packet(msg_id, seq, code=RESP_CHARACTERISTIC_NOT_FOUND)
        props = dircon_properties(char)

        if msg_id == MSG_READ_CHARACTERISTIC:
            if not props & PROP_READ:
                return packet(msg_id, seq, code=RESP_OPERATION_NOT_SUPPORTED)
            value = await trainer.client.read_gatt_char(char)
            log.debug("read %s -> %s", target, bytes(value).hex())
            return packet(msg_id, seq, target.bytes + bytes(value))

        if msg_id == MSG_WRITE_CHARACTERISTIC:
            if not props & PROP_WRITE:
                return packet(msg_id, seq, code=RESP_OPERATION_NOT_SUPPORTED)
            log.debug("write %s <- %s", target, data.hex())
            # A control point like FTMS's 0x2AD9 rejects writes until its
            # indications are on, but Zwift never asks for them over DIRCON: it
            # expects the responses unprompted. Turn them on for the writer.
            if props & (PROP_NOTIFY | PROP_INDICATE):
                await trainer.subscribe(self, target, enable=True)
            try:
                await trainer.write(char, data)
            except Exception as e:  # noqa: BLE001
                log.warning("write to %s failed: %s", target, e)
                return packet(msg_id, seq, code=RESP_WRITE_FAILED)
            return packet(msg_id, seq, target.bytes)

        if msg_id == MSG_ENABLE_NOTIFICATIONS:
            if not props & (PROP_NOTIFY | PROP_INDICATE):
                return packet(msg_id, seq, code=RESP_OPERATION_NOT_SUPPORTED)
            enable = bool(data[:1] and data[0])
            log.info("%s notifications on %s", "enable" if enable else "disable", target)
            await trainer.subscribe(self, target, enable=enable)
            return packet(msg_id, seq, target.bytes)

        log.warning("unknown message %#x", msg_id)
        return packet(msg_id, seq, code=RESP_UNKNOWN_MESSAGE)


async def find_trainer() -> str:
    """Returns the address of the one nearby trainer, waiting until one shows up."""
    while True:
        log.info("scanning for a trainer (pedal or wake it if it is asleep)")
        found = await BleakScanner.discover(timeout=10, return_adv=True)
        trainers = [
            (device, adv)
            for device, adv in found.values()
            if TRAINER_SERVICES & {u.lower() for u in adv.service_uuids}
        ]
        if len(trainers) == 1:
            device, _ = trainers[0]
            log.info("found %s (%s)", device.name, device.address)
            return device.address
        if trainers:
            listing = "\n".join(f"  {d.address}  {d.name}" for d, _ in trainers)
            raise SystemExit(f"found several trainers; pass one's address:\n{listing}")


def local_ipv4() -> str:
    # Connecting a UDP socket sends nothing; it just picks the outbound interface.
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as s:
        s.connect(("192.0.2.1", 9))
        return s.getsockname()[0]


async def main() -> None:
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        "address", nargs="?", help="trainer Bluetooth address (default: find it by scanning)"
    )
    parser.add_argument("--port", type=int, default=36866)
    parser.add_argument("--name", help="name Zwift shows (default: the trainer's Bluetooth name)")
    parser.add_argument("-v", "--verbose", action="store_true")
    args = parser.parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(message)s",
        datefmt="%H:%M:%S",
    )
    if not args.verbose:
        logging.getLogger("bleak").setLevel(logging.WARNING)

    trainer = Trainer(args.address or await find_trainer())
    ble = asyncio.create_task(trainer.run())
    await trainer.connected.wait()

    server = await asyncio.start_server(
        # Zwift reaches us at the address mDNS announces, so listen everywhere.
        lambda r, w: Session(trainer, r, w).run(),
        host="0.0.0.0",  # noqa: S104
        port=args.port,
    )

    name = args.name or trainer.name
    ip = local_ipv4()
    info = ServiceInfo(
        "_wahoo-fitness-tnp._tcp.local.",
        f"{name}._wahoo-fitness-tnp._tcp.local.",
        addresses=[socket.inet_aton(ip)],
        port=args.port,
        properties={
            "ble-service-uuids": ",".join(str(u) for u in trainer.services),
            "mac-address": trainer.address.replace(":", "-"),
            "serial-number": trainer.address.replace(":", ""),
        },
        server=f"{name.replace(' ', '-')}.local.",
    )
    zc = AsyncZeroconf(ip_version=IPVersion.V4Only)
    # "wahoo-fitness-tnp" is longer than the 15 bytes RFC 6763 allows a service
    # name, so it fails zeroconf's strict check. Wahoo picked it; Zwift expects it.
    await zc.async_register_service(info, strict=False)
    log.info("advertising %r on %s:%d; pair it from Zwift's pairing screen", name, ip, args.port)

    try:
        async with server:
            await asyncio.gather(server.serve_forever(), ble)
    finally:
        await zc.async_unregister_service(info)
        await zc.async_close()


if __name__ == "__main__":
    with contextlib.suppress(KeyboardInterrupt):
        asyncio.run(main())
