package ble

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/jmelahman/cycle-cli/ble/services/cps"
	"github.com/jmelahman/cycle-cli/ble/services/csc"
	"github.com/jmelahman/cycle-cli/ble/services/ftms"
	"github.com/jmelahman/cycle-cli/utils"
	log "github.com/sirupsen/logrus"
	"tinygo.org/x/bluetooth"
)

type Telemetry struct {
	HR              uint8
	resistanceValue int8
	Cadence         uint16
	Calories        uint16
	prevEventTime   uint16
	prevRevs        uint32
	Power           int16
	Speed           float64 // Speed in km/h or mph, depending on unitSystem
	Distance        float64 // Distance in km or miles, depending on unitSystem
}

type TelemetryHandler func(data Telemetry)

const (
	wheelCircumferenceMeters = 2.105 // in meters
	metersPerSecondToKmph    = 3.6
	metersPerSecondToMph     = 2.23694 // 1 m/s * 3600 s/hr / 1609.34 m/mi
	metersToKm               = 0.001
	metersToMiles            = 0.000621371 // 1.0 / 1609.34
)

var (
	CyclingPowerMeasurementUUID = MustParseUUID("2A63")
	HeartRateMeasurementUUID    = MustParseUUID("2A37")
	CSCMeasurementUUID          = MustParseUUID("2A5B")
	FitnessMachineUUID          = MustParseUUID("2ADA")
	IndoorBikeDataUUID          = MustParseUUID("2AD2")
	FitnessControlPointUUID     = MustParseUUID("2AD9")
)

var adapter = bluetooth.DefaultAdapter

// SetResistance sets the trainer's resistance level (0-100%)
func SetResistance(state Telemetry, level int8) error {
	state.resistanceValue = level * 2
	log.Infof("Resistance set to %d%%", level)
	return nil
}

// ConnectToTrainer scans for a trainer device and connects to it.
// It returns a connected bluetooth.Device object.
func ConnectToTrainer() (*bluetooth.Device, error) {
	utils.Must("enable BLE stack", adapter.Enable())

	var trainerAddr bluetooth.Address

	utils.Must("scan", adapter.Scan(func(adapter *bluetooth.Adapter, result bluetooth.ScanResult) {
		name := result.LocalName()
		if isTrainerName(name) {
			log.Infof("✅ Found trainer: %s [%s]\n", name, result.Address.String())
			trainerAddr = result.Address
			utils.Must("stop bluetooth adapter scan", adapter.StopScan())
		}
	}))

	device, err := adapter.Connect(trainerAddr, bluetooth.ConnectionParams{})
	utils.Must("connect to device", err)

	return &device, nil
}

