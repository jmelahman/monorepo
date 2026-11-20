// Package bluez is a small client for the parts of BlueZ's D-Bus API that a
// GATT relay needs: finding and connecting to LE devices, listing their
// characteristics, and reading, writing and subscribing to them.
package bluez

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	service             = "org.bluez"
	ifaceAdapter        = "org.bluez.Adapter1"
	ifaceDevice         = "org.bluez.Device1"
	ifaceService        = "org.bluez.GattService1"
	ifaceCharacteristic = "org.bluez.GattCharacteristic1"
	ifaceProperties     = "org.freedesktop.DBus.Properties"
	ifaceObjectManager  = "org.freedesktop.DBus.ObjectManager"

	sigPropertiesChanged = ifaceProperties + ".PropertiesChanged"
	sigInterfacesAdded   = ifaceObjectManager + ".InterfacesAdded"
	sigDisconnected      = ifaceDevice + ".Disconnected"

	// callTimeout bounds calls that only involve bluetoothd, not the radio.
	callTimeout = 10 * time.Second
)

var (
	// ErrNoAdapter is returned when the host has no Bluetooth adapter.
	ErrNoAdapter = errors.New("no Bluetooth adapter")
	// ErrBadAddress is returned for an address that isn't six hex bytes
	// separated by colons or dashes.
	ErrBadAddress = errors.New("malformed Bluetooth address, want e.g. D6:F8:D9:7F:99:4B")
	// ErrNotSeen is returned by Connect when the device never advertised.
	ErrNotSeen = errors.New("device not seen advertising")
	// ErrDropped is returned by Connect when the device disconnected before
	// its services were resolved.
	ErrDropped = errors.New("device disconnected while connecting")
)

var addressRe = regexp.MustCompile(`^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`)

// ParseAddress returns the canonical form of a Bluetooth address: upper case,
// separated by colons.
func ParseAddress(s string) (string, error) {
	a := strings.ToUpper(strings.ReplaceAll(s, "-", ":"))
	if !addressRe.MatchString(a) {
		return "", fmt.Errorf("%w: %q", ErrBadAddress, s)
	}
	return a, nil
}

type objects = map[dbus.ObjectPath]map[string]map[string]dbus.Variant

// Client is a connection to BlueZ on the system bus.
//
// One goroutine delivers BlueZ's signals to the callbacks registered here, so
// callbacks must not block.
type Client struct {
	conn *dbus.Conn
	// Object path -> callback for the signals it emits.
	watchers map[dbus.ObjectPath]func(*dbus.Signal)
	// Callbacks for device paths that are seen advertising during discovery.
	seen    map[int]func(dbus.ObjectPath)
	adapter dbus.ObjectPath
	nextID  int
	// discovering counts the callers that need discovery running. It is
	// guarded by discoveryMu, which is held across the D-Bus calls that start
	// and stop discovery. mu is never held across a D-Bus call, because
	// signal delivery needs it.
	discovering int
	discoveryMu sync.Mutex
	mu          sync.Mutex
}

// New connects to the system bus and picks the first adapter.
func New() (*Client, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:     conn,
		watchers: map[dbus.ObjectPath]func(*dbus.Signal){},
		seen:     map[int]func(dbus.ObjectPath){},
	}
	objs, err := c.objects()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	for _, path := range slices.Sorted(maps.Keys(objs)) {
		if _, ok := objs[path][ifaceAdapter]; ok {
			c.adapter = path
			break
		}
	}
	if c.adapter == "" {
		_ = conn.Close()
		return nil, ErrNoAdapter
	}
	for _, opt := range [][]dbus.MatchOption{
		{dbus.WithMatchInterface(ifaceProperties), dbus.WithMatchMember("PropertiesChanged"), dbus.WithMatchPathNamespace("/org/bluez")},
		{dbus.WithMatchInterface(ifaceObjectManager), dbus.WithMatchMember("InterfacesAdded")},
		{dbus.WithMatchInterface(ifaceDevice), dbus.WithMatchMember("Disconnected")},
	} {
		if err := conn.AddMatchSignal(append(opt, dbus.WithMatchSender(service))...); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	// Once this is full, godbus delivers each signal on a new goroutine, out
	// of order. Scanning busy surroundings produces bursts.
	signals := make(chan *dbus.Signal, 1024)
	conn.Signal(signals)
	go c.dispatch(signals)
	return c, nil
}

