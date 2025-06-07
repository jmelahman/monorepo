package main

import (
	"fmt"
	"time"

	"github.com/jmelahman/cycle-cli/ble"
	"github.com/jmelahman/cycle-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	metricSystemName   = "metric"
	imperialSystemName = "imperial"
)

var RootCmd = &cobra.Command{
	Use:   "cycle",
	Short: "Cycle trainer control",
	Long:  `Cycle is a tool to control and monitor cycling trainers.`,
	Run:   run,
}

func init() {
	RootCmd.Flags().IntP("resistance", "r", 0, "Resistance level (0-100)")
	RootCmd.Flags().BoolP("debug", "d", false, "Enable debug mode (default: true)")
	RootCmd.Flags().StringP("unit", "u", "imperial", "Unit system (metric or imperial)")
}

func run(cmd *cobra.Command, args []string) {
	debugMode, err := cmd.Flags().GetBool("debug")
	utils.Must("parse debug flag", err)

	resistanceLevel, err := cmd.Flags().GetInt("resistance")
	utils.Must("parse resistance", err)

	unitSystem, err := cmd.Flags().GetString("unit")
	if err != nil {
		log.Fatalf("❌ Invalid unit system: %v", err)
	}

	var validUnitSystems = map[string]bool{
		metricSystemName:   true,
		imperialSystemName: true,
	}

	if _, exists := validUnitSystems[unitSystem]; !exists {
		log.Fatalf("❌ Unsupported unit system: %s. Use %s or %s.", unitSystem, metricSystemName, imperialSystemName)
	}

	if debugMode {
		log.SetLevel(log.DebugLevel)
		fmt.Println("🐞 Debug mode enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	fmt.Printf("Using %s units\n", unitSystem)

	fmt.Println("🔍 Scanning for trainer...")
	device, err := ble.ConnectToTrainer()
	if err != nil {
		log.Fatalf("❌ Connection failed: %v", err)
	}
	defer device.Disconnect()

	state := ble.Telemetry{}
	startTime := time.Now()

	err = ble.SubscribeToMetrics(device, state, func(data ble.Telemetry) {
		elapsed := time.Since(startTime)
		totalSeconds := int(elapsed.Seconds())

		if totalSeconds >= 3600 {
			hours := totalSeconds / 3600
			minutes := (totalSeconds % 3600) / 60
			seconds := totalSeconds % 60
			fmt.Printf("Power: %4dW  Cadence: %3drpm  Duration: %02d:%02d:%02d\r", data.Power, data.Cadence, hours, minutes, seconds)
		} else {
			minutes := totalSeconds / 60
			seconds := totalSeconds % 60
			fmt.Printf("Power: %4dW  Cadence: %3drpm  Duration: %02d:%02d\r\nSpeed: %.2f  Distance: %.2f\r", data.Power, data.Cadence, minutes, seconds, data.Speed, data.Distance)
		}
	})
	if err != nil {
		log.Fatalf("❌ Failed to subscribe: %v", err)
	}

	if resistanceLevel != 0 {
		err := ble.SetResistance(device, uint8(resistanceLevel))
		utils.Must("set trainer resistance", err)
	}

	for {
		time.Sleep(5 * time.Second)
	}
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
