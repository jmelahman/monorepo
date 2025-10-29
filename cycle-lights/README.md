# Cycle Lights

A tool to control smart lights based on cycling power data, either from a Bluetooth power meter or the Zwift API.

## Features

- Control smart lights based on power zones
- Support for Bluetooth power meters
- Integration with Zwift API to get power data directly from Zwift sessions
- Configurable FTP (Functional Threshold Power) values
- Color-coded lighting based on power zones:
  - Zone 1 (Recovery): White
  - Zone 2 (Endurance): Blue
  - Zone 3 (Tempo): Green
  - Zone 4 (Threshold): Yellow
  - Zone 5 (VO2 Max): Orange
  - Zone 6 (Anaerobic): Red

## Installation

1. Install Go (version 1.16 or higher)
2. Clone this repository
3. Run `go mod tidy` to download dependencies
4. Build with `go build`

## Configuration

You can configure cycle-lights using command-line flags, environment variables, or a configuration file.

### Configuration File

By default, cycle-lights looks for a configuration file at `~/.config/cycle-lights/config.yaml`. You can specify a different file with the `--config` flag.

Example config.yaml:
