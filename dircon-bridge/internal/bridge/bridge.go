// Package bridge relays one Bluetooth LE device to Zwift over DIRCON.
//
// The device is announced over mDNS only while it is connected, and Zwift's
// connections are closed when it drops, so Zwift sees the device come and go
// instead of a connection that answers nothing.
package bridge

import (
	"context"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brutella/dnssd"
	"golang.org/x/sync/singleflight"

	"github.com/jmelahman/dircon-bridge/internal/bluez"
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
)

// Generic Access and Generic Attribute belong to the BLE link, not the device.
var hiddenServices = map[dircon.UUID]bool{
	dircon.MustParseUUID("00001800-0000-1000-8000-00805f9b34fb"): true,
	dircon.MustParseUUID("00001801-0000-1000-8000-00805f9b34fb"): true,
}

// Bridge relays one device.
type Bridge struct {
	dev       *bluez.Device
	responder dnssd.Responder
	log       *slog.Logger
	link      *link
	port      int
	mu        sync.Mutex
}

// New returns a bridge for dev that listens on port and announces itself
// through responder.
func New(dev *bluez.Device, port int, responder dnssd.Responder, log *slog.Logger) *Bridge {
	return &Bridge{
		dev:       dev,
		port:      port,
		responder: responder,
		log:       log.With("device", dev.Address),
	}
}

// Run keeps the device connected and relayed until ctx is done.
func (b *Bridge) Run(ctx context.Context) error {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", net.JoinHostPort("", strconv.Itoa(b.port)))
	if err != nil {
		return err
	}
	go b.accept(ln)
	defer func() { _ = ln.Close() }()

	delay := minRetryDelay
	for {
		if err := b.connectOnce(ctx); err != nil {
			if ctx.Err() == nil {
				b.log.Warn("connection failed", "err", err, "retry", delay)
			}
			delay = min(2*delay, maxRetryDelay)
		} else {
			delay = minRetryDelay
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
	}
}

func (b *Bridge) connectOnce(ctx context.Context) error {
	b.log.Info("connecting (wake the device if it is asleep)")
	dropped := make(chan struct{})
	cctx, cancel := context.WithTimeout(ctx, connectTimeout)
	err := b.dev.Connect(cctx, func() { close(dropped) })
	cancel()
	if err != nil {
		return err
	}
	defer func() { _ = b.dev.Disconnect() }()

	l, err := newLink(b.dev, b.log)
	if err != nil {
		return err
	}
	name := b.dev.Name()
	b.log.Info("connected", "name", name, "services", l.services)

	svc, err := dnssd.NewService(dnssd.Config{
		Name: name,
		Type: ServiceType,
		Port: b.port,
		Text: map[string]string{
			"ble-service-uuids": joinUUIDs(l.services),
			"mac-address":       strings.ReplaceAll(b.dev.Address, ":", "-"),
			"serial-number":     strings.ReplaceAll(b.dev.Address, ":", ""),
		},
	})
	if err != nil {
		return err
	}
	handle, err := b.responder.Add(svc)
	if err != nil {
		return err
	}
	b.setLink(l)
	b.log.Info("advertising to Zwift", "name", name, "port", b.port)

	select {
	case <-dropped:
		b.log.Warn("device disconnected", "reason", b.dev.DisconnectReason())
	case <-ctx.Done():
	}
	b.responder.Remove(handle)
	b.setLink(nil)
	l.close()
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

func joinUUIDs(uuids []dircon.UUID) string {
	s := make([]string, len(uuids))
	for i, u := range uuids {
		s[i] = u.String()
	}
	return strings.Join(s, ",")
}

// link is one BLE connection and the Zwift sessions relayed over it.
type link struct {
	log         *slog.Logger
	chars       map[dircon.UUID]*bluez.Characteristic
	byService   map[dircon.UUID][]dircon.UUID
	notifying   map[dircon.UUID]bool
	subscribers map[dircon.UUID]map[*session]bool
	sessions    map[*session]bool
	writes      singleflight.Group
	services    []dircon.UUID
	writeMu     sync.Mutex
	mu          sync.Mutex
	closed      bool
}

func newLink(dev *bluez.Device, log *slog.Logger) (*link, error) {
	chars, err := dev.Characteristics()
	if err != nil {
		return nil, err
	}
	l := &link{
		log:         log,
		chars:       map[dircon.UUID]*bluez.Characteristic{},
		byService:   map[dircon.UUID][]dircon.UUID{},
		notifying:   map[dircon.UUID]bool{},
		subscribers: map[dircon.UUID]map[*session]bool{},
		sessions:    map[*session]bool{},
	}
	for _, ch := range chars {
		svc, err := dircon.ParseUUID(ch.Service)
		if err != nil || hiddenServices[svc] {
			continue
		}
		u, err := dircon.ParseUUID(ch.UUID)
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
		_ = s.conn.Close()
	}
}

func properties(ch *bluez.Characteristic) byte {
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
// the first time anyone asks.
func (l *link) subscribe(s *session, u dircon.UUID, enable bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	subs := l.subscribers[u]
	if subs == nil {
		subs = map[*session]bool{}
		l.subscribers[u] = subs
	}
	if !enable {
		delete(subs, s)
		return nil
	}
	subs[s] = true
	if l.notifying[u] {
		return nil
	}
	err := l.chars[u].StartNotify(func(value []byte) {
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
	})
	if err != nil {
		return err
	}
	l.notifying[u] = true
	return nil
}

// write writes value to the characteristic. BlueZ fails a write while another
// is outstanding, and a write with response takes about a second. Zwift resends
// a write it hasn't heard back about, so the resend shares the original's
// result instead of colliding with it.
func (l *link) write(ctx context.Context, u dircon.UUID, value []byte) error {
	_, err, _ := l.writes.Do(string(u[:])+string(value), func() (any, error) {
		l.writeMu.Lock()
		defer l.writeMu.Unlock()
		return nil, l.chars[u].Write(ctx, value)
	})
	return err
}

func (l *link) serve(conn net.Conn) {
	s := &session{conn: conn, link: l}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		_ = conn.Close()
		return
	}
	l.sessions[s] = true
	l.mu.Unlock()

	peer := conn.RemoteAddr().String()
	l.log.Info("Zwift connected", "peer", peer)
	defer func() {
		l.mu.Lock()
		delete(l.sessions, s)
		for _, subs := range l.subscribers {
			delete(subs, s)
		}
		l.mu.Unlock()
		_ = conn.Close()
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
	mu   sync.Mutex
}

func (s *session) send(p dircon.Packet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Notifications are sent from BlueZ's signal loop, which a stuck client
	// must not hold up.
	_ = s.conn.SetWriteDeadline(time.Now().Add(sendTimeout))
	_, _ = s.conn.Write(p.Encode())
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
			if err := l.subscribe(s, target, true); err != nil {
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
		if err := l.subscribe(s, target, enable); err != nil {
			l.log.Warn("enabling notifications failed", "char", target, "err", err)
			return fail(dircon.RespUnexpectedError)
		}
		resp.Payload = target[:]
		return resp
	}

	l.log.Warn("unknown message", "id", req.MsgID)
	return fail(dircon.RespUnknownMessage)
}
