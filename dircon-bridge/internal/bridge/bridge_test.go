package bridge_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/brutella/dnssd"

	"github.com/jmelahman/dircon-bridge/internal/bridge"
	"github.com/jmelahman/dircon-bridge/internal/dircon"
)

const (
	svcGenericAccess = "00001800-0000-1000-8000-00805f9b34fb"
	svcFitness       = "00001826-0000-1000-8000-00805f9b34fb"
	svcBattery       = "0000180f-0000-1000-8000-00805f9b34fb"
	charDeviceName   = "00002a00-0000-1000-8000-00805f9b34fb"
	charBikeData     = "00002ad2-0000-1000-8000-00805f9b34fb"
	charControlPoint = "00002ad9-0000-1000-8000-00805f9b34fb"
	charFeature      = "00002acc-0000-1000-8000-00805f9b34fb"
	charBattery      = "00002a19-0000-1000-8000-00805f9b34fb"
	address          = "D6:F8:D9:7F:99:4B"
)

var errWrite = errors.New("write rejected")

type fakeChar struct {
	writeErr error
	notify   func([]byte)
	uuid     string
	service  string
	flags    []string
	value    []byte
	writes   [][]byte
	starts   int
	mu       sync.Mutex
}

func (c *fakeChar) UUID() string         { return c.uuid }
func (c *fakeChar) Service() string      { return c.service }
func (c *fakeChar) Has(flag string) bool { return slices.Contains(c.flags, flag) }
func (c *fakeChar) Read(context.Context) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value, nil
}

func (c *fakeChar) Write(_ context.Context, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeErr != nil {
		return c.writeErr
	}
	c.writes = append(c.writes, slices.Clone(value))
	return nil
}

func (c *fakeChar) StartNotify(_ context.Context, f func([]byte)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts++
	c.notify = f
	return nil
}

// push delivers a notification as BlueZ's signal loop would.
func (c *fakeChar) push(t *testing.T, value []byte) {
	t.Helper()
	c.mu.Lock()
	f := c.notify
	c.mu.Unlock()
	if f == nil {
		t.Fatalf("%s: push before StartNotify", c.uuid)
	}
	f(value)
}

func (c *fakeChar) stats() (starts int, writes [][]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.starts, slices.Clone(c.writes)
}

type fakeDevice struct {
	dropped     chan string
	chars       []*fakeChar
	connects    int
	disconnects int
	mu          sync.Mutex
}

func (d *fakeDevice) Address() string { return address }
func (d *fakeDevice) Name() string    { return "Magnus 42871" }

// Connect succeeds once; later attempts wait for ctx, like a device that
// stays away.
func (d *fakeDevice) Connect(ctx context.Context) (<-chan string, error) {
	d.mu.Lock()
	d.connects++
	n := d.connects
	d.mu.Unlock()
	if n > 1 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return d.dropped, nil
}

func (d *fakeDevice) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.disconnects++
	return nil
}

func (d *fakeDevice) Characteristics() ([]bridge.Characteristic, error) {
	out := make([]bridge.Characteristic, len(d.chars))
	for i, c := range d.chars {
		out[i] = c
	}
	return out, nil
}

func (d *fakeDevice) char(uuid string) *fakeChar {
	for _, c := range d.chars {
		if c.uuid == uuid {
			return c
		}
	}
	panic("no characteristic " + uuid)
}

type fakeAnnouncer struct {
	added   chan dnssd.Service
	removed chan struct{}
}

type fakeHandle struct{ svc dnssd.Service }

func (h fakeHandle) UpdateText(map[string]string, dnssd.Responder) {}
func (h fakeHandle) Service() dnssd.Service                        { return h.svc }

func (a *fakeAnnouncer) Add(svc dnssd.Service) (dnssd.ServiceHandle, error) {
	a.added <- svc
	return fakeHandle{svc}, nil
}

func (a *fakeAnnouncer) Remove(dnssd.ServiceHandle) { a.removed <- struct{}{} }

func newDevice() *fakeDevice {
	return &fakeDevice{
		dropped: make(chan string, 1),
		chars: []*fakeChar{
			{uuid: charDeviceName, service: svcGenericAccess, flags: []string{"read"}},
			{uuid: charBattery, service: svcBattery, flags: []string{"read", "notify"}, value: []byte{87}},
			{uuid: charFeature, service: svcFitness, flags: []string{"read"}, value: []byte{1, 2}},
			{uuid: charBikeData, service: svcFitness, flags: []string{"notify"}},
			{uuid: charControlPoint, service: svcFitness, flags: []string{"write", "indicate"}},
		},
	}
}

