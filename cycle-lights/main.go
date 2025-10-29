package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmelahman/cycle-cli/ble"
	"github.com/jmelahman/cycle-cli/ble/services/cps"
	smartlightsble "github.com/jmelahman/smart-lights/ble"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
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
	
	// Zwift API flags
	rootCmd.Flags().String("zwift-email", "", "Zwift account email")
	rootCmd.Flags().String("zwift-password", "", "Zwift account password")
	rootCmd.Flags().Int("zwift-user-id", 0, "Zwift user ID to follow (0 for self)")
	rootCmd.Flags().Bool("zwift", false, "Use Zwift API instead of power meter")

	// Bind flags to viper
	cobra.CheckErr(viper.BindPFlag("ftp", rootCmd.Flags().Lookup("ftp")))
	cobra.CheckErr(viper.BindPFlag("power_meter", rootCmd.Flags().Lookup("power-meter")))
	cobra.CheckErr(viper.BindPFlag("smart_light", rootCmd.Flags().Lookup("smart-light")))
	cobra.CheckErr(viper.BindPFlag("zwift_email", rootCmd.Flags().Lookup("zwift-email")))
	cobra.CheckErr(viper.BindPFlag("zwift_password", rootCmd.Flags().Lookup("zwift-password")))
	cobra.CheckErr(viper.BindPFlag("zwift_user_id", rootCmd.Flags().Lookup("zwift-user-id")))
	cobra.CheckErr(viper.BindPFlag("zwift", rootCmd.Flags().Lookup("zwift")))
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

// Zwift API structures
type ZwiftAuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type ZwiftRiderState struct {
	ID              int64   `json:"id"`
	WorldID         int     `json:"worldId"`
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Z               float64 `json:"z"`
	Heading         float64 `json:"heading"`
	Speed           float64 `json:"speed"`
	Power           int     `json:"power"`
	Heartrate       int     `json:"heartrate"`
	Cadence         int     `json:"cadence"`
	PlayerTypeID    int     `json:"playerTypeId"`
	Ftp             int     `json:"ftp"`
	InsidePowerZone bool    `json:"insidePowerZone"`
}