// Close disconnects from the bus.
func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) call(obj dbus.BusObject, method string, args ...any) *dbus.Call {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	return obj.CallWithContext(ctx, method, 0, args...)
}

func (c *Client) objects() (objects, error) {
	var objs objects
	err := c.call(c.conn.Object(service, "/"), ifaceObjectManager+".GetManagedObjects").Store(&objs)
	return objs, err
}

func (c *Client) dispatch(signals <-chan *dbus.Signal) {
	for sig := range signals {
		if sig.Name == sigInterfacesAdded {
			if len(sig.Body) < 2 {
				continue
			}
			path, _ := sig.Body[0].(dbus.ObjectPath)
			ifaces, _ := sig.Body[1].(map[string]map[string]dbus.Variant)
			if _, ok := ifaces[ifaceDevice]; ok {
				c.notifySeen(path)
			}
			continue
		}
		c.mu.Lock()
		watch := c.watchers[sig.Path]
		c.mu.Unlock()
		if watch != nil {
			watch(sig)
		}
		if iface, changed, ok := propertiesChanged(sig); ok && iface == ifaceDevice &&
			hasAny(changed, "RSSI", "ManufacturerData", "ServiceData") {
			// An advertisement updates RSSI or the advertised data.
			c.notifySeen(sig.Path)
		}
	}
}

func propertiesChanged(sig *dbus.Signal) (iface string, changed map[string]dbus.Variant, ok bool) {
	if sig.Name != sigPropertiesChanged || len(sig.Body) < 2 {
		return "", nil, false
	}
	iface, _ = sig.Body[0].(string)
	changed, _ = sig.Body[1].(map[string]dbus.Variant)
	return iface, changed, true
}

func hasAny(m map[string]dbus.Variant, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func (c *Client) notifySeen(path dbus.ObjectPath) {
	c.mu.Lock()
	if len(c.seen) == 0 {
		c.mu.Unlock()
		return
	}
	seen := slices.Collect(maps.Values(c.seen))
	c.mu.Unlock()
	for _, f := range seen {
		f(path)
	}
}

func (c *Client) watch(path dbus.ObjectPath, f func(*dbus.Signal)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchers[path] = f
}

// unwatch stops watching path and every object below it.
func (c *Client) unwatch(path dbus.ObjectPath) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for p := range c.watchers {
		if p == path || strings.HasPrefix(string(p), string(path)+"/") {
			delete(c.watchers, p)
		}
	}
}

// discover calls seen with each device path that advertises until the
// returned stop function is called. Concurrent calls share one discovery,
// with the filter of whichever started it.
func (c *Client) discover(filter map[string]any, seen func(dbus.ObjectPath)) (stop func(), err error) {
	c.discoveryMu.Lock()
	defer c.discoveryMu.Unlock()
	if c.discovering == 0 {
		adapter := c.conn.Object(service, c.adapter)
		if err := c.call(adapter, ifaceAdapter+".SetDiscoveryFilter", filter).Err; err != nil {
			return nil, fmt.Errorf("set discovery filter: %w", err)
		}
		if err := c.call(adapter, ifaceAdapter+".StartDiscovery").Err; err != nil {
			return nil, fmt.Errorf("start discovery: %w", err)
		}
	}
	c.discovering++
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.seen[id] = seen
	c.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			delete(c.seen, id)
			c.mu.Unlock()
			c.discoveryMu.Lock()
			defer c.discoveryMu.Unlock()
			c.discovering--
			if c.discovering == 0 {
				_ = c.call(c.conn.Object(service, c.adapter), ifaceAdapter+".StopDiscovery").Err
			}
		})
	}, nil
}

// DeviceInfo describes a device found by Discover.
type DeviceInfo struct {
	Address string
	Name    string
	UUIDs   []string
}