type harness struct {
	dev       *fakeDevice
	announcer *fakeAnnouncer
	svc       dnssd.Service
	cancel    context.CancelFunc
	done      chan error
}

// start runs a bridge for a fresh fake device until it is announced.
func start(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		dev: newDevice(),
		announcer: &fakeAnnouncer{
			added:   make(chan dnssd.Service, 1),
			removed: make(chan struct{}, 1),
		},
		done: make(chan error, 1),
	}
	ctx, cancel := context.WithCancel(t.Context())
	h.cancel = cancel
	b := bridge.New(h.dev, 0, h.announcer, slog.New(slog.DiscardHandler))
	go func() { h.done <- b.Run(ctx) }()
	select {
	case h.svc = <-h.announcer.added:
	case <-time.After(5 * time.Second):
		t.Fatal("bridge not announced")
	}
	t.Cleanup(func() {
		cancel()
		if err := <-h.done; err != nil {
			t.Errorf("Run = %v", err)
		}
	})
	return h
}

func (h *harness) dial(t *testing.T) *client {
	t.Helper()
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(h.svc.Port)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &client{conn: conn}
}

type client struct {
	conn net.Conn
	seq  byte
}

func (c *client) request(t *testing.T, msg byte, payload ...byte) dircon.Packet {
	t.Helper()
	c.seq++
	if _, err := c.conn.Write(dircon.Packet{MsgID: msg, Seq: c.seq, Payload: payload}.Encode()); err != nil {
		t.Fatal(err)
	}
	for {
		p := c.read(t)
		if p.MsgID == dircon.MsgNotification {
			continue
		}
		if p.MsgID != msg || p.Seq != c.seq {
			t.Fatalf("got response %d seq %d, want %d seq %d", p.MsgID, p.Seq, msg, c.seq)
		}
		return p
	}
}

func (c *client) read(t *testing.T) dircon.Packet {
	t.Helper()
	_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	p, err := dircon.Read(c.conn)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func uuid(s string) []byte {
	u := dircon.MustParseUUID(s)
	return u[:]
}

func cat(parts ...[]byte) []byte { return slices.Concat(parts...) }

func TestHandle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload []byte
		want    []byte
		msg     byte
		code    byte
	}{
		{
			name: "discover services hides the link's own",
			msg:  dircon.MsgDiscoverServices,
			want: cat(uuid(svcBattery), uuid(svcFitness)),
		},
		{
			name:    "discover characteristics",
			msg:     dircon.MsgDiscoverCharacteristics,
			payload: uuid(svcFitness),
			want: cat(
				uuid(svcFitness),
				uuid(charFeature), []byte{dircon.PropRead},
				uuid(charBikeData), []byte{dircon.PropNotify},
				uuid(charControlPoint), []byte{dircon.PropWrite | dircon.PropIndicate},
			),
		},
		{name: "discover hidden service", msg: dircon.MsgDiscoverCharacteristics, payload: uuid(svcGenericAccess), code: dircon.RespServiceNotFound},
		{name: "read", msg: dircon.MsgReadCharacteristic, payload: uuid(charFeature), want: cat(uuid(charFeature), []byte{1, 2})},
		{name: "read unreadable", msg: dircon.MsgReadCharacteristic, payload: uuid(charBikeData), code: dircon.RespOperationNotSupported},
		{name: "read hidden", msg: dircon.MsgReadCharacteristic, payload: uuid(charDeviceName), code: dircon.RespCharacteristicNotFound},
		{name: "write unwritable", msg: dircon.MsgWriteCharacteristic, payload: cat(uuid(charFeature), []byte{0}), code: dircon.RespOperationNotSupported},
		{name: "write", msg: dircon.MsgWriteCharacteristic, payload: cat(uuid(charControlPoint), []byte{0}), want: uuid(charControlPoint)},
		{name: "enable notifications", msg: dircon.MsgEnableNotifications, payload: cat(uuid(charBikeData), []byte{1}), want: uuid(charBikeData)},
		{name: "notify unnotifiable", msg: dircon.MsgEnableNotifications, payload: cat(uuid(charFeature), []byte{1}), code: dircon.RespOperationNotSupported},
		{name: "short payload", msg: dircon.MsgReadCharacteristic, payload: []byte{1, 2}, code: dircon.RespUnexpectedError},
		{name: "message 7", msg: dircon.MsgUnknown07},
		{name: "unknown message", msg: 0x42, payload: uuid(charFeature), code: dircon.RespUnknownMessage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := start(t).dial(t)
			got := c.request(t, tt.msg, tt.payload...)
			if got.Code != tt.code {
				t.Fatalf("code = %d, want %d", got.Code, tt.code)
			}
			if !bytes.Equal(got.Payload, tt.want) {
				t.Errorf("payload = %x, want %x", got.Payload, tt.want)
			}
		})
	}
}

