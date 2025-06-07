package cps

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type CyclingPowerMeasurement struct {
	Flags uint16

	InstantaneousPower int16

	PedalPowerBalance     *uint8
	AccumulatedTorque     *uint16
	WheelRevs             *uint32
	WheelEventTime        *uint16
	CrankRevs             *uint16
	CrankEventTime        *uint16
	ExtremeForceMin       *int16
	ExtremeForceMax       *int16
	ExtremeTorqueMin      *int16
	ExtremeTorqueMax      *int16
	ExtremeAngles         *[2]uint16 // degrees (min, max)
	TopDeadSpotAngle      *uint16
	BottomDeadSpotAngle   *uint16
	AccumulatedEnergy     *uint8
	OffsetCompensationSet bool
}

func ParseCyclingPowerMeasurement(data []byte) (*CyclingPowerMeasurement, error) {
	if len(data) < 4 {
		return nil, errors.New("data too short")
	}

	cpm := &CyclingPowerMeasurement{}
	cpm.Flags = binary.LittleEndian.Uint16(data[0:2])
	cpm.InstantaneousPower = int16(binary.LittleEndian.Uint16(data[2:4]))
	offset := 4

	readU8 := func() (uint8, error) {
		if offset+1 > len(data) {
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

	readU32 := func() (uint32, error) {
		if offset+4 > len(data) {
			return 0, errors.New("unexpected end of data")
		}
		v := binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		return v, nil
	}

	flags := cpm.Flags

	if flags&(1<<0) != 0 {
		if v, err := readU8(); err == nil {
			cpm.PedalPowerBalance = &v
		}
	}

	if flags&(1<<2) != 0 {
		if v, err := readU16(); err == nil {
			cpm.AccumulatedTorque = &v
		}
	}

	if flags&(1<<4) != 0 {
		if revs, err := readU32(); err == nil {
			if evt, err := readU16(); err == nil {
				cpm.WheelRevs = &revs
				cpm.WheelEventTime = &evt
			}
		}
	}

	if flags&(1<<5) != 0 {
		if revs, err := readU16(); err == nil {
			if evt, err := readU16(); err == nil {
				cpm.CrankRevs = &revs
				cpm.CrankEventTime = &evt
			}
		}
	}

	if flags&(1<<6) != 0 {
		if min, err := readS16(); err == nil {
			if max, err := readS16(); err == nil {
				cpm.ExtremeForceMin = &min
				cpm.ExtremeForceMax = &max
			}
		}
	}

	if flags&(1<<7) != 0 {
		if min, err := readS16(); err == nil {
			if max, err := readS16(); err == nil {
				cpm.ExtremeTorqueMin = &min
				cpm.ExtremeTorqueMax = &max
			}
		}
	}

	if flags&(1<<8) != 0 {
		if angleBytes, err := readU16(); err == nil {
			// upper 12 bits = max angle, lower 12 bits = min angle
			maxAngle := uint16((angleBytes >> 12) & 0x0FFF)
			minAngle := uint16(angleBytes & 0x0FFF)
			cpm.ExtremeAngles = &[2]uint16{minAngle, maxAngle}
		}
	}

	if flags&(1<<9) != 0 {
		if v, err := readU16(); err == nil {
			cpm.TopDeadSpotAngle = &v
		}
	}

	if flags&(1<<10) != 0 {
		if v, err := readU16(); err == nil {
			cpm.BottomDeadSpotAngle = &v
		}
	}

	if flags&(1<<11) != 0 {
		if v, err := readU8(); err == nil {
			cpm.AccumulatedEnergy = &v
		}
	}

	if flags&(1<<12) != 0 {
		cpm.OffsetCompensationSet = true
	}

	return cpm, nil
}

func (d *CyclingPowerMeasurement) String() string {
	s := fmt.Sprintf("Power: %dW", d.InstantaneousPower)

	if d.PedalPowerBalance != nil {
		s += fmt.Sprintf(", Balance: %d%%", *d.PedalPowerBalance)
	}
	if d.AccumulatedTorque != nil {
		s += fmt.Sprintf(", Torque: %d", *d.AccumulatedTorque)
	}
	if d.WheelRevs != nil {
		s += fmt.Sprintf(", Wheel Revs: %d", *d.WheelRevs)
	}
	if d.WheelEventTime != nil {
		s += fmt.Sprintf(", Wheel Time: %d", *d.WheelEventTime)
	}
	if d.CrankRevs != nil {
		s += fmt.Sprintf(", Crank Revs: %d", *d.CrankRevs)
	}
	if d.CrankEventTime != nil {
		s += fmt.Sprintf(", Crank Time: %d", *d.CrankEventTime)
	}
	if d.ExtremeForceMin != nil && d.ExtremeForceMax != nil {
		s += fmt.Sprintf(", Force Min/Max: %d/%d", *d.ExtremeForceMin, *d.ExtremeForceMax)
	}
	if d.ExtremeTorqueMin != nil && d.ExtremeTorqueMax != nil {
		s += fmt.Sprintf(", Torque Min/Max: %d/%d", *d.ExtremeTorqueMin, *d.ExtremeTorqueMax)
	}
	if d.ExtremeAngles != nil {
		s += fmt.Sprintf(", Angles Min/Max: %d/%d°", d.ExtremeAngles[0], d.ExtremeAngles[1])
	}
	if d.TopDeadSpotAngle != nil {
		s += fmt.Sprintf(", Top Dead Angle: %d°", *d.TopDeadSpotAngle)
	}
	if d.BottomDeadSpotAngle != nil {
		s += fmt.Sprintf(", Bottom Dead Angle: %d°", *d.BottomDeadSpotAngle)
	}
	if d.AccumulatedEnergy != nil {
		s += fmt.Sprintf(", Energy: %dkJ", *d.AccumulatedEnergy)
	}
	if d.OffsetCompensationSet {
		s += ", OffsetComp"
	}

	return s
}
