package main

import (
	"fmt"
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
	RootCmd.Flags().IntP("ftp", "f", 230, "Functional Threshold Power (FTP) in watts")
}

func run(cmd *cobra.Command, args []string) {
	debugMode, err := cmd.Flags().GetBool("debug")
	utils.Must("parse debug flag", err)

	headlessMode, err := cmd.Flags().GetBool("headless")
	utils.Must("parse headless flag", err)

	resistanceLevel, err := cmd.Flags().GetInt("resistance")
	utils.Must("parse resistance", err)

	ftp, err := cmd.Flags().GetInt("ftp")
	utils.Must("parse ftp flag", err)

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
		log.Debug("🐞 Debug mode enabled")
	} else {
		log.SetLevel(log.InfoLevel)
	}

	// Create a channel to signal when to stop
	stop := make(chan struct{})
	startTime := time.Now() // Initialize startTime here

	var appUI *ui.UI
	if !headlessMode {
		// Create UI
		appUI = ui.NewUI(unitSystem, ftp)

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
		log.Info("🎃 Running in headless mode")
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

	currentResistanceLevel := resistanceLevel // Initialize with the flag value

	if !headlessMode {
		appUI.UpdateStatus("✅ Connected to trainer")

		// Update resistance display initially if a value was provided
		if currentResistanceLevel != 0 {
			appUI.UpdateResistance(uint8(currentResistanceLevel))
		}

		// Set up key handling for UI
		appUI.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape || (event.Key() == tcell.KeyRune && event.Rune() == 'q') {
				appUI.Stop()
				return nil
			}

			if event.Key() == tcell.KeyRune {
				switch event.Rune() {
				case '(': // Decrease resistance
					newLevel := currentResistanceLevel - 5
					if newLevel < 0 {
						newLevel = 0
					}
					if newLevel != currentResistanceLevel {
						currentResistanceLevel = newLevel
						go func(levelToSet uint8) {
							appUI.UpdateStatus(fmt.Sprintf("Setting resistance to %d%%...", levelToSet))
							if err := ble.SetResistance(device, levelToSet); err != nil {
								appUI.UpdateStatus(fmt.Sprintf("❌ Failed to set resistance: %v", err))
								log.Errorf("Failed to set resistance: %v", err)
							} else {
								appUI.UpdateResistance(levelToSet)
								appUI.UpdateStatus(fmt.Sprintf("✅ Resistance set to %d%%", levelToSet))
							}
						}(uint8(currentResistanceLevel))
					}
				case ')': // Increase resistance
					newLevel := currentResistanceLevel + 5
					if newLevel > 100 {
						newLevel = 100
					}
					if newLevel != currentResistanceLevel {
						currentResistanceLevel = newLevel
						go func(levelToSet uint8) {
							appUI.UpdateStatus(fmt.Sprintf("Setting resistance to %d%%...", levelToSet))
							if err := ble.SetResistance(device, levelToSet); err != nil {
								appUI.UpdateStatus(fmt.Sprintf("❌ Failed to set resistance: %v", err))
								log.Errorf("Failed to set resistance: %v", err)
							} else {
								appUI.UpdateResistance(levelToSet)
								appUI.UpdateStatus(fmt.Sprintf("✅ Resistance set to %d%%", levelToSet))
							}
						}(uint8(currentResistanceLevel))
					}
				}
			}
			return event
		})

	} else {
		log.Info("✅ Connected to trainer")
	}

	state := ble.Telemetry{}

	err = ble.SubscribeToMetrics(device, state, unitSystem, func(data ble.Telemetry) {
		if !headlessMode {
			appUI.UpdateTelemetry(data)
		} else {
			speedUnit := "km/h"
			distanceUnit := "km"
			if unitSystem == imperialSystemName {
				speedUnit = "mph"
				distanceUnit = "mi"
			}

			// Calculate duration
			elapsed := time.Since(startTime)
			totalSeconds := int(elapsed.Seconds())
			var durationStr string
			if totalSeconds >= 3600 {
				hours := totalSeconds / 3600
				minutes := (totalSeconds % 3600) / 60
				seconds := totalSeconds % 60
				durationStr = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
			} else {
				minutes := totalSeconds / 60
				seconds := totalSeconds % 60
				durationStr = fmt.Sprintf("%02d:%02d", minutes, seconds)
			}

			fmt.Printf("\rPower: %4dW, Cadence: %3drpm, Speed: %5.1f%s, Distance: %5.1f%s, Duration: %s",
				data.Power, data.Cadence, data.Speed, speedUnit,
				data.Distance, distanceUnit, durationStr)
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

		<-sigChan
		fmt.Println("\n👋 Exiting...")
	}
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
