// Package bridge relays one Bluetooth LE device to Zwift over DIRCON.
//
// The device is announced over mDNS only while it is connected, and Zwift's
// connections are closed when it drops, so Zwift sees the device come and go
// instead of a connection that answers nothing.
package bridge

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brutella/dnssd"

	"github.com/jmelahman/dircon-bridge/internal/dircon"
)

// ServiceType is the mDNS service Zwift browses for. It is longer than the 15
// bytes RFC 6763 allows a service name; Wahoo picked it and Zwift expects it.
const ServiceType = "_wahoo-fitness-tnp._tcp"

const (
	connectTimeout = 30 * time.Second
	// Retries back off while the device can't be reached, because looking
	// for it scans, and scanning disturbs the other devices' links.
	minRetryDelay = 3 * time.Second
	maxRetryDelay = 30 * time.Second
	gattTimeout   = 10 * time.Second
	sendTimeout   = 5 * time.Second
	// sendQueue is how many packets a session may have waiting to be sent
	// before it is dropped as stuck.
	sendQueue = 64
	// resendWindow is how long after a write succeeds that the same value
	// written again is taken for Zwift resending it.
	resendWindow = 2 * time.Second
	// maxTXT is the most a TXT record string, "key=value", can hold.
	maxTXT          = 255
	txtServiceUUIDs = "ble-service-uuids"
)

// Generic Access and Generic Attribute belong to the BLE link, not the device.
var hiddenServices = map[dircon.UUID]bool{
	dircon.MustParseUUID("00001800-0000-1000-8000-00805f9b34fb"): true,
	dircon.MustParseUUID("00001801-0000-1000-8000-00805f9b34fb"): true,
}

// The services Zwift pairs by, in the order to announce them while there is
// room: Fitness Machine, Cycling Power, Cycling Speed and Cadence, Heart Rate.
var fitnessServices = []dircon.UUID{
	dircon.MustParseUUID("00001826-0000-1000-8000-00805f9b34fb"),
	dircon.MustParseUUID("00001818-0000-1000-8000-00805f9b34fb"),
	dircon.MustParseUUID("00001816-0000-1000-8000-00805f9b34fb"),
	dircon.MustParseUUID("0000180d-0000-1000-8000-00805f9b34fb"),
}

// Device is the Bluetooth LE device a bridge relays.
type Device interface {
	// Address returns the device's address, e.g. D6:F8:D9:7F:99:4B.
	Address() string
	Name() string
	// Connect connects and resolves the device's services, giving up when
	// ctx is done. The channel receives why the connection dropped.
	Connect(ctx context.Context) (dropped <-chan string, err error)
	Disconnect() error
	Characteristics() ([]Characteristic, error)
}

// Characteristic is a GATT characteristic of a connected Device.
type Characteristic interface {
	UUID() string
	// Service returns the UUID of the characteristic's service.
	Service() string
	// Has reports whether the characteristic has a BlueZ flag, e.g. "read",
	// "write", "write-without-response", "notify" or "indicate".
	Has(flag string) bool
	Read(ctx context.Context) ([]byte, error)
	Write(ctx context.Context, value []byte) error
	// StartNotify turns on notifications or indications and calls f with
	// each value until the device is disconnected. f must not block.
	StartNotify(ctx context.Context, f func([]byte)) error
}

// Announcer announces services over mDNS; dnssd.Responder is one.
type Announcer interface {
	Add(svc dnssd.Service) (dnssd.ServiceHandle, error)
	Remove(h dnssd.ServiceHandle)
}

// Bridge relays one device.
type Bridge struct {
	dev       Device
	announcer Announcer
	log       *slog.Logger
	link      *link
	port      int
	mu        sync.Mutex
}

// New returns a bridge for dev that listens on port and announces itself
// through announcer. Port 0 picks a free port.
func New(dev Device, port int, announcer Announcer, log *slog.Logger) *Bridge {
	return &Bridge{
		dev:       dev,
		port:      port,
		announcer: announcer,
		log:       log.With("device", dev.Address()),
	}
}

// Run keeps the device connected and relayed until ctx is done. It withdraws
// the announcement before it returns.
func (b *Bridge) Run(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", net.JoinHostPort("", strconv.Itoa(b.port)))
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()
	addr, err := netip.ParseAddrPort(ln.Addr().String())
	if err != nil {
		return err
	}
	port := int(addr.Port())
	go b.accept(ln)

	var bo backoff
	for {
		err := b.connectOnce(ctx, port)
		if ctx.Err() != nil {
			return nil
		}
		wait := minRetryDelay
		if err != nil {
			wait = bo.next()
			b.log.Warn("connection failed", "err", err, "retry in", wait)
		} else {
			bo = backoff{}
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
		}
	}
}