type ZwiftWorld struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ZwiftProfile struct {
	ID       int64  `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// ZwiftClient handles communication with Zwift API
type ZwiftClient struct {
	client   *http.Client
	baseURL  string
	token    *oauth2.Token
	userID   int64
	worldID  int
}

// NewZwiftClient creates a new Zwift API client
func NewZwiftClient(email, password string, userID int) (*ZwiftClient, error) {
	// Authenticate with Zwift
	authURL := "https://secure.zwift.com/auth/realms/zwift/protocol/openid-connect/token"
	data := url.Values{}
	data.Set("username", email)
	data.Set("password", password)
	data.Set("grant_type", "password")
	data.Set("client_id", "Zwift_Mobile_Link")

	req, err := http.NewRequest("POST", authURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth failed: %s", string(body))
	}

	var authResp ZwiftAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, err
	}

	token := &oauth2.Token{
		AccessToken:  authResp.AccessToken,
		RefreshToken: authResp.RefreshToken,
		TokenType:    authResp.TokenType,
		Expiry:       time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second),
	}

	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.StaticTokenSource(token),
			Base:   http.DefaultTransport,
		},
	}

	zwiftClient := &ZwiftClient{
		client:  httpClient,
		baseURL: "https://us-or-rly101.zwift.com/api",
		token:   token,
	}

	// Get user ID if not provided
	if userID == 0 {
		profile, err := zwiftClient.GetProfile()
		if err != nil {
			return nil, err
		}
		zwiftClient.userID = profile.ID
	} else {
		zwiftClient.userID = int64(userID)
	}

	return zwiftClient, nil
}

// GetProfile gets the user's profile
func (z *ZwiftClient) GetProfile() (*ZwiftProfile, error) {
	url := fmt.Sprintf("%s/users/me", z.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get profile failed: %d", resp.StatusCode)
	}

	var profile ZwiftProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// GetRiderState gets the current state of a rider
func (z *ZwiftClient) GetRiderState(worldID, userID int64) (*ZwiftRiderState, error) {
	url := fmt.Sprintf("%s/users/%d/profiles/%d/events", z.baseURL, userID, userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := z.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get rider state failed: %d", resp.StatusCode)
	}

	// For simplicity, we'll just get the latest event
	// In a real implementation, you'd want to track the current event
	var events []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return nil, err
	}

	// If we have events, get the world ID from the first one
	if len(events) > 0 {
		// This is a simplified approach - in reality you'd need to parse the event structure
		// For now we'll use a default world ID
		if z.worldID == 0 {
			z.worldID = 1 // Default world
		}
	}

	// Get rider state from world
	worldURL := fmt.Sprintf("%s/worlds/%d/riders/%d", z.baseURL, z.worldID, userID)
	worldReq, err := http.NewRequest("GET", worldURL, nil)
	if err != nil {
		return nil, err
	}
	worldReq.Header.Set("Accept", "application/json")

	worldResp, err := z.client.Do(worldReq)
	if err != nil {
		return nil, err
	}
	defer worldResp.Body.Close()

	if worldResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get world rider state failed: %d", worldResp.StatusCode)
	}

	var riderState ZwiftRiderState
	if err := json.NewDecoder(worldResp.Body).Decode(&riderState); err != nil {
		return nil, err
	}

	return &riderState, nil
}

// StartPowerMonitoring starts monitoring power data from Zwift
func (z *ZwiftClient) StartPowerMonitoring(lightDevice *smartlightsble.Device, ftp int, ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

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

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			state, err := z.GetRiderState(int64(z.worldID), z.userID)
			if err != nil {
				log.Printf("Error getting rider state: %v", err)
				continue
			}

			power.mu.Lock()
			power.values = append(power.values, int16(state.Power))

			// Every second, calculate average and update lights
			if time.Since(power.lastPrint) >= time.Second {
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
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "cycle-lights",
	Short: "Control smart lights based on cycling power data",
	Long:  "A tool to control smart lights based on power meter data from cycling or Zwift API",
	RunE: func(cmd *cobra.Command, args []string) error {
		ftp := viper.GetInt("ftp")
		powerMeterAddress := viper.GetString("power_meter")
		smartLightAddress := viper.GetString("smart_light")
		useZwift := viper.GetBool("zwift")
		zwiftEmail := viper.GetString("zwift_email")
		zwiftPassword := viper.GetString("zwift_password")
		zwiftUserID := viper.GetInt("zwift_user_id")

		if ftp == 0 {
			return fmt.Errorf("required flag \"ftp\" not set")
		}
		if smartLightAddress == "" {
			return fmt.Errorf("required flag \"smart-light\" not set")
		}

		adapter := bluetooth.DefaultAdapter
		must("enable adapter", adapter.Enable())

		var lightDevice *smartlightsble.Device

		// Connect to smart light
		err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
			if lightDevice == nil && result.Address.String() == smartLightAddress {
				device, err := smartlightsble.ConnectAndDiscover(adapter, result.Address)
				must("connect to smart lights", err)
				lightDevice = device
				fmt.Println("Connected to smart lights:", result.LocalName())
				_ = adapter.StopScan()
			}
		})
		must("scan for smart light", err)

		if useZwift {
			// Use Zwift API
			if zwiftEmail == "" || zwiftPassword == "" {
				return fmt.Errorf("zwift-email and zwift-password are required when using Zwift API")
			}

			zwiftClient, err := NewZwiftClient(zwiftEmail, zwiftPassword, zwiftUserID)
			if err != nil {
				return fmt.Errorf("failed to create Zwift client: %v", err)
			}

			fmt.Println("Monitoring power data from Zwift and controlling lights. Press Ctrl+C to exit.")
			
			// Create context for cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			
			// Start monitoring in a goroutine
			go zwiftClient.StartPowerMonitoring(lightDevice, ftp, ctx)
			
			// Keep the program running
			select {}
		} else {
			// Use Bluetooth power meter
			if powerMeterAddress == "" {
				return fmt.Errorf("required flag \"power-meter\" not set")
			}

			var powerDevice *bluetooth.Device

			err := adapter.Scan(func(a *bluetooth.Adapter, result bluetooth.ScanResult) {
				if powerDevice == nil && result.Address.String() == powerMeterAddress {
					device, err := adapter.Connect(result.Address, bluetooth.ConnectionParams{})
					must("connect to power meter", err)
					powerDevice = &device
					fmt.Println("Connected to power meter: ", result.LocalName())
					_ = adapter.StopScan()
				}
			})
			must("scan for power meter", err)

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

				// Every second, calculate average and update lights
				if time.Since(power.lastPrint) >= time.Second {
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
		}

		return nil
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
