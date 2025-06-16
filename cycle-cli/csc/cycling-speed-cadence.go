package csc

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type CSCMeasurement struct {
	Flags          uint8
	WheelRevs      *uint32
	WheelEventTime *uint16 // in 1/1024s
	CrankRevs      *uint16
	CrankEventTime *uint16 // in 1/1024s
}

func ParseCSCMeasurement(data []byte) (*CSCMeasurement, error) {
	if len(data) < 1 {
		return nil, errors.New("data too short")
	}

	offset := 1
	flags := data[0]
	csc := &CSCMeasurement{Flags: flags}

	if flags&0x01 != 0 {
		if offset+6 > len(data) {
			return nil, errors.New("wheel revolution data missing or incomplete")
		}
		wheelRevs := binary.LittleEndian.Uint32(data[offset:])
		wheelTime := binary.LittleEndian.Uint16(data[offset+4:])
		csc.WheelRevs = &wheelRevs
		csc.WheelEventTime = &wheelTime
		offset += 6
	}

	if flags&0x02 != 0 {
		if offset+4 > len(data) {
			return nil, errors.New("crank revolution data missing or incomplete")
		}
		crankRevs := binary.LittleEndian.Uint16(data[offset:])
		crankTime := binary.LittleEndian.Uint16(data[offset+2:])
		csc.CrankRevs = &crankRevs
		csc.CrankEventTime = &crankTime
	}

	return csc, nil
}

func (c *CSCMeasurement) String() string {
	var s string
	if c.WheelRevs != nil && c.WheelEventTime != nil {
		s += fmt.Sprintf("WheelRevs: %d @ %d, ", *c.WheelRevs, *c.WheelEventTime)
	}
	if c.CrankRevs != nil && c.CrankEventTime != nil {
		s += fmt.Sprintf("CrankRevs: %d @ %d, ", *c.CrankRevs, *c.CrankEventTime)
	}

	// Remove trailing comma if present
	if len(s) > 0 {
		s = s[:len(s)-2]
	}

	return s
}
