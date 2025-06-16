package ftms

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// IndoorBikeData holds the parsed values
type IndoorBikeData struct {
	Flags                uint16
	InstantaneousSpeed   *uint16
	AverageSpeed         *uint16
	InstantaneousCadence *uint16
	AverageCadence       *uint16
	TotalDistance        *uint32
	ResistanceLevel      *int16
	InstantaneousPower   *int16
	AveragePower         *int16
	TotalEnergy          *uint16
	EnergyPerHour        *uint16
	EnergyPerMinute      *uint8
	HeartRate            *uint8
	MetabolicEquivalent  *uint8
	ElapsedTime          *uint16
	RemainingTime        *uint16
}

// ParseIndoorBikeData decodes raw bytes into IndoorBikeData
func ParseIndoorBikeData(data []byte) (*IndoorBikeData, error) {
	if len(data) < 2 {
		return nil, errors.New("data too short")
	}

	offset := 2
	flags := binary.LittleEndian.Uint16(data[0:2])
	d := &IndoorBikeData{Flags: flags}

	readU8 := func() (uint8, error) {
		if offset >= len(data) {
			return 0, errors.New("unexpected end of data")
		}
		v := data[offset]
		offset++
		return v, nil
	}

	readU16 := func() (uint16, error) {
		if offset+2 > len(data) {
			return 0, errors.New("unexpected end of data")
		}
		v := binary.LittleEndian.Uint16(data[offset:])
		offset += 2
		return v, nil
	}

	readS16 := func() (int16, error) {
		v, err := readU16()
		return int16(v), err
	}

	readU24 := func() (uint32, error) {
		if offset+3 > len(data) {
			return 0, errors.New("unexpected end of data")
		}
		v := uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16
		offset += 3
		return v, nil
	}

	if flags&(1<<0) != 0 {
		if v, err := readU16(); err == nil {
			d.InstantaneousSpeed = &v
		}
	}

	if flags&(1<<1) != 0 {
		if v, err := readU16(); err == nil {
			d.AverageSpeed = &v
		}
	}
	if flags&(1<<2) != 0 {
		if v, err := readU16(); err == nil {
			d.InstantaneousCadence = &v
		}
	}
	if flags&(1<<3) != 0 {
		if v, err := readU16(); err == nil {
			d.AverageCadence = &v
		}
	}
	if flags&(1<<4) != 0 {
		if v, err := readU24(); err == nil {
			d.TotalDistance = &v
		}
	}
	if flags&(1<<5) != 0 {
		if v, err := readS16(); err == nil {
			d.ResistanceLevel = &v
		}
	}
	if flags&(1<<6) != 0 {
		if v, err := readS16(); err == nil {
			d.InstantaneousPower = &v
		}
	}
	if flags&(1<<7) != 0 {
		if v, err := readS16(); err == nil {
			d.AveragePower = &v
		}
	}
	if flags&(1<<8) != 0 {
		if v, err := readU16(); err == nil {
			d.TotalEnergy = &v
		}
	}
	if flags&(1<<9) != 0 {
		if v, err := readU16(); err == nil {
			d.EnergyPerHour = &v
		}
	}
	if flags&(1<<10) != 0 {
		if v, err := readU8(); err == nil {
			d.EnergyPerMinute = &v
		}
	}
	if flags&(1<<11) != 0 {
		if v, err := readU8(); err == nil {
			d.HeartRate = &v
		}
	}
	if flags&(1<<12) != 0 {
		if v, err := readU8(); err == nil {
			d.MetabolicEquivalent = &v
		}
	}
	if flags&(1<<13) != 0 {
		if v, err := readU16(); err == nil {
			d.ElapsedTime = &v
		}
	}
	if flags&(1<<14) != 0 {
		if v, err := readU16(); err == nil {
			d.RemainingTime = &v
		}
	}

	return d, nil
}

func (d *IndoorBikeData) String() string {
	var s string

	if d.InstantaneousSpeed != nil {
		s += fmt.Sprintf("Speed: %d km/h, ", *d.InstantaneousSpeed)
	}
	if d.AverageSpeed != nil {
		s += fmt.Sprintf("Avg Speed: %d km/h, ", *d.AverageSpeed)
	}
	if d.InstantaneousCadence != nil {
		s += fmt.Sprintf("Cadence: %d rpm, ", *d.InstantaneousCadence)
	}
	if d.AverageCadence != nil {
		s += fmt.Sprintf("Avg Cadence: %d rpm, ", *d.AverageCadence)
	}
	if d.TotalDistance != nil {
		s += fmt.Sprintf("Distance: %d m, ", *d.TotalDistance)
	}
	// NOTE: This is interestingly stuck at 20W.
	// if d.InstantaneousPower != nil {
	// 	s += fmt.Sprintf("Power: %d W, ", *d.InstantaneousPower)
	// }
	if d.AveragePower != nil {
		s += fmt.Sprintf("Avg Power: %d W, ", *d.AveragePower)
	}
	if d.TotalEnergy != nil {
		s += fmt.Sprintf("Energy: %d kJ, ", *d.TotalEnergy)
	}
	if d.HeartRate != nil {
		s += fmt.Sprintf("HR: %d bpm, ", *d.HeartRate)
	}
	if d.ElapsedTime != nil {
		s += fmt.Sprintf("Elapsed: %d s, ", *d.ElapsedTime)
	}
	if d.RemainingTime != nil {
		s += fmt.Sprintf("Remaining: %d s, ", *d.RemainingTime)
	}

	// Remove trailing comma if present
	if len(s) > 0 {
		s = s[:len(s)-2]
	}

	return s
}
