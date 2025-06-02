package ftms

import (
	"fmt"
)

// FitnessMachineStatus represents a parsed status notification
type FitnessMachineStatus struct {
	Opcode byte
	Params []byte
}

func ParseFitnessMachineStatus(data []byte) (*FitnessMachineStatus, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("data too short to contain status")
	}

	opcode := data[0]
	params := data[1:]

	return &FitnessMachineStatus{
		Opcode: opcode,
		Params: params,
	}, nil
}

// Optional: interpret opcodes into human-readable strings
func (s *FitnessMachineStatus) OpcodeString() string {
	switch s.Opcode {
	case 0x01:
		return "Reset"
	case 0x02:
		return "Stopped or Paused by User"
	case 0x03:
		return "Stopped by Safety Key"
	case 0x04:
		return "Started or Resumed by User"
	case 0x05:
		return "Target Speed Changed"
	case 0x06:
		return "Target Incline Changed"
	case 0x07:
		return "Target Resistance Level Changed"
	case 0x08:
		return "Target Power Changed"
	case 0x09:
		return "Target Heart Rate Changed"
	case 0x0A:
		return "Indoor Bike Simulation Parameters Changed"
	case 0x0B:
		return "Spin Down Status"
	default:
		return fmt.Sprintf("Unknown Opcode 0x%02X", s.Opcode)
	}
}