// backoff doubles the delay between failed attempts, from minRetryDelay up to
// maxRetryDelay. The zero value is ready to use.
type backoff struct{ delay time.Duration }

func (b *backoff) next() time.Duration {
	if b.delay == 0 {
		b.delay = minRetryDelay
	} else {
		b.delay = min(2*b.delay, maxRetryDelay)
	}
	return b.delay
}

func (b *Bridge) connectOnce(ctx context.Context, port int) error {
	b.log.Info("connecting (wake the device if it is asleep)")
	cctx, cancel := context.WithTimeout(ctx, connectTimeout)
	dropped, err := b.dev.Connect(cctx)
	cancel()
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = b.dev.Disconnect() }()

	l, err := newLink(b.dev, b.log)
	if err != nil {
		return fmt.Errorf("list characteristics: %w", err)
	}
	defer l.close()
	name := b.dev.Name()
	b.log.Info("connected", "name", name, "services", l.services)

	uuids, omitted := serviceUUIDs(l.services)
	if len(omitted) > 0 {
		b.log.Warn("services left out of the announcement for lack of room", "services", omitted)
	}
	svc, err := dnssd.NewService(dnssd.Config{
		Name: name,
		Type: ServiceType,
		Port: port,
		Text: map[string]string{
			txtServiceUUIDs: uuids,
			"mac-address":   strings.ReplaceAll(b.dev.Address(), ":", "-"),
			"serial-number": strings.ReplaceAll(b.dev.Address(), ":", ""),
		},
	})
	if err != nil {
		return fmt.Errorf("mDNS service: %w", err)
	}
	handle, err := b.announcer.Add(svc)
	if err != nil {
		return fmt.Errorf("announce: %w", err)
	}
	b.setLink(l)
	b.log.Info("advertising to Zwift", "name", name, "port", port)

	select {
	case reason := <-dropped:
		b.log.Warn("device disconnected", "reason", reason)
	case <-ctx.Done():
	}
	b.setLink(nil)
	b.announcer.Remove(handle)
	return nil
}

func (b *Bridge) setLink(l *link) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.link = l
}

func (b *Bridge) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		b.mu.Lock()
		l := b.link
		b.mu.Unlock()
		if l == nil {
			// Not announced, so a stale client; it reconnects once we are.
			_ = conn.Close()
			continue
		}
		go l.serve(conn)
	}
}

// serviceUUIDs returns the ble-service-uuids TXT value: the fitness services
// first, then the rest, as many as fit in one TXT string. It returns the
// services that didn't fit.
func serviceUUIDs(services []dircon.UUID) (value string, omitted []dircon.UUID) {
	ordered := slices.Clone(services)
	rank := func(u dircon.UUID) int {
		if i := slices.Index(fitnessServices, u); i >= 0 {
			return i
		}
		return len(fitnessServices)
	}
	slices.SortStableFunc(ordered, func(a, b dircon.UUID) int { return rank(a) - rank(b) })

	room := maxTXT - len(txtServiceUUIDs+"=")
	var kept []string
	for _, u := range ordered {
		s := u.String()
		need := len(s)
		if len(kept) > 0 {
			need++ // the comma
		}
		if need > room {
			omitted = append(omitted, u)
			continue
		}
		room -= need
		kept = append(kept, s)
	}
	return strings.Join(kept, ","), omitted
}

// notifyState is a characteristic's notifications being turned on.
type notifyState struct {
	err  error
	done chan struct{}
}

type lastWrite struct {
	at    time.Time
	value []byte
}

// link is one BLE connection and the Zwift sessions relayed over it.
type link struct {
	log         *slog.Logger
	chars       map[dircon.UUID]Characteristic
	byService   map[dircon.UUID][]dircon.UUID
	notifying   map[dircon.UUID]*notifyState
	subscribers map[dircon.UUID]map[*session]bool
	sessions    map[*session]bool
	// The last successful write to each characteristic. Guarded by writeMu.
	lastWrites map[dircon.UUID]lastWrite
	services   []dircon.UUID
	writeMu    sync.Mutex
	mu         sync.Mutex
	closed     bool
}

