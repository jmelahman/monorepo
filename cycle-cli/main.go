package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jmelahman/cycle-cli/ble"
	"github.com/jmelahman/cycle-cli/ui"
	"github.com/jmelahman/cycle-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

// handleResistanceChange adjusts the resistance level and updates UI or logs accordingly.
func handleResistanceChange(state ble.Telemetry, currentLevel *int, change int, appUI *ui.UI, headless bool) {
	newLevel := *currentLevel + change
	if newLevel < 0 {
		newLevel = 0
	}
	if newLevel > 100 {
		newLevel = 100
	}

	if newLevel != *currentLevel {
		*currentLevel = newLevel
		levelToSet := int8(*currentLevel)

		go func() {
			statusMsg := fmt.Sprintf("Setting resistance to %d%%...", levelToSet)
			if headless {
				// Print on new lines to avoid clobbering telemetry output
				fmt.Printf("\n%s\n", statusMsg)
			} else if appUI != nil {
				appUI.UpdateStatus(statusMsg)
			}

			if err := ble.SetResistance(state, levelToSet); err != nil {
				errorMsg := fmt.Sprintf("❌ Failed to set resistance: %v", err)
				if headless {
					fmt.Printf("\n%s\n", errorMsg)
					log.Errorf("Failed to set resistance: %v", err)
				} else if appUI != nil {
					appUI.UpdateStatus(errorMsg)
					log.Errorf("Failed to set resistance: %v", err) // Also log for UI mode
				}
			} else {
				successMsg := fmt.Sprintf("✅ Resistance set to %d%%", levelToSet)
				if headless {
					fmt.Printf("\n%s\n", successMsg)
				} else if appUI != nil {
					appUI.UpdateResistance(levelToSet)
					appUI.UpdateStatus(successMsg)
				}
			}
		}()
	}
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
			appUI.UpdateStatus(fmt.Sprintf("❌ Connection failed: %v", err))
			time.Sleep(2 * time.Second)
			appUI.Stop()
		}
		log.Fatalf("❌ Connection failed: %v", err)
	}
	defer func() {
		if err := device.Disconnect(); err != nil {
			log.Errorf("Error disconnecting device: %v", err)
		}
	}()

	var inputChan chan rune
	state := ble.Telemetry{}
	currentResistanceLevel := resistanceLevel // Initialize with the flag value

	if !headlessMode {
		appUI.UpdateStatus("✅ Connected to trainer")

		// Update resistance display initially if a value was provided
		if currentResistanceLevel != 0 {
			appUI.UpdateResistance(int8(currentResistanceLevel))
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
					handleResistanceChange(state, &currentResistanceLevel, -5, appUI, false)
				case ')': // Increase resistance
					handleResistanceChange(state, &currentResistanceLevel, +5, appUI, false)
				}
			}
			return event
		})

	} else { // Headless mode
		var oldState *term.State
		var errTerm error

		// Attempt to set raw terminal mode for keyboard shortcuts
		oldState, errTerm = term.MakeRaw(int(os.Stdin.Fd()))
		if errTerm != nil {
			log.Warnf("⚠️  Failed to set raw terminal mode, keyboard shortcuts ('(',')','q') may not work. Use Ctrl+C to exit. Error: %v", errTerm)
		} else {
			// Restore terminal state when done with headless mode or if function exits.
			// This defer needs to be conditional on oldState being non-nil if we want to avoid panic on Restore.
			// However, term.Restore handles nil oldState gracefully (it's a no-op).
			// For clarity, one might wrap it: if oldState != nil { defer term.Restore(...) }
			// But current `defer term.Restore` is fine.
			defer func() {
				if err := term.Restore(int(os.Stdin.Fd()), oldState); err != nil {
					log.Error("Failed to restore terminal to previous state.")
				}
			}()

			inputChan = make(chan rune)
			go func() {
				defer close(inputChan) // Ensure channel is closed when goroutine exits
				reader := bufio.NewReader(os.Stdin)
				for {
					char, _, readErr := reader.ReadRune()
					if readErr != nil {
						log.Debugf("Error reading rune: %v", readErr)
						return // Exit goroutine, will close channel
					}
					inputChan <- char
				}
			}()
		}
	}

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

			fmt.Printf("Power: %4dW, Cadence: %3drpm, Speed: %5.1f%s, Distance: %5.1f%s, Duration: %s\r",
				data.Power, data.Cadence, data.Speed, speedUnit,
				data.Distance, distanceUnit, durationStr)
		}
	})
	if err != nil {
		if !headlessMode {
			appUI.UpdateStatus(fmt.Sprintf("❌ Failed to subscribe: %v", err))
			time.Sleep(2 * time.Second)
			appUI.Stop()
		}
		log.Fatalf("❌ Failed to subscribe: %v", err)
	}

	if resistanceLevel != 0 {
		err := ble.SetResistance(state, int8(resistanceLevel))
		if err != nil {
			if !headlessMode {
				appUI.UpdateStatus(fmt.Sprintf("❌ Failed to set resistance: %v", err))
			} else {
				log.Errorf("❌ Failed to set resistance: %v", err)
			}
		} else {
			if !headlessMode {
				appUI.UpdateStatus(fmt.Sprintf("✅ Resistance set to %d%%", resistanceLevel))
			} else {
				log.Infof("✅ Resistance set to %d%%", resistanceLevel)
			}
		}
	}

	if !headlessMode {
		// Wait for UI to exit
		<-stop
	} else {
		// Headless mode main loop
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	headlessLoop:
		for {
			select {
			case <-sigChan:
				fmt.Println("\n👋 Exiting via signal...")
				break headlessLoop
			case char, ok := <-inputChan: // This case is only effective if inputChan is not nil
				if !ok { // Channel closed, reader goroutine exited
					log.Debug("Input channel closed. Relying on Ctrl+C to exit.")
					inputChan = nil // Prevent further selection on this case
					continue
				}

				shouldExit := false
				switch char {
				case '(':
					handleResistanceChange(state, &currentResistanceLevel, -5, nil, true)
				case ')':
					handleResistanceChange(state, &currentResistanceLevel, +5, nil, true)
				case 'q', 3: // 'q' or Ctrl+C (ETX)
					shouldExit = true
				}
				if shouldExit {
					break headlessLoop
				}
			}
		}
		// Terminal state is restored by defer if it was set to raw.
		fmt.Println("\n👋 Exiting...\r")
	}
}

func main() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