func SubscribeToMetrics(dev *bluetooth.Device, state Telemetry, unitSystem string, handler TelemetryHandler) error {
	services, err := dev.DiscoverServices(nil)
	utils.Must("discover services", err)

	var powerChar, hrChar, bikeChar, cscChar, controlPointChar, fitnessChar *bluetooth.DeviceCharacteristic

	for _, srv := range services {
		chars, err := srv.DiscoverCharacteristics(nil)
		if err != nil {
			return fmt.Errorf("discover characteristics: %w", err)
		}
		for _, char := range chars {
			switch char.UUID() {
			case FitnessControlPointUUID:
				log.Debug("Registered: Fitness Control Point")
				controlPointChar = &char
			case IndoorBikeDataUUID:
				log.Debug("Registered: Indoor Bike Data")
				bikeChar = &char
			case CSCMeasurementUUID:
				log.Debug("Registered: Cycling Speed and Cadence (CSC) Measurement")
				cscChar = &char
			case FitnessMachineUUID:
				log.Debug("Registered: Fitness Machine Status")
				fitnessChar = &char
			case HeartRateMeasurementUUID:
				log.Debug("Registered: Heart Rate Measurement")
				hrChar = &char
			case CyclingPowerMeasurementUUID:
				log.Debug("Registered: Cycling Power Measurement")
				powerChar = &char
			}
		}
	}

	if bikeChar != nil {
		err = bikeChar.EnableNotifications(func(buf []byte) {
			data, err := ftms.ParseIndoorBikeData(buf)
			utils.Must("parse indoor bike data", err)

			log.Debugf("Indoor Bike Data: %v", data)

			// Magic 30. Possibly from 1/2 RPM * 60 Seconds?
			state.Cadence = *data.InstantaneousCadence / 30

			handler(state)
		})
		utils.Must("enable bike data notifications", err)
	}

	if controlPointChar != nil {
		// FTMS Opcode for "Set Target Resistance Level" is 0x07
		// Resistance level is a uint8, where 0-100% is represented as 0-200 (0.5% resolution)
		//val := int16(state.resistanceValue * 10)
		//data := []byte{0x05, 0x01} // 0x01: request handle
		//data = append(data, byte(val), byte(val>>8))
		payload := []byte{0x07, byte(state.resistanceValue)}

		log.Debugf("Writing to control point: Opcode 0x07, Value %d", state.resistanceValue)
		_, err = controlPointChar.WriteWithoutResponse(payload)
		if err != nil {
			return fmt.Errorf("could not write resistance level: %w", err)
		}
	}

	if cscChar != nil {
		err = cscChar.EnableNotifications(func(buf []byte) {
			data, err := csc.ParseCSCMeasurement(buf)
			utils.Must("parse cycling speed and cadence", err)

			log.Debugf("CSC Data: %v", data)

			handler(state)
		})
		utils.Must("enable cycling speed and cadence notifications", err)
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
		utils.Must("enable fitness machine status notifications", err)
	}

	if hrChar != nil {
		err = hrChar.EnableNotifications(func(buf []byte) {
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
		utils.Must("enable heart rate notifications", err)
	}

	if powerChar != nil {
		err = powerChar.EnableNotifications(func(buf []byte) {
			data, err := cps.ParseCyclingPowerMeasurement(buf)
			utils.Must("parse cycling power measurement", err)

			log.Debugf("CPS Data: %v", data)

			if state.prevRevs > 0 && state.prevEventTime > 0 {
				var currRevs *uint32
				var currEventTime *uint16
				if data.WheelRevs != nil {
					currRevs = data.WheelRevs
				} else {
					currRevs = &state.prevRevs
				}
				if data.WheelEventTime != nil {
					currEventTime = data.WheelEventTime
				} else {
					currEventTime = &state.prevEventTime
				}
				log.Debugf("Revs: %v  Time: %v", currRevs, currEventTime)
				deltaRevs := float64(*currRevs - state.prevRevs)
				deltaTime := float64((*currEventTime-state.prevEventTime)&0xFFFF) / 1024.0 // in seconds

				if deltaTime > 0 {
					deltaDistanceMeters := deltaRevs * wheelCircumferenceMeters
					speedMetersPerSecond := deltaDistanceMeters / deltaTime

					if unitSystem == "imperial" {
						state.Speed = speedMetersPerSecond * metersPerSecondToMph
						state.Distance += deltaDistanceMeters * metersToMiles
					} else { // metric
						state.Speed = speedMetersPerSecond * metersPerSecondToKmph
						state.Distance += deltaDistanceMeters * metersToKm
					}
				} else {
					state.Speed = 0
					// Distance does not change if time or revs do not change
				}
			}

			if data.WheelRevs != nil {
				state.prevRevs = *data.WheelRevs
			}
			if data.WheelEventTime != nil {
				state.prevEventTime = *data.WheelEventTime
			}

			state.Power = data.InstantaneousPower

			handler(state)
		})
		utils.Must("enable cycling power measurement notifications", err)
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

func MustParseUUID(s string) bluetooth.UUID {
	u, err := bluetooth.ParseUUID(fmt.Sprintf("0000%s-0000-1000-8000-00805f9b34fb", s))
	if err != nil {
		panic("invalid UUID: " + s)
	}
	return u
}
