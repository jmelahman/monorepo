package bridge

import "github.com/jmelahman/dircon-bridge/internal/bluez"

// FromBlueZ adapts a BlueZ device to Device.
func FromBlueZ(d *bluez.Device) Device { return blueZDevice{d} }

type blueZDevice struct{ *bluez.Device }

func (d blueZDevice) Characteristics() ([]Characteristic, error) {
	chars, err := d.Device.Characteristics()
	if err != nil {
		return nil, err
	}
	out := make([]Characteristic, len(chars))
	for i, ch := range chars {
		out[i] = ch
	}
	return out, nil
}
