package main

import (
	"time"

	"github.com/jmelahman/cycle-cli/ble"
	"github.com/jmelahman/cycle-cli/ui"
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
		log.Debug("Debug mode enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// Create UI
	appUI := ui.NewUI(unitSystem)

	// Create a channel to signal when UI exits
	stop := make(chan struct{})

	// Start UI in a separate goroutine
	go func() {
		if err := appUI.Start(); err != nil {
			log.Fatalf("❌ UI error: %v", err)
		}
		// Signal that UI has exited
		close(stop)
	}()

	appUI.UpdateStatus("🔍 Scanning for trainer...")
	device, err := ble.ConnectToTrainer()
	if err != nil {
		appUI.UpdateStatus("❌ Connection failed: %v", err)
		time.Sleep(2 * time.Second)
		appUI.Stop()
		log.Fatalf("❌ Connection failed: %v", err)
	}
	defer device.Disconnect()

	appUI.UpdateStatus("✅ Connected to trainer")

	// Update resistance display
	if resistanceLevel != 0 {
		appUI.UpdateResistance(uint8(resistanceLevel))
	}

	state := ble.Telemetry{}

	err = ble.SubscribeToMetrics(device, state, func(data ble.Telemetry) {
		appUI.UpdateTelemetry(data)
	})
	if err != nil {
		appUI.UpdateStatus("❌ Failed to subscribe: %v", err)
		time.Sleep(2 * time.Second)
		appUI.Stop()
		log.Fatalf("❌ Failed to subscribe: %v", err)
	}

	if resistanceLevel != 0 {
		err := ble.SetResistance(device, uint8(resistanceLevel))
		if err != nil {
			appUI.UpdateStatus("❌ Failed to set resistance: %v", err)
		} else {
			appUI.UpdateStatus("✅ Resistance set to %d%%", resistanceLevel)
		}
	}

	// Wait for UI to exit
	<-stop
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
