// Command dircon-bridge relays Bluetooth LE fitness devices, such as smart
// trainers and heart rate monitors, to Zwift over Wahoo Direct Connect
// (DIRCON).
//
// It is for setups where Zwift can't use Bluetooth itself, such as Zwift under
// Wine, whose Bluetooth LE support is incomplete. Each device is reached
// through BlueZ and announced over mDNS as `_wahoo-fitness-tnp._tcp`, and
// service discovery, reads, writes and notifications are relayed between the
// two.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/brutella/dnssd"
	"golang.org/x/sync/errgroup"

	"github.com/jmelahman/dircon-bridge/internal/bluez"
	"github.com/jmelahman/dircon-bridge/internal/bridge"
	"github.com/jmelahman/dircon-bridge/internal/version"
)

// The services a device has to advertise to be found without an address.
const (
	uuidFitnessMachine = "00001826-0000-1000-8000-00805f9b34fb"
	uuidCyclingPower   = "00001818-0000-1000-8000-00805f9b34fb"
	uuidHeartRate      = "0000180d-0000-1000-8000-00805f9b34fb"
)

// ErrAmbiguous is returned when scanning finds more than one device of a kind.
var ErrAmbiguous = errors.New("found several devices of one kind; pass the addresses to bridge")

var programName = filepath.Base(os.Args[0])

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, programName+":", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: "+programName+" [flags] [address...]")
		_, _ = fmt.Fprintln(stderr, "\nWith no addresses, bridges the one trainer and the one heart rate monitor found by scanning.")
		fs.PrintDefaults()
	}
	var (
		port        = fs.Int("port", 36866, "TCP port for the first device; each further device uses the next one")
		scan        = fs.Duration("scan", 10*time.Second, "how long to scan when no addresses are given")
		verbose     = fs.Bool("v", false, "log every message relayed")
		showVersion = fs.Bool("version", false, "print the version and exit")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(stdout, version.Version())
		return err
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: level}))

	bz, err := bluez.New()
	if err != nil {
		return fmt.Errorf("connect to BlueZ: %w", err)
	}
	defer func() { _ = bz.Close() }()

	addresses := fs.Args()
	if len(addresses) == 0 {
		if addresses, err = find(ctx, bz, *scan, log); err != nil {
			return err
		}
	}

	responder, err := dnssd.NewResponder()
	if err != nil {
		return fmt.Errorf("start mDNS: %w", err)
	}
	g, ctx := errgroup.WithContext(ctx)
	for i, addr := range addresses {
		dev, err := bz.Device(ctx, addr)
		if err != nil {
			return err
		}
		b := bridge.New(dev, *port+i, responder, log)
		g.Go(func() error { return b.Run(ctx) })
	}
	g.Go(func() error {
		if err := responder.Respond(ctx); err != nil && ctx.Err() == nil {
			return fmt.Errorf("mDNS: %w", err)
		}
		return nil
	})
	return g.Wait()
}

// find scans until it sees a trainer or a heart rate monitor, and returns the
// addresses of the one of each kind it found.
func find(ctx context.Context, bz *bluez.Client, d time.Duration, log *slog.Logger) ([]string, error) {
	for {
		log.Info("scanning for a trainer and a heart rate monitor (wake them if they are asleep)")
		found, err := bz.Discover(ctx, d, uuidFitnessMachine, uuidCyclingPower, uuidHeartRate)
		if err != nil {
			return nil, err
		}
		var trainers, monitors []bluez.DeviceInfo
		for _, dev := range found {
			switch {
			case slices.Contains(dev.UUIDs, uuidFitnessMachine), slices.Contains(dev.UUIDs, uuidCyclingPower):
				trainers = append(trainers, dev)
			case slices.Contains(dev.UUIDs, uuidHeartRate):
				monitors = append(monitors, dev)
			}
		}
		if len(trainers) > 1 || len(monitors) > 1 {
			var b strings.Builder
			for _, dev := range slices.Concat(trainers, monitors) {
				fmt.Fprintf(&b, "\n  %s  %s", dev.Address, dev.Name)
			}
			return nil, fmt.Errorf("%w:%s", ErrAmbiguous, b.String())
		}
		var addresses []string
		for _, dev := range slices.Concat(trainers, monitors) {
			log.Info("found", "name", dev.Name, "address", dev.Address)
			addresses = append(addresses, dev.Address)
		}
		if len(addresses) > 0 {
			return addresses, nil
		}
	}
}
