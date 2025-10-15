package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	viper.BindPFlag("ftp", rootCmd.Flags().Lookup("ftp"))
	viper.BindPFlag("power_meter", rootCmd.Flags().Lookup("power-meter"))
	viper.BindPFlag("smart_light", rootCmd.Flags().Lookup("smart-light"))

	// Mark flags as required
	rootCmd.MarkFlagRequired("ftp")
	rootCmd.MarkFlagRequired("power-meter")
	rootCmd.MarkFlagRequired("smart-light")
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

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

var rootCmd = &cobra.Command{
	Use:   "cycle-lights",
	Short: "Control smart lights based on cycling power data",
	Long:  "A tool to control smart lights based on power meter data from cycling",
	Run: func(cmd *cobra.Command, args []string) {
		ftp := viper.GetInt("ftp")
		powerMeter := viper.GetString("power_meter")
		smartLight := viper.GetString("smart_light")

		fmt.Printf("FTP: %d\n", ftp)
		fmt.Printf("Power Meter: %s\n", powerMeter)
		fmt.Printf("Smart Light: %s\n", smartLight)

		// TODO: Implement the actual light control logic here
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