func TestWriteFailure(t *testing.T) {
	t.Parallel()

	h := start(t)
	h.dev.char(charControlPoint).writeErr = errWrite
	got := h.dial(t).request(t, dircon.MsgWriteCharacteristic, cat(uuid(charControlPoint), []byte{0})...)
	if got.Code != dircon.RespWriteFailed {
		t.Fatalf("code = %d, want %d", got.Code, dircon.RespWriteFailed)
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()

	h := start(t)
	c := h.dial(t)
	cp := h.dev.char(charControlPoint)
	for _, v := range []byte{1, 1, 2, 1} {
		if got := c.request(t, dircon.MsgWriteCharacteristic, cat(uuid(charControlPoint), []byte{v})...); got.Code != dircon.RespSuccess {
			t.Fatalf("write %d: code %d", v, got.Code)
		}
	}
	starts, writes := cp.stats()
	// The second 1 is a resend; the last follows a different value, so it is
	// a new command.
	if want := [][]byte{{1}, {2}, {1}}; !slices.EqualFunc(writes, want, bytes.Equal) {
		t.Errorf("writes = %v, want %v", writes, want)
	}
	// Indications are turned on before the first write, once.
	if starts != 1 {
		t.Errorf("StartNotify called %d times, want 1", starts)
	}
	cp.push(t, []byte{0x80, 0, 1})
	if got, want := c.read(t), cat(uuid(charControlPoint), []byte{0x80, 0, 1}); !bytes.Equal(got.Payload, want) {
		t.Errorf("indication = %x, want %x", got.Payload, want)
	}
}

func TestNotifications(t *testing.T) {
	t.Parallel()

	h := start(t)
	a, b := h.dial(t), h.dial(t)
	enable := func(c *client, on byte) {
		t.Helper()
		if got := c.request(t, dircon.MsgEnableNotifications, cat(uuid(charBikeData), []byte{on})...); got.Code != dircon.RespSuccess {
			t.Fatalf("enable %d: code %d", on, got.Code)
		}
	}
	enable(a, 1)
	enable(b, 1)
	ch := h.dev.char(charBikeData)
	if starts, _ := ch.stats(); starts != 1 {
		t.Errorf("StartNotify called %d times, want 1", starts)
	}

	ch.push(t, []byte{7})
	want := cat(uuid(charBikeData), []byte{7})
	for _, c := range []*client{a, b} {
		if got := c.read(t); got.MsgID != dircon.MsgNotification || !bytes.Equal(got.Payload, want) {
			t.Errorf("got %d %x, want notification %x", got.MsgID, got.Payload, want)
		}
	}

	enable(a, 0)
	ch.push(t, []byte{8})
	if got := b.read(t); !bytes.Equal(got.Payload, cat(uuid(charBikeData), []byte{8})) {
		t.Errorf("b got %x", got.Payload)
	}
	// a is past its last notification once its next response arrives.
	if got := a.request(t, dircon.MsgUnknown07); got.Code != dircon.RespSuccess {
		t.Errorf("code = %d", got.Code)
	}
}

// A client that stops reading must not stall the notifications, which BlueZ
// delivers from the loop every device shares.
func TestStuckClient(t *testing.T) {
	t.Parallel()

	h := start(t)
	stuck := h.dial(t)
	if got := stuck.request(t, dircon.MsgEnableNotifications, cat(uuid(charBikeData), []byte{1})...); got.Code != dircon.RespSuccess {
		t.Fatalf("code = %d", got.Code)
	}
	ch := h.dev.char(charBikeData)
	value := make([]byte, 200)
	deadline := time.Now().Add(5 * time.Second)
	for range 50_000 {
		ch.push(t, value)
		if time.Now().After(deadline) {
			t.Fatal("notifications blocked by a client that isn't reading")
		}
	}
	// The stuck client was dropped: reading drains what was sent, then ends.
	_ = stuck.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.Copy(io.Discard, stuck.conn); err != nil {
		t.Errorf("stuck client not disconnected: %v", err)
	}
}

func TestAnnouncement(t *testing.T) {
	t.Parallel()

	h := start(t)
	for k, want := range map[string]string{
		"ble-service-uuids": svcFitness + "," + svcBattery,
		"mac-address":       "D6-F8-D9-7F-99-4B",
		"serial-number":     "D6F8D97F994B",
	} {
		if got := h.svc.Text[k]; got != want {
			t.Errorf("TXT %s = %q, want %q", k, got, want)
		}
	}
	if h.svc.Type != bridge.ServiceType || h.svc.Name != "Magnus 42871" {
		t.Errorf("service %q of type %q", h.svc.Name, h.svc.Type)
	}

	c := h.dial(t)
	c.request(t, dircon.MsgDiscoverServices)
	h.dev.dropped <- "Connection timeout (org.bluez.Reason.Timeout)"
	select {
	case <-h.announcer.removed:
	case <-time.After(5 * time.Second):
		t.Fatal("announcement not withdrawn when the device dropped")
	}
	// Zwift's connections are closed too.
	_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := dircon.Read(c.conn); !errors.Is(err, io.EOF) {
		t.Errorf("read after drop = %v, want EOF", err)
	}
	// The device is disconnected just after.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		h.dev.mu.Lock()
		disconnects := h.dev.disconnects
		h.dev.mu.Unlock()
		if disconnects == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Disconnect called %d times, want 1", disconnects)
		}
	}
}

