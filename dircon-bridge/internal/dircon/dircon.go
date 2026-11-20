// Package dircon encodes and decodes Wahoo Direct Connect (DIRCON) packets.
//
// DIRCON is GATT over TCP. Each packet is a six-byte header (version, message
// id, sequence number, response code, big-endian payload length) followed by
// the payload, in which UUIDs are 16 big-endian bytes. The layout follows
// qdomyos-zwift's src/devices/dircon/dirconpacket.cpp.
package dircon

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Message ids.
const (
	MsgDiscoverServices        byte = 0x01
	MsgDiscoverCharacteristics byte = 0x02
	MsgReadCharacteristic      byte = 0x03
	MsgWriteCharacteristic     byte = 0x04
	MsgEnableNotifications     byte = 0x05
	MsgNotification            byte = 0x06
	// MsgUnknown07 is sent by Zwift with no documented meaning; it expects
	// an empty success response.
	MsgUnknown07 byte = 0x07
)

// Response codes.
const (
	RespSuccess                byte = 0x00
	RespUnknownMessage         byte = 0x01
	RespUnexpectedError        byte = 0x02
	RespServiceNotFound        byte = 0x03
	RespCharacteristicNotFound byte = 0x04
	RespOperationNotSupported  byte = 0x05
	RespWriteFailed            byte = 0x06
)

// Characteristic property flags, as reported by MsgDiscoverCharacteristics.
const (
	PropRead     byte = 0x01
	PropWrite    byte = 0x02
	PropNotify   byte = 0x04
	PropIndicate byte = 0x08
)

const (
	version   = 1
	headerLen = 6
	// UUIDLen is the length of a UUID in a payload.
	UUIDLen = 16
)

// ErrBadUUID is returned by ParseUUID for a malformed UUID.
var ErrBadUUID = errors.New("malformed UUID")

// UUID is a 128-bit Bluetooth UUID in DIRCON's big-endian byte order.
type UUID [UUIDLen]byte

// ParseUUID parses the canonical form, e.g. 00002ad9-0000-1000-8000-00805f9b34fb.
func ParseUUID(s string) (UUID, error) {
	var u UUID
	b, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if err != nil || len(b) != UUIDLen {
		return u, fmt.Errorf("%w: %q", ErrBadUUID, s)
	}
	copy(u[:], b)
	return u, nil
}

// MustParseUUID is ParseUUID for constants.
func MustParseUUID(s string) UUID {
	u, err := ParseUUID(s)
	if err != nil {
		panic(err)
	}
	return u
}

func (u UUID) String() string {
	h := hex.EncodeToString(u[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// Packet is one DIRCON message.
type Packet struct {
	Payload []byte
	MsgID   byte
	Seq     byte
	Code    byte
}

// Read reads one packet from r.
func Read(r io.Reader) (Packet, error) {
	var h [headerLen]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Packet{}, err
	}
	p := Packet{MsgID: h[1], Seq: h[2], Code: h[3]}
	p.Payload = make([]byte, binary.BigEndian.Uint16(h[4:]))
	if _, err := io.ReadFull(r, p.Payload); err != nil {
		return Packet{}, err
	}
	return p, nil
}

// Encode returns the packet's wire form. An error response carries no payload.
func (p Packet) Encode() []byte {
	payload := p.Payload
	if p.Code != RespSuccess {
		payload = nil
	}
	b := make([]byte, headerLen, headerLen+len(payload))
	b[0] = version
	b[1] = p.MsgID
	b[2] = p.Seq
	b[3] = p.Code
	binary.BigEndian.PutUint16(b[4:], uint16(len(payload))) //nolint:gosec // GATT values are far below 64 KiB.
	return append(b, payload...)
}