func newLink(dev Device, log *slog.Logger) (*link, error) {
	chars, err := dev.Characteristics()
	if err != nil {
		return nil, err
	}
	l := &link{
		log:         log,
		chars:       map[dircon.UUID]Characteristic{},
		byService:   map[dircon.UUID][]dircon.UUID{},
		notifying:   map[dircon.UUID]*notifyState{},
		subscribers: map[dircon.UUID]map[*session]bool{},
		sessions:    map[*session]bool{},
		lastWrites:  map[dircon.UUID]lastWrite{},
	}
	for _, ch := range chars {
		svc, err := dircon.ParseUUID(ch.Service())
		if err != nil || hiddenServices[svc] {
			continue
		}
		u, err := dircon.ParseUUID(ch.UUID())
		if err != nil {
			continue
		}
		if _, ok := l.byService[svc]; !ok {
			l.services = append(l.services, svc)
		}
		l.byService[svc] = append(l.byService[svc], u)
		l.chars[u] = ch
	}
	return l, nil
}

func (l *link) close() {
	l.mu.Lock()
	l.closed = true
	sessions := l.sessions
	l.sessions = map[*session]bool{}
	l.mu.Unlock()
	for s := range sessions {
		s.close()
	}
}

func properties(ch Characteristic) byte {
	var p byte
	if ch.Has("read") {
		p |= dircon.PropRead
	}
	if ch.Has("write") || ch.Has("write-without-response") {
		p |= dircon.PropWrite
	}
	if ch.Has("notify") {
		p |= dircon.PropNotify
	}
	if ch.Has("indicate") {
		p |= dircon.PropIndicate
	}
	return p
}

// subscribe turns notifications on or off for s, starting them on the device
// the first time anyone asks. The device is not called with l.mu held, as
// its notifications need l.mu to be delivered.
func (l *link) subscribe(ctx context.Context, s *session, u dircon.UUID, enable bool) error {
	l.mu.Lock()
	subs := l.subscribers[u]
	if subs == nil {
		subs = map[*session]bool{}
		l.subscribers[u] = subs
	}
	if !enable {
		delete(subs, s)
		l.mu.Unlock()
		return nil
	}
	subs[s] = true
	st := l.notifying[u]
	start := st == nil
	if start {
		st = &notifyState{done: make(chan struct{})}
		l.notifying[u] = st
	}
	l.mu.Unlock()

	if start {
		st.err = l.chars[u].StartNotify(ctx, func(value []byte) { l.notify(u, value) })
		if st.err != nil {
			// Let the next subscriber try again.
			l.mu.Lock()
			delete(l.notifying, u)
			l.mu.Unlock()
		}
		close(st.done)
	}
	select {
	case <-st.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if st.err != nil {
		l.mu.Lock()
		delete(subs, s)
		l.mu.Unlock()
	}
	return st.err
}

func (l *link) notify(u dircon.UUID, value []byte) {
	p := dircon.Packet{MsgID: dircon.MsgNotification, Payload: append(u[:], value...)}
	l.mu.Lock()
	targets := make([]*session, 0, len(l.subscribers[u]))
	for t := range l.subscribers[u] {
		targets = append(targets, t)
	}
	l.mu.Unlock()
	for _, t := range targets {
		t.send(p)
	}
}

// write writes value to the characteristic. BlueZ fails a write while another
// is outstanding, so writes go one at a time. A write with response takes
// about a second, and Zwift resends a write it hasn't heard back about; a
// resend of a write that just succeeded is answered without writing again.
func (l *link) write(ctx context.Context, u dircon.UUID, value []byte) error {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	if last, ok := l.lastWrites[u]; ok && bytes.Equal(last.value, value) && time.Since(last.at) < resendWindow {
		l.log.Debug("resend answered without writing", "char", u)
		return nil
	}
	if err := l.chars[u].Write(ctx, value); err != nil {
		delete(l.lastWrites, u)
		return err
	}
	l.lastWrites[u] = lastWrite{value: slices.Clone(value), at: time.Now()}
	return nil
}

func (l *link) serve(conn net.Conn) {
	s := &session{
		conn: conn,
		link: l,
		out:  make(chan dircon.Packet, sendQueue),
		done: make(chan struct{}),
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		_ = conn.Close()
		return
	}
	l.sessions[s] = true
	l.mu.Unlock()
	go s.writeLoop()

	peer := conn.RemoteAddr().String()
	l.log.Info("Zwift connected", "peer", peer)
	defer func() {
		l.mu.Lock()
		delete(l.sessions, s)
		for _, subs := range l.subscribers {
			delete(subs, s)
		}
		l.mu.Unlock()
		s.close()
		l.log.Info("Zwift disconnected", "peer", peer)
	}()
	for {
		p, err := dircon.Read(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				l.log.Debug("read failed", "err", err)
			}
			return
		}
		s.send(s.handle(p))
	}
}

// session is one TCP connection from Zwift.
type session struct {
	conn net.Conn
	link *link
	out  chan dircon.Packet
	// done is closed when the session ends.
	done      chan struct{}
	closeOnce sync.Once
}

