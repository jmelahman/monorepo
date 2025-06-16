package ble

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/jmelahman/cycle-cli/cps"
	"github.com/jmelahman/cycle-cli/csc"
	"github.com/jmelahman/cycle-cli/ftms"
	"github.com/jmelahman/cycle-cli/utils"
	log "github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

type Telemetry struct {
	Cadence     *uint16
	Speed       *uint16
	Power       uint16
	Revolutions uint16
	HR          uint8
}

type TelemetryHandler func(data Telemetry)

var adapter = bluetooth.DefaultAdapter

// SetResistance sets the trainer's resistance level (0-100%)
func SetResistance(dev *bluetooth.Device, level uint8) error {
	services, err := dev.DiscoverServices([]bluetooth.UUID{
		bluetooth.NewUUID([16]byte{0x18, 0x26, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB, 0x00, 0x00}), // FTMS Service
	})
	if err != nil {
		return fmt.Errorf("discover services: %w", err)
	}

	for _, srv := range services {
		chars, err := srv.DiscoverCharacteristics([]bluetooth.UUID{
			bluetooth.NewUUID([16]byte{0xAD, 0xD2, 0x00, 0x00, 0x10, 0x00, 0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB, 0x00, 0x00}), // Resistance Level Char
		})
		if err != nil {
			return fmt.Errorf("discover characteristics: %w", err)
		}

		for _, char := range chars {
			data := make([]byte, 2)
			binary.LittleEndian.PutUint16(data, uint16(level))
			_, err := char.WriteWithoutResponse(data)
			if err != nil {
				return fmt.Errorf("write resistance: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("resistance control characteristic not found")
}

// ConnectToTrainer scans for a trainer device and connects to it.
// It returns a connected bluetooth.Device object.
func ConnectToTrainer() (*bluetooth.Device, error) {
	utils.Must("enable BLE stack", adapter.Enable())

	var trainerAddr bluetooth.Address
	var device = bluetooth.Device{}

	utils.Must("scan", adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		name := result.LocalName()
		if isTrainerName(name) {
			fmt.Printf("✅ Found trainer: %s [%s]\n", name, result.Address.String())
			trainerAddr = result.Address
			adapter.StopScan()
		}
	}))

	device, err := adapter.Connect(trainerAddr, bluetooth.ConnectionParams{})
	utils.Must("connect to device", err)

	return &device, nil
}

func SubscribeToMetrics(dev *bluetooth.Device, state Telemetry, handler TelemetryHandler) error {
	services, err := dev.DiscoverServices(nil)
	utils.Must("discover services", err)

	var powerChar, hrChar, bikeChar, cscChar, fitnessChar *bluetooth.DeviceCharacteristic

	for _, srv := range services {
		chars, err := srv.DiscoverCharacteristics(nil)
		if err != nil {
			return fmt.Errorf("discover characteristics: %w", err)
		}
		for _, char := range chars {
			switch char.UUID().String() {
			case "00002ad2-0000-1000-8000-00805f9b34fb":
				log.Debug("Registered: Indoor Bike Data")
				bikeChar = &char
			case "00002a5b-0000-1000-8000-00805f9b34fb":
				log.Debug("Registered: Cycling Speed and Cadence (CSC) Measurement")
				cscChar = &char
			case "00002ada-0000-1000-8000-00805f9b34fb":
				log.Debug("Registered: Fitness Machine Status")
				fitnessChar = &char
			case "00002a37-0000-1000-8000-00805f9b34fb":
				log.Debug("Registered: Heart Rate Measurement")
				hrChar = &char
			case "00002a63-0000-1000-8000-00805f9b34fb":
				log.Debug("Registered: Cycling Power Measurement")
				powerChar = &char
			}
		}
	}

	if bikeChar != nil {
		err = bikeChar.EnableNotifications(func(buf []byte) {
			data, err := ftms.ParseIndoorBikeData(buf)
			utils.Must("parse indoor bike data", err)

			log.Debug(data)

			handler(state)
		})
	}

	if cscChar != nil {
		err = cscChar.EnableNotifications(func(buf []byte) {
			data, err := csc.ParseCSCMeasurement(buf)
			utils.Must("parse cycling speed and cadence", err)

			log.Debug(data)

			handler(state)
		})
	}

	if fitnessChar != nil {
		err = fitnessChar.EnableNotifications(func(buf []byte) {
			status, err := ftms.ParseFitnessMachineStatus(buf)
			utils.Must("parse fitness machine status", err)

			fmt.Printf("Status: %s\n", status.OpcodeString())
			if status.Opcode == 0x08 && len(status.Params) >= 2 {
				targetPower := binary.LittleEndian.Uint16(status.Params[:2])
				fmt.Printf("New Target Power: %d watts\n", targetPower)
			}
		})
	}

	if hrChar != nil {
		hrChar.EnableNotifications(func(buf []byte) {
			if len(buf) < 2 {
				return
			}
			flags := buf[0]
			hr := buf[1]
			if flags&0x01 == 0x01 && len(buf) >= 3 {
				hr = buf[1] // Ignore extended HR values for now
			}
			state.HR = hr
			handler(state)
		})
	}

	if powerChar != nil {
		powerChar.EnableNotifications(func(buf []byte) {
			data, err := cps.ParseCyclingPowerMeasurement(buf)
			utils.Must("parse cycling power measurement", err)

			log.Debug(data)

			handler(state)
		})
	}

	return nil
}

func isTrainerName(name string) bool {
	if name == "" {
		return false
	}
	name = strings.ToLower(name)
	match := []string{"zwift", "kickr", "trainer", "tacx", "wahoo", "elite", "suito", "neo", "bike"}
	for _, s := range match {
		if strings.Contains(name, s) {
			return true
		}
	}
	return false
}