// Discover scans for d and returns the devices seen advertising any of uuids.
func (c *Client) Discover(ctx context.Context, d time.Duration, uuids ...string) ([]DeviceInfo, error) {
	var mu sync.Mutex
	paths := map[dbus.ObjectPath]bool{}
	// DuplicateData reports a device BlueZ already knows each time it
	// advertises, not only when its data changes, so it is seen this scan.
	filter := map[string]any{"Transport": "le", "DuplicateData": true}
	if len(uuids) > 0 {
		filter["UUIDs"] = uuids
	}
	stop, err := c.discover(filter, func(p dbus.ObjectPath) {
		mu.Lock()
		defer mu.Unlock()
		paths[p] = true
	})
	if err != nil {
		return nil, err
	}
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	timer.Stop()
	stop()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	objs, err := c.objects()
	if err != nil {
		return nil, err
	}
	var found []DeviceInfo
	mu.Lock()
	defer mu.Unlock()
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		props, ok := objs[p][ifaceDevice]
		if !ok {
			continue
		}
		info := DeviceInfo{
			Address: str(props["Address"]),
			Name:    str(props["Alias"]),
			UUIDs:   strs(props["UUIDs"]),
		}
		// A shared discovery may be running with another filter.
		if len(uuids) == 0 || slices.ContainsFunc(info.UUIDs, func(u string) bool { return slices.Contains(uuids, u) }) {
			found = append(found, info)
		}
	}
	return found, nil
}

func str(v dbus.Variant) string {
	s, _ := v.Value().(string)
	return s
}

func strs(v dbus.Variant) []string {
	s, _ := v.Value().([]string)
	return s
}

// Device is a remote LE device, which BlueZ need not know yet.
type Device struct {
	c       *Client
	path    dbus.ObjectPath
	address string
}

// Device returns the device with the given address.
func (c *Client) Device(address string) (*Device, error) {
	address, err := ParseAddress(address)
	if err != nil {
		return nil, err
	}
	path := c.adapter + dbus.ObjectPath("/dev_"+strings.ReplaceAll(address, ":", "_"))
	return &Device{c: c, path: path, address: address}, nil
}

// Address returns the device's address, e.g. D6:F8:D9:7F:99:4B.
func (d *Device) Address() string { return d.address }

func (d *Device) obj() dbus.BusObject { return d.c.conn.Object(service, d.path) }

func (d *Device) prop(name string) (dbus.Variant, error) {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	var v dbus.Variant
	err := d.obj().CallWithContext(ctx, ifaceProperties+".Get", 0, ifaceDevice, name).Store(&v)
	return v, err
}

func (d *Device) known() bool {
	_, err := d.prop("Address")
	return err == nil
}

// waitKnown scans until BlueZ knows the device. BlueZ forgets an unpaired
// device it hasn't seen for a while, so this is needed again before each
// connect.
func (d *Device) waitKnown(ctx context.Context) error {
	if d.known() {
		return nil
	}
	found := make(chan struct{}, 1)
	// BlueZ reports an unknown device with InterfacesAdded the first time it
	// is seen, so this needs no DuplicateData.
	stop, err := d.c.discover(map[string]any{"Transport": "le"}, func(p dbus.ObjectPath) {
		if p == d.path {
			select {
			case found <- struct{}{}:
			default:
			}
		}
	})
	if err != nil {
		return err
	}
	defer stop()
	// The device may have appeared before discovery reported it.
	if d.known() {
		return nil
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrNotSeen, ctx.Err())
	case <-found:
		return nil
	}
}

// Name returns the device's name, or its address if it has none.
func (d *Device) Name() string {
	if v, err := d.prop("Alias"); err == nil && str(v) != "" {
		return str(v)
	}
	return d.address
}

// Connect connects to the device and waits until its services are resolved,
// scanning for it first if BlueZ doesn't know it. ctx bounds the attempt, not
// the connection. If the connection drops later, the returned channel
// receives why.
func (d *Device) Connect(ctx context.Context) (dropped <-chan string, err error) {
	if err := d.waitKnown(ctx); err != nil {
		return nil, err
	}
	resolved := make(chan struct{})
	drops := make(chan string, 1)
	var (
		mu       sync.Mutex
		reason   string
		resolve  sync.Once
		dropOnce sync.Once
	)
	d.c.watch(d.path, func(sig *dbus.Signal) {
		if sig.Name == sigDisconnected && len(sig.Body) >= 2 {
			// BlueZ 5.82 and later send why, before Connected turns false.
			name, _ := sig.Body[0].(string)
			msg, _ := sig.Body[1].(string)
			mu.Lock()
			reason = msg + " (" + name + ")"
			mu.Unlock()
			return
		}
		iface, changed, ok := propertiesChanged(sig)
		if !ok || iface != ifaceDevice {
			return
		}
		if v, ok := changed["ServicesResolved"]; ok {
			if b, _ := v.Value().(bool); b {
				resolve.Do(func() { close(resolved) })
			}
		}
		if v, ok := changed["Connected"]; ok {
			if b, _ := v.Value().(bool); !b {
				dropOnce.Do(func() {
					mu.Lock()
					defer mu.Unlock()
					if reason == "" {
						reason = "unknown"
					}
					drops <- reason
				})
			}
		}
	})
	if err := d.connect(ctx, resolved, drops); err != nil {
		// BlueZ keeps trying after the call is abandoned, and would refuse
		// the next attempt with "In Progress"; disconnecting cancels it.
		_ = d.Disconnect()
		return nil, err
	}
	return drops, nil
}

