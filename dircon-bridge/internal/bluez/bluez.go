// Package bluez is a small client for the parts of BlueZ's D-Bus API that a
// GATT relay needs: finding and connecting to LE devices, listing their
// characteristics, and reading, writing and subscribing to them.
package bluez

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	service            = "org.bluez"
	ifaceAdapter       = "org.bluez.Adapter1"
	ifaceDevice        = "org.bluez.Device1"
	ifaceService       = "org.bluez.GattService1"
	ifaceCharacteristc = "org.bluez.GattCharacteristic1"
	ifaceProperties    = "org.freedesktop.DBus.Properties"
	ifaceObjectManager = "org.freedesktop.DBus.ObjectManager"
)

// ErrNoAdapter is returned when the host has no Bluetooth adapter.
var ErrNoAdapter = errors.New("no Bluetooth adapter")

type objects = map[dbus.ObjectPath]map[string]map[string]dbus.Variant

// Client is a connection to BlueZ on the system bus.
type Client struct {
	conn *dbus.Conn
	// Object path -> callback for its PropertiesChanged signals.
	watchers map[dbus.ObjectPath]func(iface string, changed map[string]dbus.Variant)
	// Callbacks for objects that are seen advertising during discovery.
	seen    map[int]func(dbus.ObjectPath)
	adapter dbus.ObjectPath
	mu      sync.Mutex
	nextID  int
	// Discovery runs while anyone needs it; this counts them.
	discovering int
}

// New connects to the system bus and picks the first adapter.
func New() (*Client, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:     conn,
		watchers: map[dbus.ObjectPath]func(string, map[string]dbus.Variant){},
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
	} {
		if err := conn.AddMatchSignal(append(opt, dbus.WithMatchSender(service))...); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	signals := make(chan *dbus.Signal, 256)
	conn.Signal(signals)
	go c.dispatch(signals)
	return c, nil
}

// Close disconnects from the bus.
func (c *Client) Close() error { return c.conn.Close() }

func (c *Client) objects() (objects, error) {
	var objs objects
	err := c.conn.Object(service, "/").Call(ifaceObjectManager+".GetManagedObjects", 0).Store(&objs)
	return objs, err
}

func (c *Client) dispatch(signals <-chan *dbus.Signal) {
	for sig := range signals {
		switch sig.Name {
		case ifaceProperties + ".PropertiesChanged":
			if len(sig.Body) < 2 {
				continue
			}
			iface, _ := sig.Body[0].(string)
			changed, _ := sig.Body[1].(map[string]dbus.Variant)
			c.mu.Lock()
			watch := c.watchers[sig.Path]
			seen := c.seenCallbacks()
			c.mu.Unlock()
			if watch != nil {
				watch(iface, changed)
			}
			// An advertisement updates RSSI or the advertised data.
			if iface == ifaceDevice && hasAny(changed, "RSSI", "ManufacturerData", "ServiceData") {
				for _, f := range seen {
					f(sig.Path)
				}
			}
		case ifaceObjectManager + ".InterfacesAdded":
			if len(sig.Body) < 2 {
				continue
			}
			path, _ := sig.Body[0].(dbus.ObjectPath)
			ifaces, _ := sig.Body[1].(map[string]map[string]dbus.Variant)
			if _, ok := ifaces[ifaceDevice]; !ok {
				continue
			}
			c.mu.Lock()
			seen := c.seenCallbacks()
			c.mu.Unlock()
			for _, f := range seen {
				f(path)
			}
		}
	}
}

