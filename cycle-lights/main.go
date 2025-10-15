package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jmelahman/cycle-cli/ble"
	"github.com/jmelahman/cycle-cli/ble/services/cps"
	smartlightsble "github.com/jmelahman/smart-lights/ble"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"tinygo.org/x/bluetooth"
)

var (
	cfgFile string
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/cycle-lights/config.yaml)")

	rootCmd.Flags().Int("ftp", 0, "FTP value")
	rootCmd.Flags().String("power-meter", "", "Power meter bluetooth address")
	rootCmd.Flags().String("smart-light", "", "Smart light bluetooth address")

	// Bind flags to viper
	cobra.CheckErr(viper.BindPFlag("ftp", rootCmd.Flags().Lookup("ftp")))
	cobra.CheckErr(viper.BindPFlag("power_meter", rootCmd.Flags().Lookup("power-meter")))
	cobra.CheckErr(viper.BindPFlag("smart_light", rootCmd.Flags().Lookup("smart-light")))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Use ~/.config/cycle-lights/config.yaml as default
		configPath := filepath.Join(home, ".config", "cycle-lights")
		viper.AddConfigPath(configPath)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

// getRGBForPower returns RGB values based on power percentage of FTP
func getRGBForPower(powerPercentage float64) (int32, int32, int32) {
	switch {
	case powerPercentage < 60:
		return 255, 160, 120 // White (Zone 1: Recovery)
	case powerPercentage <= 75:
		return 0, 0, 255 // Blue (Zone 2: Endurance)
	case powerPercentage <= 89:
		return 0, 255, 0 // Green (Zone 3: Tempo)
	case powerPercentage <= 104:
		return 255, 55, 0 // Yellow (Zone 4: Threshold)
	case powerPercentage <= 118:
		return 255, 20, 0 // Orange (Zone 5: VO2 Max)
	default:
		return 255, 0, 0 // Red (Zone 6: Anaerobic)
	}
}

var rootCmd = &cobra.Command{
	Use:   "cycle-lights",
	Short: "Control smart lights based on cycling power data",
	Long:  "A tool to control smart lights based on power meter data from cycling",
	RunE: func(cmd *cobra.Command, args []string) error {
		ftp := viper.GetInt("ftp")
		powerMeterAddress := viper.GetString("power_meter")
		smartLightAddress := viper.GetString("smart_light")

		if ftp == 0 {
			return fmt.Errorf("required flag \"ftp\" not set")
		}
		if powerMeterAddress == "" {
			return fmt.Errorf("required flag \"power-meter\" not set")
		}
		if smartLightAddress == "" {
			return fmt.Errorf("required flag \"smart-light\" not set")
		}

		adapter := bluetooth.DefaultAdapter
		must("enable adapter", adapter.Enable())

		var lightDevice *smartlightsble.Device
		var powerDevice *bluetooth.Device

		err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
			if lightDevice == nil && result.Address.String() == smartLightAddress {
				device, err := smartlightsble.ConnectAndDiscover(adapter, result.Address)
				must("connect to smart lights", err)
				lightDevice = device
				fmt.Println("Connected to smart lights")
			} else if powerDevice == nil && result.Address.String() == powerMeterAddress {
				device, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
				must("connect to power meter", err)
				powerDevice = &device
				fmt.Println("Connected to power meter")
			}
			if lightDevice != nil && powerDevice != nil {
				_ = adapter.StopScan()
			}
		})
		must("scan for device", err)

		// Subscribe to power meter notifications
		services, err := powerDevice.DiscoverServices(nil)
		must("discover services", err)

		var powerChar *bluetooth.DeviceCharacteristic
		cyclingPowerMeasurementUUID := ble.MustParseUUID("2A63")

		for _, srv := range services {
			chars, err := srv.DiscoverCharacteristics(nil)
			must("discover characteristics", err)
			for _, char := range chars {
				if char.UUID() == cyclingPowerMeasurementUUID {
					powerChar = &char
					break
				}
			}
			if powerChar != nil {
				break
			}
		}

		if powerChar == nil {
			log.Fatal("Could not find cycling power measurement characteristic")
		}

		// Data structure for tracking power values
		type powerData struct {
			mu        sync.Mutex
			values    []int16
			lastPrint time.Time
		}
		power := &powerData{
			values:    make([]int16, 0),
			lastPrint: time.Now(),
		}

		// Enable notifications for power data
		err = powerChar.EnableNotifications(func(buf []byte) {
			data, err := cps.ParseCyclingPowerMeasurement(buf)
			if err != nil {
				log.Printf("Error parsing power data: %v", err)
				return
			}

			power.mu.Lock()
			power.values = append(power.values, data.InstantaneousPower)

			// Every 1 seconds, calculate average and update lights
			if time.Since(power.lastPrint) >= 1*time.Second {
				if len(power.values) > 0 {
					var sum int64
					for _, v := range power.values {
						sum += int64(v)
					}
					avg := int16(sum / int64(len(power.values)))
					fmt.Printf("\rLatest Power: %4d", avg)

					// Calculate power percentage of FTP
					powerPercentage := float64(avg) / float64(ftp) * 100.0

					// Get RGB values based on power zone
					r, g, b := getRGBForPower(powerPercentage)

					// Set the light color
					if lightDevice != nil {
						err := lightDevice.SetRGB(r, g, b)
						if err != nil {
							log.Printf("Error setting light color: %v", err)
						}
					}

					// Reset for next interval
					power.values = make([]int16, 0)
				}
				power.lastPrint = time.Now()
			}
			power.mu.Unlock()
		})
		must("enable power notifications", err)

		// Keep the program running
		fmt.Println("Monitoring power data and controlling lights. Press Ctrl+C to exit.")
		select {}
	},
}

func main() {
	must("execute", rootCmd.Execute())
}

func must(msg string, err error) {
	if err != nil {
		log.Fatalf("failed to "+msg+": %v", err)
	}
}
