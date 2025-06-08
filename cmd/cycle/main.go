package main

import (
	"os"
	"os/signal"
	"syscall"
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
	RootCmd.Flags().BoolP("headless", "H", false, "Run in headless mode without UI")
}

func run(cmd *cobra.Command, args []string) {
	debugMode, err := cmd.Flags().GetBool("debug")
	utils.Must("parse debug flag", err)

	headlessMode, err := cmd.Flags().GetBool("headless")
	utils.Must("parse headless flag", err)

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

	// Create a channel to signal when to stop
	stop := make(chan struct{})
	
	var appUI *ui.UI
	if !headlessMode {
		// Create UI
		appUI = ui.NewUI(unitSystem)

		// Start UI in a separate goroutine
		go func() {
			if err := appUI.Start(); err != nil {
				log.Fatalf("❌ UI error: %v", err)
			}
			// Signal that UI has exited
			close(stop)
		}()
		
		appUI.UpdateStatus("🔍 Scanning for trainer...")
	} else {
		log.Info("Running in headless mode")
		log.Info("🔍 Scanning for trainer...")
	}
	device, err := ble.ConnectToTrainer()
	if err != nil {
		if !headlessMode {
			appUI.UpdateStatus("❌ Connection failed: %v", err)
			time.Sleep(2 * time.Second)
			appUI.Stop()
		}
		log.Fatalf("❌ Connection failed: %v", err)
	}
	defer device.Disconnect()

	if !headlessMode {
		appUI.UpdateStatus("✅ Connected to trainer")
		
		// Update resistance display
		if resistanceLevel != 0 {
			appUI.UpdateResistance(uint8(resistanceLevel))
		}
	} else {
		log.Info("✅ Connected to trainer")
	}

	state := ble.Telemetry{}

	err = ble.SubscribeToMetrics(device, state, func(data ble.Telemetry) {
		if !headlessMode {
			appUI.UpdateTelemetry(data)
		} else {
			log.Infof("Power: %dW, Cadence: %drpm, Speed: %.1f%s, Distance: %.1f%s", 
				data.Power, data.Cadence, data.Speed, 
				unitSystem == imperialSystemName ? "mph" : "km/h",
				data.Distance, 
				unitSystem == imperialSystemName ? "mi" : "km")
		}
	})
	if err != nil {
		if !headlessMode {
			appUI.UpdateStatus("❌ Failed to subscribe: %v", err)
			time.Sleep(2 * time.Second)
			appUI.Stop()
		}
		log.Fatalf("❌ Failed to subscribe: %v", err)
	}

	if resistanceLevel != 0 {
		err := ble.SetResistance(device, uint8(resistanceLevel))
		if err != nil {
			if !headlessMode {
				appUI.UpdateStatus("❌ Failed to set resistance: %v", err)
			} else {
				log.Errorf("❌ Failed to set resistance: %v", err)
			}
		} else {
			if !headlessMode {
				appUI.UpdateStatus("✅ Resistance set to %d%%", resistanceLevel)
			} else {
				log.Infof("✅ Resistance set to %d%%", resistanceLevel)
			}
		}
	}

	if !headlessMode {
		// Wait for UI to exit
		<-stop
	} else {
		// In headless mode, we need to keep the program running
		// Set up a signal handler to catch Ctrl+C
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		
		log.Info("Press Ctrl+C to exit")
		<-sigChan
		log.Info("Exiting...")
	}
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
