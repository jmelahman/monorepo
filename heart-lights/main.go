package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/heart-lights/config.yaml)")

	rootCmd.Flags().Int("max-hr", 0, "Maximum heart rate")
	rootCmd.Flags().String("hr-monitor", "", "Heart rate monitor bluetooth address")
	rootCmd.Flags().String("smart-light", "", "Smart light bluetooth address")

	cobra.CheckErr(viper.BindPFlag("max_hr", rootCmd.Flags().Lookup("max-hr")))
	cobra.CheckErr(viper.BindPFlag("hr_monitor", rootCmd.Flags().Lookup("hr-monitor")))
	cobra.CheckErr(viper.BindPFlag("smart_light", rootCmd.Flags().Lookup("smart-light")))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		configPath := filepath.Join(home, ".config", "heart-lights")
		viper.AddConfigPath(configPath)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("HEART_LIGHTS")
	_ = viper.ReadInConfig()
}

// getRGBForHeartRate returns RGB values based on heart rate percentage of max HR.
// Zones follow standard heart rate training zones.
func getRGBForHeartRate(hrPercentage float64) (int32, int32, int32) {
	switch {
	case hrPercentage < 60:
		return 255, 160, 120 // White (Zone 1: Recovery)
	case hrPercentage <= 70:
		return 0, 0, 255 // Blue (Zone 2: Easy)
	case hrPercentage <= 80:
		return 0, 255, 0 // Green (Zone 3: Aerobic)
	case hrPercentage <= 90:
		return 255, 55, 0 // Yellow (Zone 4: Threshold)
	case hrPercentage <= 95:
		return 255, 20, 0 // Orange (Zone 5: VO2 Max)
	default:
		return 255, 0, 0 // Red (Zone 6: Anaerobic)
	}
}

// parseHeartRate parses a Heart Rate Measurement characteristic value (0x2A37).
func parseHeartRate(buf []byte) (uint16, error) {
	if len(buf) < 2 {
		return 0, fmt.Errorf("heart rate data too short: %d bytes", len(buf))
	}

	flags := buf[0]
	// Bit 0: Heart Rate Value Format (0 = UINT8, 1 = UINT16)
	if flags&0x01 == 0 {
		return uint16(buf[1]), nil
	}

	if len(buf) < 3 {
		return 0, fmt.Errorf("heart rate data too short for uint16 format: %d bytes", len(buf))
	}
	return uint16(buf[1]) | uint16(buf[2])<<8, nil
}

var rootCmd = &cobra.Command{
	Use:   "heart-lights",
	Short: "Control smart lights based on heart rate data",
	Long:  "A tool to control smart lights based on heart rate monitor data via Bluetooth",
	RunE: func(cmd *cobra.Command, args []string) error {
		maxHR := viper.GetInt("max_hr")
		hrMonitorAddress := viper.GetString("hr_monitor")
		smartLightAddress := viper.GetString("smart_light")

		if maxHR == 0 {
			return fmt.Errorf("required flag \"max-hr\" not set")
		}
		if hrMonitorAddress == "" {
			return fmt.Errorf("required flag \"hr-monitor\" not set")
		}
		if smartLightAddress == "" {
			return fmt.Errorf("required flag \"smart-light\" not set")
		}

		adapter := bluetooth.DefaultAdapter
		must("enable adapter", adapter.Enable())

		var lightDevice *smartlightsble.Device
		var hrDevice *bluetooth.Device

		// Scan for both devices
		err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
			if lightDevice == nil && result.Address.String() == smartLightAddress {
				device, err := smartlightsble.ConnectAndDiscover(adapter, result.Address)
				must("connect to smart lights", err)
				lightDevice = device
				fmt.Println("Connected to smart lights:", result.LocalName())
			}
			if hrDevice == nil && result.Address.String() == hrMonitorAddress {
				device, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
				must("connect to heart rate monitor", err)
				hrDevice = &device
				fmt.Println("Connected to heart rate monitor:", result.LocalName())
			}
			if lightDevice != nil && hrDevice != nil {
				_ = adapter.StopScan()
			}
		})
		must("scan for devices", err)

		// Discover heart rate measurement characteristic
		services, err := hrDevice.DiscoverServices(nil)
		must("discover services", err)

		heartRateMeasurementUUID := bluetooth.NewUUID([16]byte{
			0x00, 0x00, 0x2A, 0x37,
			0x00, 0x00, 0x10, 0x00,
			0x80, 0x00, 0x00, 0x80,
			0x5F, 0x9B, 0x34, 0xFB,
		})

		var hrChar *bluetooth.DeviceCharacteristic
		for _, srv := range services {
			chars, err := srv.DiscoverCharacteristics(nil)
			must("discover characteristics", err)
			for _, char := range chars {
				if char.UUID() == heartRateMeasurementUUID {
					hrChar = &char
					break
				}
			}
			if hrChar != nil {
				break
			}
		}

		if hrChar == nil {
			log.Fatal("Could not find heart rate measurement characteristic")
		}

		type hrData struct {
			mu        sync.Mutex
			values    []uint16
			lastPrint time.Time
		}
		hr := &hrData{
			values:    make([]uint16, 0),
			lastPrint: time.Now(),
		}

		err = hrChar.EnableNotifications(func(buf []byte) {
			heartRate, err := parseHeartRate(buf)
			if err != nil {
				log.Printf("Error parsing heart rate data: %v", err)
				return
			}

			hr.mu.Lock()
			hr.values = append(hr.values, heartRate)

			if time.Since(hr.lastPrint) >= time.Second {
				if len(hr.values) > 0 {
					var sum uint64
					for _, v := range hr.values {
						sum += uint64(v)
					}
					avg := uint16(sum / uint64(len(hr.values)))
					fmt.Printf("\rLatest HR: %4d bpm", avg)

					hrPercentage := float64(avg) / float64(maxHR) * 100.0

					r, g, b := getRGBForHeartRate(hrPercentage)

					if lightDevice != nil {
						err := lightDevice.SetRGB(r, g, b)
						if err != nil {
							log.Printf("Error setting light color: %v", err)
						}
					}

					hr.values = make([]uint16, 0)
				}
				hr.lastPrint = time.Now()
			}
			hr.mu.Unlock()
		})
		must("enable heart rate notifications", err)

		fmt.Println("Monitoring heart rate and controlling lights. Press Ctrl+C to exit.")
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