func (d *Device) connect(ctx context.Context, resolved <-chan struct{}, dropped <-chan string) error {
	if err := d.obj().CallWithContext(ctx, ifaceDevice+".Connect", 0).Err; err != nil {
		return err
	}
	// Already resolved, e.g. if it was still connected, so no signal comes.
	if v, err := d.prop("ServicesResolved"); err == nil {
		if b, _ := v.Value().(bool); b {
			return nil
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-resolved:
		return nil
	case reason := <-dropped:
		return fmt.Errorf("%w: %s", ErrDropped, reason)
	}
}

// Disconnect drops the connection and stops watching the device and its
// characteristics.
func (d *Device) Disconnect() error {
	d.c.unwatch(d.path)
	return d.c.call(d.obj(), ifaceDevice+".Disconnect").Err
}

// Characteristic is a GATT characteristic of a connected device.
type Characteristic struct {
	d       *Device
	path    dbus.ObjectPath
	uuid    string
	service string
	flags   []string
}

// Characteristics lists the device's characteristics.
func (d *Device) Characteristics() ([]*Characteristic, error) {
	objs, err := d.c.objects()
	if err != nil {
		return nil, err
	}
	prefix := string(d.path) + "/"
	var chars []*Characteristic
	for _, p := range slices.Sorted(maps.Keys(objs)) {
		props, ok := objs[p][ifaceCharacteristic]
		if !ok || !strings.HasPrefix(string(p), prefix) {
			continue
		}
		svcPath, _ := props["Service"].Value().(dbus.ObjectPath)
		chars = append(chars, &Characteristic{
			d:       d,
			path:    p,
			uuid:    str(props["UUID"]),
			service: str(objs[svcPath][ifaceService]["UUID"]),
			flags:   strs(props["Flags"]),
		})
	}
	return chars, nil
}

// UUID returns the characteristic's UUID.
func (ch *Characteristic) UUID() string { return ch.uuid }

// Service returns the UUID of the characteristic's service.
func (ch *Characteristic) Service() string { return ch.service }

// Has reports whether the characteristic has the BlueZ flag, e.g. "notify".
func (ch *Characteristic) Has(flag string) bool { return slices.Contains(ch.flags, flag) }

func (ch *Characteristic) obj() dbus.BusObject { return ch.d.c.conn.Object(service, ch.path) }

// Read reads the characteristic's value.
func (ch *Characteristic) Read(ctx context.Context) ([]byte, error) {
	var value []byte
	err := ch.obj().CallWithContext(ctx, ifaceCharacteristic+".ReadValue", 0, map[string]any{}).Store(&value)
	return value, err
}

// Write writes value, with a response when the characteristic supports it.
func (ch *Characteristic) Write(ctx context.Context, value []byte) error {
	kind := "command"
	if ch.Has("write") {
		kind = "request"
	}
	return ch.obj().CallWithContext(ctx, ifaceCharacteristic+".WriteValue", 0, value, map[string]any{"type": kind}).Err
}

// StartNotify turns on notifications or indications and calls f with each
// value until the device is disconnected. f must not block.
func (ch *Characteristic) StartNotify(ctx context.Context, f func([]byte)) error {
	ch.d.c.watch(ch.path, func(sig *dbus.Signal) {
		iface, changed, ok := propertiesChanged(sig)
		if !ok || iface != ifaceCharacteristic {
			return
		}
		if v, ok := changed["Value"]; ok {
			if b, ok := v.Value().([]byte); ok {
				f(b)
			}
		}
	})
	if err := ch.obj().CallWithContext(ctx, ifaceCharacteristic+".StartNotify", 0).Err; err != nil {
		ch.d.c.unwatch(ch.path)
		return err
	}
	return nil
}