func hasAny(m map[string]dbus.Variant, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// seenCallbacks must be called with c.mu held.
func (c *Client) seenCallbacks() []func(dbus.ObjectPath) {
	return slices.Collect(maps.Values(c.seen))
}

func (c *Client) watch(path dbus.ObjectPath, f func(iface string, changed map[string]dbus.Variant)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if f == nil {
		delete(c.watchers, path)
	} else {
		c.watchers[path] = f
	}
}

// discover calls seen with each device path that advertises until the
// returned stop function is called. Concurrent calls share one discovery.
func (c *Client) discover(uuids []string, seen func(dbus.ObjectPath)) (stop func(), err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.discovering == 0 {
		adapter := c.conn.Object(service, c.adapter)
		filter := map[string]any{"Transport": "le", "DuplicateData": true}
		if len(uuids) > 0 {
			filter["UUIDs"] = uuids
		}
		if err := adapter.Call(ifaceAdapter+".SetDiscoveryFilter", 0, filter).Err; err != nil {
			return nil, fmt.Errorf("set discovery filter: %w", err)
		}
		if err := adapter.Call(ifaceAdapter+".StartDiscovery", 0).Err; err != nil {
			return nil, fmt.Errorf("start discovery: %w", err)
		}
	}
	c.discovering++
	id := c.nextID
	c.nextID++
	c.seen[id] = seen
	var once sync.Once
	return func() {
		once.Do(func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			delete(c.seen, id)
			c.discovering--
			if c.discovering == 0 {
				_ = c.conn.Object(service, c.adapter).Call(ifaceAdapter+".StopDiscovery", 0).Err
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
	stop, err := c.discover(uuids, func(p dbus.ObjectPath) {
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

// Device is a remote LE device known to BlueZ.
type Device struct {
	c       *Client
	Path    dbus.ObjectPath
	Address string
}

// Device returns the device with the given address, scanning until it shows
// up if BlueZ has never seen it.
func (c *Client) Device(ctx context.Context, address string) (*Device, error) {
	address = strings.ToUpper(address)
	path := c.adapter + dbus.ObjectPath("/dev_"+strings.ReplaceAll(address, ":", "_"))
	d := &Device{c: c, Path: path, Address: address}
	if c.known(path) {
		return d, nil
	}
	found := make(chan struct{}, 1)
	stop, err := c.discover(nil, func(p dbus.ObjectPath) {
		if p == path {
			select {
			case found <- struct{}{}:
			default:
			}
		}
	})
	if err != nil {
		return nil, err
	}
	defer stop()
	// The device may have appeared between the check and the discovery.
	if c.known(path) {
		return d, nil
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-found:
		return d, nil
	}
}

func (c *Client) known(path dbus.ObjectPath) bool {
	objs, err := c.objects()
	if err != nil {
		return false
	}
	_, ok := objs[path][ifaceDevice]
	return ok
}

func (d *Device) obj() dbus.BusObject { return d.c.conn.Object(service, d.Path) }

func (d *Device) prop(name string) (dbus.Variant, error) {
	return d.obj().GetProperty(ifaceDevice + "." + name)
}

// Name returns the device's name, or its address if it has none.
func (d *Device) Name() string {
	if v, err := d.prop("Alias"); err == nil && str(v) != "" {
		return str(v)
	}
	return d.Address
}

// Connect connects to the device and waits until its services are resolved.
// BlueZ waits for an LE device to advertise, so ctx bounds that wait.
// onDisconnect is called once if the connection later drops.
func (d *Device) Connect(ctx context.Context, onDisconnect func()) error {
	var once sync.Once
	d.c.watch(d.Path, func(iface string, changed map[string]dbus.Variant) {
		if v, ok := changed["Connected"]; ok && iface == ifaceDevice {
			if connected, _ := v.Value().(bool); !connected {
				once.Do(onDisconnect)
			}
		}
	})
	if err := d.obj().CallWithContext(ctx, ifaceDevice+".Connect", 0).Err; err != nil {
		return err
	}
	for {
		if v, err := d.prop("ServicesResolved"); err == nil {
			if resolved, _ := v.Value().(bool); resolved {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Disconnect drops the connection and stops watching it.
func (d *Device) Disconnect() error {
	d.c.watch(d.Path, nil)
	return d.obj().Call(ifaceDevice+".Disconnect", 0).Err
}

// Characteristic is a GATT characteristic of a connected device.
type Characteristic struct {
	d       *Device
	Path    dbus.ObjectPath
	UUID    string
	Service string
	Flags   []string
}

// Characteristics lists the device's characteristics.
func (d *Device) Characteristics() ([]*Characteristic, error) {
	objs, err := d.c.objects()
	if err != nil {
		return nil, err
	}
	prefix := string(d.Path) + "/"
	var chars []*Characteristic
	for _, p := range slices.Sorted(maps.Keys(objs)) {
		props, ok := objs[p][ifaceCharacteristc]
		if !ok || !strings.HasPrefix(string(p), prefix) {
			continue
		}
		svcPath, _ := props["Service"].Value().(dbus.ObjectPath)
		chars = append(chars, &Characteristic{
			d:       d,
			Path:    p,
			UUID:    str(props["UUID"]),
			Service: str(objs[svcPath][ifaceService]["UUID"]),
			Flags:   strs(props["Flags"]),
		})
	}
	return chars, nil
}

func (ch *Characteristic) obj() dbus.BusObject { return ch.d.c.conn.Object(service, ch.Path) }

// Has reports whether the characteristic has the BlueZ flag, e.g. "notify".
func (ch *Characteristic) Has(flag string) bool { return slices.Contains(ch.Flags, flag) }

// Read reads the characteristic's value.
func (ch *Characteristic) Read(ctx context.Context) ([]byte, error) {
	var value []byte
	err := ch.obj().CallWithContext(ctx, ifaceCharacteristc+".ReadValue", 0, map[string]any{}).Store(&value)
	return value, err
}

// Write writes the value, with a response when the characteristic supports it.
func (ch *Characteristic) Write(ctx context.Context, value []byte) error {
	kind := "command"
	if ch.Has("write") {
		kind = "request"
	}
	return ch.obj().CallWithContext(ctx, ifaceCharacteristc+".WriteValue", 0, value, map[string]any{"type": kind}).Err
}

// StartNotify enables notifications or indications and calls f with each value.
func (ch *Characteristic) StartNotify(f func([]byte)) error {
	ch.d.c.watch(ch.Path, func(iface string, changed map[string]dbus.Variant) {
		if v, ok := changed["Value"]; ok && iface == ifaceCharacteristc {
			if b, ok := v.Value().([]byte); ok {
				f(b)
			}
		}
	})
	if err := ch.obj().Call(ifaceCharacteristc+".StartNotify", 0).Err; err != nil {
		ch.d.c.watch(ch.Path, nil)
		return err
	}
	return nil
}
