package dircon_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jmelahman/dircon-bridge/internal/dircon"
)

func TestUUIDRoundTrip(t *testing.T) {
	t.Parallel()

	const s = "00002ad9-0000-1000-8000-00805f9b34fb"
	u, err := dircon.ParseUUID(s)
	if err != nil {
		t.Fatal(err)
	}
	if u[2] != 0x2a || u[3] != 0xd9 {
		t.Errorf("ParseUUID(%q) = % x, want big-endian bytes", s, u[:])
	}
	if got := u.String(); got != s {
		t.Errorf("String() = %q, want %q", got, s)
	}
}

func TestParseUUIDRejects(t *testing.T) {
	t.Parallel()

	for _, s := range []string{"", "2ad9", "zz002ad9-0000-1000-8000-00805f9b34fb"} {
		if _, err := dircon.ParseUUID(s); !errors.Is(err, dircon.ErrBadUUID) {
			t.Errorf("ParseUUID(%q) error = %v, want ErrBadUUID", s, err)
		}
	}
}

func TestPacketRoundTrip(t *testing.T) {
	t.Parallel()

	want := dircon.Packet{MsgID: dircon.MsgWriteCharacteristic, Seq: 7, Payload: []byte{1, 2, 3}}
	wire := want.Encode()
	if !bytes.Equal(wire[:6], []byte{1, 4, 7, 0, 0, 3}) {
		t.Errorf("header = % x", wire[:6])
	}
	got, err := dircon.Read(bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	if got.MsgID != want.MsgID || got.Seq != want.Seq || !bytes.Equal(got.Payload, want.Payload) {
		t.Errorf("Read(Encode(p)) = %+v, want %+v", got, want)
	}
}

func TestErrorDropsPayload(t *testing.T) {
	t.Parallel()

	p := dircon.Packet{MsgID: 3, Code: dircon.RespCharacteristicNotFound, Payload: []byte{9}}
	if got := p.Encode(); !bytes.Equal(got, []byte{1, 3, 0, 4, 0, 0}) {
		t.Errorf("Encode() = % x", got)
	}
}