func TestShutdownWithdraws(t *testing.T) {
	t.Parallel()

	h := start(t)
	h.cancel()
	select {
	case <-h.announcer.removed:
	case <-time.After(5 * time.Second):
		t.Fatal("announcement not withdrawn on shutdown")
	}
	if err := <-h.done; err != nil {
		t.Errorf("Run = %v, want nil", err)
	}
	h.done <- nil // for the cleanup
}

func TestServiceUUIDs(t *testing.T) {
	t.Parallel()

	short := func(n uint16) dircon.UUID {
		u := dircon.MustParseUUID(svcFitness)
		u[2], u[3] = byte(n>>8), byte(n)
		return u
	}
	vendor := func(n byte) dircon.UUID {
		u := dircon.MustParseUUID("a026ee0b-0a7d-4ab3-97fa-f1500f9feb8b")
		u[15] = n
		return u
	}
	tests := []struct {
		name        string
		in          []dircon.UUID
		want        []dircon.UUID
		wantOmitted []dircon.UUID
	}{
		{name: "none"},
		{
			name: "fitness services first",
			in:   []dircon.UUID{short(0x180f), short(0x180d), short(0x1818), short(0x1826)},
			want: []dircon.UUID{short(0x1826), short(0x1818), short(0x180d), short(0x180f)},
		},
		{
			name:        "what doesn't fit is left out",
			in:          []dircon.UUID{vendor(1), vendor(2), vendor(3), vendor(4), vendor(5), short(0x1826), vendor(6)},
			want:        []dircon.UUID{short(0x1826), vendor(1), vendor(2), vendor(3), vendor(4), vendor(5)},
			wantOmitted: []dircon.UUID{vendor(6)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, omitted := bridge.ServiceUUIDs(tt.in)
			var want []byte
			for i, u := range tt.want {
				if i > 0 {
					want = append(want, ',')
				}
				want = append(want, u.String()...)
			}
			if got != string(want) {
				t.Errorf("value = %q, want %q", got, want)
			}
			if len("ble-service-uuids="+got) > 255 {
				t.Errorf("TXT string is %d bytes, over 255", len("ble-service-uuids="+got))
			}
			if !slices.Equal(omitted, tt.wantOmitted) {
				t.Errorf("omitted = %v, want %v", omitted, tt.wantOmitted)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	t.Parallel()

	got := bridge.BackoffDelays(6)
	want := []time.Duration{3 * time.Second, 6 * time.Second, 12 * time.Second, 24 * time.Second, 30 * time.Second, 30 * time.Second}
	if !slices.Equal(got, want) {
		t.Errorf("delays = %v, want %v", got, want)
	}
}
