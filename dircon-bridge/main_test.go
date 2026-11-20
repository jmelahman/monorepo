package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jmelahman/dircon-bridge/internal/bluez"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "help", args: []string{"-h"}, wantErr: flag.ErrHelp},
		// flag reports it; main only sets the exit status.
		{name: "unknown flag", args: []string{"-nope"}, wantErr: errUsage},
		{name: "bad address", args: []string{"D6:F8:D9:7F:99"}, wantErr: bluez.ErrBadAddress},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr strings.Builder
			if err := run(t.Context(), tt.args, &stdout, &stderr); !errors.Is(err, tt.wantErr) {
				t.Fatalf("run(%q) = %v, want %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if err := run(t.Context(), []string{"-version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() == 0 {
		t.Error("no version printed")
	}
}

var (
	trainer  = bluez.DeviceInfo{Address: "D6:F8:D9:7F:99:4B", Name: "Magnus", UUIDs: []string{uuidFitnessMachine, uuidCyclingPower}}
	trainer2 = bluez.DeviceInfo{Address: "AA:F8:D9:7F:99:4B", Name: "Other", UUIDs: []string{uuidCyclingPower}}
	monitor  = bluez.DeviceInfo{Address: "F5:B4:81:3D:07:26", Name: "64558-1", UUIDs: []string{uuidHeartRate}}
)

// fakeScanner returns one scan's results per call, then nothing.
type fakeScanner struct {
	scans [][]bluez.DeviceInfo
	asked [][]string
}

func (s *fakeScanner) Discover(_ context.Context, _ time.Duration, uuids ...string) ([]bluez.DeviceInfo, error) {
	s.asked = append(s.asked, uuids)
	if len(s.scans) == 0 {
		return nil, nil
	}
	found := s.scans[0]
	s.scans = s.scans[1:]
	return found, nil
}

func TestFind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		scans     [][]bluez.DeviceInfo
		want      []string
		wantScans int
		wantErr   error
	}{
		{
			name:      "both at once",
			scans:     [][]bluez.DeviceInfo{{monitor, trainer}},
			want:      []string{trainer.Address, monitor.Address},
			wantScans: 1,
		},
		{
			name:      "keeps looking for the other kind",
			scans:     [][]bluez.DeviceInfo{nil, {trainer}, nil, {monitor}},
			want:      []string{trainer.Address, monitor.Address},
			wantScans: 4,
		},
		{
			name:      "gives up on the other kind",
			scans:     [][]bluez.DeviceInfo{{monitor}},
			want:      []string{monitor.Address},
			wantScans: 1 + followUpScans,
		},
		{
			name:      "several trainers",
			scans:     [][]bluez.DeviceInfo{{trainer, trainer2, monitor}},
			wantScans: 1,
			wantErr:   ErrAmbiguous,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := &fakeScanner{scans: tt.scans}
			var started []string
			err := find(t.Context(), sc, time.Millisecond, slog.New(slog.DiscardHandler), func(addr string) error {
				started = append(started, addr)
				return nil
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("find = %v, want %v", err, tt.wantErr)
			}
			if !slices.Equal(started, tt.want) {
				t.Errorf("started %v, want %v", started, tt.want)
			}
			if len(sc.asked) != tt.wantScans {
				t.Errorf("scanned %d times, want %d", len(sc.asked), tt.wantScans)
			}
		})
	}
}

// Once a kind is found, scans look only for the other.
func TestFindNarrowsScan(t *testing.T) {
	t.Parallel()

	sc := &fakeScanner{scans: [][]bluez.DeviceInfo{{trainer}, {monitor}}}
	if err := find(t.Context(), sc, time.Millisecond, slog.New(slog.DiscardHandler), func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{uuidFitnessMachine, uuidCyclingPower, uuidHeartRate}, {uuidHeartRate}}
	if !slices.EqualFunc(sc.asked, want, slices.Equal) {
		t.Errorf("scanned for %v, want %v", sc.asked, want)
	}
}

func TestClassify(t *testing.T) {
	t.Parallel()

	both := bluez.DeviceInfo{Address: "11:22:33:44:55:66", UUIDs: []string{uuidHeartRate, uuidFitnessMachine}}
	other := bluez.DeviceInfo{Address: "11:22:33:44:55:77", UUIDs: []string{"0000180f-0000-1000-8000-00805f9b34fb"}}
	trainers, monitors := classify([]bluez.DeviceInfo{trainer, monitor, both, other})
	addrs := func(ds []bluez.DeviceInfo) []string {
		out := make([]string, 0, len(ds))
		for _, d := range ds {
			out = append(out, d.Address)
		}
		return out
	}
	// A trainer with a heart rate service is still a trainer.
	if got, want := addrs(trainers), []string{trainer.Address, both.Address}; !slices.Equal(got, want) {
		t.Errorf("trainers = %v, want %v", got, want)
	}
	if got, want := addrs(monitors), []string{monitor.Address}; !slices.Equal(got, want) {
		t.Errorf("monitors = %v, want %v", got, want)
	}
}
