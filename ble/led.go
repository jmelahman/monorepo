package ble

import (
	"fmt"
	"tinygo.org/x/bluetooth"
)

type Device struct {
	adapter   *bluetooth.Adapter
	address   bluetooth.Address
	dev       *bluetooth.Device
	writeChar *bluetooth.DeviceCharacteristic
}

type ScanResult struct {
	Address bluetooth.Address
	Name    string
}

var (
	ServiceUUID = bluetooth.NewUUID([16]byte{
		0x00, 0x00, 0x18, 0x00, 0x00, 0x00,
		0x10, 0x00, 0x80, 0x00, 0x00, 0x80,
		0x5f, 0x9b, 0x34, 0xfb,
	})
	writeCharUUID = bluetooth.NewUUID([16]byte{
		0x00, 0x00, 0xFF, 0x12, 0x00, 0x00,
		0x10, 0x00, 0x80, 0x00, 0x00, 0x80,
		0x5f, 0x9b, 0x34, 0xfb,
	})
)

// ScanForDevices scans for BLE devices advertising the specific service UUID.
// It calls the provided callback for each discovered device.
func ScanForDevices(adapter *bluetooth.Adapter, callback func(ScanResult)) error {
	return adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
		payload := result.AdvertisementPayload
		if payload == nil {
			return
		}
		// check advertisement contains the service UUID
		if !payload.HasServiceUUID(ServiceUUID) {
			return
		}

		name := payload.LocalName()
		if name == "" {
			name = "<unknown>"
		}

		callback(ScanResult{
			Address: result.Address,
			Name:    name,
		})
	})
}

// StopScanning stops the BLE scanning process.
func StopScanning(adapter *bluetooth.Adapter) error {
	return adapter.StopScan()
}

// ConnectAndDiscover connects to a device and discovers the control characteristic.
func ConnectAndDiscover(adapter *bluetooth.Adapter, address bluetooth.Address) (*Device, error) {
	if err := adapter.Enable(); err != nil {
		return nil, err
	}

	dev, err := adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return nil, err
	}

	d := &Device{dev: &dev, adapter: adapter, address: address}
	if err := d.discover(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Device) discover() error {
	svcs, err := d.dev.DiscoverServices(nil)
	if err != nil {
		return err
	}
	for _, s := range svcs {
		chs, _ := s.DiscoverCharacteristics(nil)
		for _, ch := range chs {
			if ch.UUID() == writeCharUUID {
				d.writeChar = &ch
				return nil
			}
		}
	}
	return nil
}

func (d *Device) Connect() error {
	if err := d.adapter.Enable(); err != nil {
		return err
	}

	dev, err := d.adapter.Connect(d.address, bluetooth.ConnectionParams{})
	if err != nil {
		return err
	}
	d.dev = &dev
	return nil
}

// Power toggles the LED power state.
func (d *Device) Power(on bool) error {
	if d.writeChar == nil {
		return fmt.Errorf("write characteristic undiscovered")
	}

	if err := d.Connect(); err != nil {
		return err
	}

	cmd := []byte{0xA0, 0x11, 0x04}
	if on {
		cmd = append(cmd, 0x01, 0xB1, 0x21) // on
	} else {
		cmd = append(cmd, 0x00, 0x70, 0xE1) // off
	}
	_, err := d.writeChar.WriteWithoutResponse(cmd)
	return err
}

func getLevelHex(level int) []byte {
	baseHex := [][]byte{
		{0x01, 0x10, 0xE1}, {0x0C, 0xD1, 0x24}, {0x14, 0xD1, 0x2E}, {0x16, 0x50, 0xEF},
		{0x1B, 0x91, 0x2A}, {0x1E, 0x51, 0x29}, {0x22, 0x51, 0x38}, {0x29, 0x10, 0xFF},
		{0x2A, 0x50, 0xFE}, {0x32, 0x50, 0xF4}, {0x3D, 0x10, 0xF0}, {0x49, 0x10, 0xD7},
		{0x4E, 0x51, 0x15}, {0x58, 0xD0, 0xDB}, {0x5A, 0x51, 0x1A}, {0x64, 0xD0, 0xCA},
	}

	return baseHex[level]
}

func (d *Device) SetBrightness(level int) error {
	if d.writeChar == nil {
		return fmt.Errorf("write characteristic undiscovered")
	}

	if err := d.Connect(); err != nil {
		return err
	}

	cmd := append([]byte{0xA0, 0x13, 0x04}, getLevelHex(level)...)
	_, err := d.writeChar.WriteWithoutResponse(cmd)
	return err
}

func crc16Modbus(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func (d *Device) SetRGB(r, g, b int32) error {
	if d.writeChar == nil {
		return fmt.Errorf("write characteristic undiscovered")
	}

	if err := d.Connect(); err != nil {
		return err
	}

	cmd := []byte{0xA0, 0x15, 0x06, byte(r & 0xFF), byte(g & 0xFF), byte(b & 0xFF)}
	crc := crc16Modbus(cmd)
	cmd = append(cmd, byte(crc&0xFF), byte(crc>>8))

	_, err := d.writeChar.WriteWithoutResponse(cmd)
	return err
}
