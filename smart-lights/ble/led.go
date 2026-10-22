package ble

import (
	"errors"
	"fmt"

	"tinygo.org/x/bluetooth"
)

// MaxBrightness is the highest level accepted by SetBrightness.
const MaxBrightness = 15

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
	// ServiceUUID is the Generic Access service (0x1800), not a vendor service.
	// The lights advertise it, but so do other devices, so a match is only a
	// candidate: ConnectAndDiscover rejects devices without the write
	// characteristic.
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

	d := &Device{adapter: adapter, address: address}
	if err := d.connect(); err != nil {
		return nil, err
	}
	return d, nil
}

// connect opens a connection and discovers the write characteristic,
// disconnecting again if the device turns out not to be usable.
func (d *Device) connect() error {
	dev, err := d.adapter.Connect(d.address, bluetooth.ConnectionParams{})
	if err != nil {
		return err
	}
	d.dev = &dev
	d.writeChar = nil
	if err := d.discover(); err != nil {
		_ = dev.Disconnect()
		return err
	}
	return nil
}

func (d *Device) discover() error {
	svcs, err := d.dev.DiscoverServices(nil)
	if err != nil {
		return err
	}
	// Some services may refuse characteristic discovery; only report those
	// errors if the write characteristic isn't found elsewhere.
	var errs []error
	for _, s := range svcs {
		chs, err := s.DiscoverCharacteristics(nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, ch := range chs {
			if ch.UUID() == writeCharUUID {
				d.writeChar = &ch
				return nil
			}
		}
	}
	return fmt.Errorf("write characteristic %s not found: %w", writeCharUUID, errors.Join(errs...))
}

// Disconnect closes the connection to the device.
func (d *Device) Disconnect() error {
	return d.dev.Disconnect()
}

// write sends cmd, reconnecting first if the connection has dropped.
func (d *Device) write(cmd []byte) error {
	connected, err := d.dev.Connected()
	if err != nil || !connected || d.writeChar == nil {
		if err := d.connect(); err != nil {
			return fmt.Errorf("reconnect: %w", err)
		}
	}
	_, err = d.writeChar.WriteWithoutResponse(cmd)
	return err
}

// command frames a payload as: 0xA0, op, total length, payload, CRC16 (LE).
func command(op byte, payload ...byte) []byte {
	cmd := append([]byte{0xA0, op, byte(3 + len(payload))}, payload...)
	crc := crc16Modbus(cmd)
	return append(cmd, byte(crc&0xFF), byte(crc>>8))
}

// Power toggles the LED power state.
func (d *Device) Power(on bool) error {
	var state byte
	if on {
		state = 0x01
	}
	return d.write(command(0x11, state))
}

// brightnessLevels maps levels 0..MaxBrightness to the device's values.
var brightnessLevels = [MaxBrightness + 1]byte{
	0x01, 0x0C, 0x14, 0x16, 0x1B, 0x1E, 0x22, 0x29,
	0x2A, 0x32, 0x3D, 0x49, 0x4E, 0x58, 0x5A, 0x64,
}

func (d *Device) SetBrightness(level int) error {
	if level < 0 || level > MaxBrightness {
		return fmt.Errorf("brightness %d out of range [0, %d]", level, MaxBrightness)
	}
	return d.write(command(0x13, brightnessLevels[level]))
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
	return d.write(command(0x15, byte(r&0xFF), byte(g&0xFF), byte(b&0xFF)))
}
