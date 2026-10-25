package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/jview"
	"github.com/jmelahman/smart-lights/ble"
	"github.com/rivo/tview"
	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

const defaultBrightness = 8

func main() {
	app := tview.NewApplication()

	must("enable adapter", adapter.Enable())

	status := tview.NewTextView().SetLabel("Status").SetSize(1, 0).SetText("Scanning…")
	report := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		app.QueueUpdateDraw(func() { status.SetText(msg) })
	}

	scan := newScanner()
	ctrl := newController(report, scan.Restart)
	go ctrl.run()

	dropdown := tview.NewDropDown().SetLabel("Device")

	// The form is the source of truth: on connect, the device is set to match
	// it, since the lights' current state can't be read back.
	powerCheckbox := tview.NewCheckbox().
		SetLabel("Power").
		SetChecked(true).
		SetChangedFunc(ctrl.SetPower)

	slider := jview.NewSlider(0, ble.MaxBrightness, defaultBrightness)
	slider.SetLabel("Brightness").SetChangedFunc(ctrl.SetBrightness)

	colorGrid := jview.NewColorGrid(5, 8).SetLabel("Color").
		SetChangedFunc(func(idx int, color tcell.Color) {
			ctrl.SetColor(ColorToRGB(color))
		})

	form := tview.NewForm()
	form.
		AddFormItem(dropdown).
		AddFormItem(powerCheckbox).
		AddFormItem(slider).
		AddFormItem(colorGrid).
		AddFormItem(status).
		AddButton("Quit", func() { app.Stop() })

	go scan.run(app, dropdown, ctrl.Connect, report)

	app.SetRoot(form, true).SetFocus(dropdown).EnableMouse(true)
	if err := app.Run(); err != nil {
		panic(err)
	}
}

// scanner lists discovered lights in the dropdown. Scanning stops when a
// device is selected and resumes on Restart.
type scanner struct {
	mu      sync.Mutex
	seen    map[string]bool
	names   []string
	addrs   []bluetooth.Address
	restart chan struct{}
}

func newScanner() *scanner {
	return &scanner{seen: make(map[string]bool), restart: make(chan struct{}, 1)}
}

// Restart resumes scanning after it has been stopped.
func (s *scanner) Restart() {
	select {
	case s.restart <- struct{}{}:
	default:
	}
}

func (s *scanner) run(
	app *tview.Application,
	dropdown *tview.DropDown,
	onSelect func(bluetooth.Address),
	report func(string, ...any)) {
	for {
		// Returns once ble.StopScanning is called.
		err := ble.ScanForDevices(adapter, func(result ble.ScanResult) {
			addrStr := result.Address.String()
			s.mu.Lock()
			if s.seen[addrStr] {
				s.mu.Unlock()
				return
			}
			s.seen[addrStr] = true
			s.names = append(s.names, fmt.Sprintf("%s — %s", result.Name, addrStr))
			s.addrs = append(s.addrs, result.Address)
			// Snapshot under the lock; the UI reads these after it's released.
			names := append([]string(nil), s.names...)
			addrs := append([]bluetooth.Address(nil), s.addrs...)
			s.mu.Unlock()

			app.QueueUpdateDraw(func() {
				dropdown.SetOptions(names, func(text string, index int) {
					if index >= 0 && index < len(addrs) {
						onSelect(addrs[index])
					}
				})
			})
		})
		if err != nil {
			report("Scanning failed: %v", err)
		}
		<-s.restart
		report("Scanning…")
	}
}

type rgb struct{ r, g, b int32 }

// state is the light state requested through the form.
type state struct {
	power      bool
	brightness int
	color      rgb
	hasColor   bool
}

// controller performs device I/O on its own goroutine so BLE calls never
// block the UI. Only the latest requested state is sent, so dragging the
// slider doesn't queue up a write per step.
type controller struct {
	report        func(string, ...any)
	onConnectFail func()

	mu        sync.Mutex
	want      state
	dirty     bool
	connectTo *bluetooth.Address
	wake      chan struct{}

	dev *ble.Device // owned by run
}

func newController(report func(string, ...any), onConnectFail func()) *controller {
	return &controller{
		report:        report,
		onConnectFail: onConnectFail,
		want:          state{power: true, brightness: defaultBrightness},
		wake:          make(chan struct{}, 1),
	}
}

func (c *controller) update(f func()) {
	c.mu.Lock()
	f()
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Connect disconnects from the current device, if any, and connects to addr.
func (c *controller) Connect(addr bluetooth.Address) {
	c.update(func() { c.connectTo = &addr })
}

func (c *controller) SetPower(on bool) {
	c.update(func() { c.want.power, c.dirty = on, true })
}

func (c *controller) SetBrightness(level int) {
	c.update(func() { c.want.brightness, c.dirty = level, true })
}

func (c *controller) SetColor(r, g, b int32) {
	c.update(func() { c.want.color, c.want.hasColor, c.dirty = rgb{r, g, b}, true, true })
}

func (c *controller) run() {
	var sent sentState
	for range c.wake {
		c.mu.Lock()
		want, dirty, addr := c.want, c.dirty, c.connectTo
		c.dirty, c.connectTo = false, nil
		c.mu.Unlock()

		if addr != nil {
			if !c.connect(*addr) {
				continue
			}
			// Bring the new device in line with the form.
			sent, dirty = sentState{}, true
		}
		if c.dev == nil || !dirty {
			continue
		}
		c.apply(want, &sent)
	}
}

func (c *controller) connect(addr bluetooth.Address) bool {
	if c.dev != nil {
		_ = c.dev.Disconnect()
		c.dev = nil
	}
	// Stop scanning before connecting; errors if the scan already stopped.
	_ = ble.StopScanning(adapter)

	c.report("Connecting to %s…", addr)
	dev, err := ble.ConnectAndDiscover(adapter, addr)
	if err != nil {
		c.report("Failed to connect to %s: %v", addr, err)
		c.onConnectFail()
		return false
	}
	c.dev = dev
	c.report("Connected to %s", addr)
	return true
}

// sentState is what the device was last set to; nil fields are unknown.
type sentState struct {
	power      *bool
	brightness *int
	color      *rgb
}

// apply sends the fields of want that the device may not match. Power goes
// last so it decides the final on/off state.
func (c *controller) apply(want state, sent *sentState) {
	if sent.brightness == nil || *sent.brightness != want.brightness {
		sent.brightness = nil
		if err := c.dev.SetBrightness(want.brightness); err != nil {
			c.report("Failed to set brightness: %v", err)
		} else {
			sent.brightness = &want.brightness
		}
	}
	if want.hasColor && (sent.color == nil || *sent.color != want.color) {
		sent.color = nil
		if err := c.dev.SetRGB(want.color.r, want.color.g, want.color.b); err != nil {
			c.report("Failed to set color: %v", err)
		} else {
			sent.color = &want.color
		}
	}
	if sent.power == nil || *sent.power != want.power {
		sent.power = nil
		if err := c.dev.Power(want.power); err != nil {
			c.report("Failed to set power: %v", err)
		} else {
			sent.power = &want.power
		}
	}
}

func ColorToRGB(c tcell.Color) (r, g, b int32) {
	if c == tcell.ColorDefault {
		return 0, 0, 0
	}

	// tcell.Color contains 24-bit RGB in the lower 24 bits
	r = int32((c >> 16) & 0xFF)
	g = int32((c >> 8) & 0xFF)
	b = int32(c & 0xFF)
	return
}

func must(msg string, err error) {
	if err != nil {
		log.Fatalf("failed to "+msg+": %v", err)
	}
}