// send queues p without blocking: notifications are sent from BlueZ's signal
// loop, which a stuck client must not hold up. A client too far behind is
// dropped; Zwift reconnects.
func (s *session) send(p dircon.Packet) {
	select {
	case <-s.done:
	case s.out <- p:
	default:
		s.link.log.Warn("Zwift isn't keeping up; dropping its connection", "peer", s.conn.RemoteAddr())
		s.close()
	}
}

func (s *session) writeLoop() {
	for {
		select {
		case <-s.done:
			return
		case p := <-s.out:
			_ = s.conn.SetWriteDeadline(time.Now().Add(sendTimeout))
			if _, err := s.conn.Write(p.Encode()); err != nil {
				s.link.log.Debug("send failed", "err", err)
				s.close()
				return
			}
		}
	}
}

func (s *session) close() {
	s.closeOnce.Do(func() {
		close(s.done)
		_ = s.conn.Close()
	})
}

func (s *session) handle(req dircon.Packet) dircon.Packet {
	resp := dircon.Packet{MsgID: req.MsgID, Seq: req.Seq}
	fail := func(code byte) dircon.Packet {
		resp.Code = code
		return resp
	}
	l := s.link

	switch req.MsgID {
	case dircon.MsgDiscoverServices:
		l.log.Debug("discover services")
		for _, u := range l.services {
			resp.Payload = append(resp.Payload, u[:]...)
		}
		return resp
	case dircon.MsgUnknown07:
		return resp
	}

	if len(req.Payload) < dircon.UUIDLen {
		return fail(dircon.RespUnexpectedError)
	}
	target := dircon.UUID(req.Payload[:dircon.UUIDLen])
	data := req.Payload[dircon.UUIDLen:]

	if req.MsgID == dircon.MsgDiscoverCharacteristics {
		chars, ok := l.byService[target]
		if !ok {
			return fail(dircon.RespServiceNotFound)
		}
		l.log.Debug("discover characteristics", "service", target)
		resp.Payload = append(resp.Payload, target[:]...)
		for _, u := range chars {
			resp.Payload = append(resp.Payload, u[:]...)
			resp.Payload = append(resp.Payload, properties(l.chars[u]))
		}
		return resp
	}

	ch, ok := l.chars[target]
	if !ok {
		return fail(dircon.RespCharacteristicNotFound)
	}
	props := properties(ch)
	ctx, cancel := context.WithTimeout(context.Background(), gattTimeout)
	defer cancel()

	switch req.MsgID {
	case dircon.MsgReadCharacteristic:
		if props&dircon.PropRead == 0 {
			return fail(dircon.RespOperationNotSupported)
		}
		value, err := ch.Read(ctx)
		if err != nil {
			l.log.Warn("read failed", "char", target, "err", err)
			return fail(dircon.RespUnexpectedError)
		}
		l.log.Debug("read", "char", target, "value", hex.EncodeToString(value))
		resp.Payload = append(target[:], value...)
		return resp

	case dircon.MsgWriteCharacteristic:
		if props&dircon.PropWrite == 0 {
			return fail(dircon.RespOperationNotSupported)
		}
		l.log.Debug("write", "char", target, "value", hex.EncodeToString(data))
		// A control point like FTMS's 0x2AD9 rejects writes until its
		// indications are on, but Zwift never asks for them over DIRCON: it
		// expects the responses unprompted. Turn them on for the writer.
		if props&(dircon.PropNotify|dircon.PropIndicate) != 0 {
			if err := l.subscribe(ctx, s, target, true); err != nil {
				l.log.Warn("enabling indications failed", "char", target, "err", err)
			}
		}
		if err := l.write(ctx, target, data); err != nil {
			l.log.Warn("write failed", "char", target, "err", err)
			return fail(dircon.RespWriteFailed)
		}
		resp.Payload = target[:]
		return resp

	case dircon.MsgEnableNotifications:
		if props&(dircon.PropNotify|dircon.PropIndicate) == 0 {
			return fail(dircon.RespOperationNotSupported)
		}
		enable := len(data) > 0 && data[0] != 0
		l.log.Info("notifications", "char", target, "enable", enable)
		if err := l.subscribe(ctx, s, target, enable); err != nil {
			l.log.Warn("enabling notifications failed", "char", target, "err", err)
			return fail(dircon.RespUnexpectedError)
		}
		resp.Payload = target[:]
		return resp
	}

	l.log.Warn("unknown message", "id", req.MsgID)
	return fail(dircon.RespUnknownMessage)
}
