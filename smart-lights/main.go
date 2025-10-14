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
var device *ble.Device

func main() {
	app := tview.NewApplication()

	must("enable adapter", adapter.Enable())

	// store discovered addresses to avoid duplicates
	devices := make(map[string]bluetooth.Address)
	var values []string
	var dropdown *tview.DropDown
	var powerCheckbox *tview.Checkbox
	var deviceAddresses []bluetooth.Address
	var mutex sync.Mutex

	dropdown = tview.NewDropDown().SetLabel("Device")

	powerCheckbox = tview.NewCheckbox().
		SetLabel("Power").
		SetChecked(false).
		SetChangedFunc(func(checked bool) {
			if device != nil {
				if err := device.Power(checked); err != nil {
					log.Printf("Failed to power device: %v", err)
				}
			}
		})

	slider := jview.NewSlider(0, 15, 8)
	slider.SetLabel("Brightness").SetChangedFunc(func(v int) {
		if device != nil {
			if err := device.SetBrightness(v); err != nil {
				log.Printf("Failed to power device: %v", err)
			}
		}
	})

	colorGrid := jview.NewColorGrid(5, 8).SetLabel("Color").
		SetChangedFunc(func(idx int, color tcell.Color) {
			if device != nil {
				if err := device.SetRGB(ColorToRGB(color)); err != nil {
					log.Printf("Failed to power device: %v", err)
				}
			}
		})

	form := tview.NewForm()
	form.
		AddFormItem(dropdown).
		AddFormItem(powerCheckbox).
		AddFormItem(slider).
		AddFormItem(colorGrid).
		AddButton("Quit", func() { app.Stop() })

	// Start scanning for devices
	go startScanning(app, dropdown, devices, &values, &deviceAddresses, &mutex)

	app.SetRoot(form, true).SetFocus(dropdown).EnableMouse(true)
	if err := app.Run(); err != nil {
		panic(err)
	}
}

func startScanning(
	app *tview.Application,
	dropdown *tview.DropDown,
	devices map[string]bluetooth.Address,
	values *[]string,
	deviceAddresses *[]bluetooth.Address,
	mutex *sync.Mutex) {

	err := ble.ScanForDevices(adapter, func(result ble.ScanResult) {
		addrStr := result.Address.String()
		mutex.Lock()
		if _, ok := devices[addrStr]; ok {
			// already added
			mutex.Unlock()
			return
		}
		devices[addrStr] = result.Address

		display := fmt.Sprintf("%s — %s", result.Name, addrStr)
		*values = append(*values, display)
		*deviceAddresses = append(*deviceAddresses, result.Address)

		// Update dropdown options in the UI thread
		app.QueueUpdateDraw(func() {
			dropdown.SetOptions(*values, func(text string, index int) {
				if index >= 0 && index < len(*deviceAddresses) {
					// Stop scanning before connecting
					_ = ble.StopScanning(adapter)

					newDevice, err := ble.ConnectAndDiscover(adapter, (*deviceAddresses)[index])
					if err != nil {
						log.Printf("Failed to connect to device: %v", err)
						return
					}
					device = newDevice
				}
			})
		})
		mutex.Unlock()
	})
	if err != nil {
		log.Printf("Scanning failed: %v", err)
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
