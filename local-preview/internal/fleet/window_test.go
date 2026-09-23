package fleet

import (
	"testing"
	"time"
)

// at builds a UTC instant on a given 2026 date. Jan 5 2026 is a Monday.
func at(day, hour, minute int) time.Time {
	return time.Date(2026, time.January, day, hour, minute, 0, 0, time.UTC)
}

func TestParseMinWarmWindow(t *testing.T) {
	cases := []struct {
		spec string
		time time.Time
		want bool
	}{
		// Weekday business hours, UTC default.
		{"Mon-Fri 08:00-18:00", at(5, 9, 0), true},    // Mon morning
		{"Mon-Fri 08:00-18:00", at(5, 7, 59), false},  // before opening
		{"Mon-Fri 08:00-18:00", at(5, 18, 0), false},  // end is exclusive
		{"Mon-Fri 08:00-18:00", at(10, 12, 0), false}, // Saturday
		// Day list.
		{"Mon,Wed 08:00-18:00", at(6, 12, 0), false}, // Tuesday
		{"Mon,Wed 08:00-18:00", at(7, 12, 0), true},  // Wednesday
		// Wrapping day range: Fri-Mon = Fri,Sat,Sun,Mon.
		{"Fri-Mon 08:00-18:00", at(10, 12, 0), true}, // Saturday
		{"Fri-Mon 08:00-18:00", at(7, 12, 0), false}, // Wednesday
		// Overnight span belongs to the day it starts on.
		{"Fri 18:00-02:00", at(9, 23, 0), true},   // Fri night
		{"Fri 18:00-02:00", at(10, 1, 0), true},   // Sat 01:00, still Friday's window
		{"Fri 18:00-02:00", at(10, 3, 0), false},  // Sat 03:00
		{"Fri 18:00-02:00", at(11, 1, 0), false},  // Sun 01:00 (Saturday isn't active)
		// Timezone: 09:00 in Chicago is 15:00 UTC in January (CST, UTC-6).
		{"Mon-Fri 08:00-18:00 America/Chicago", at(5, 15, 0), true},
		{"Mon-Fri 08:00-18:00 America/Chicago", at(5, 9, 0), false}, // 03:00 in Chicago
	}
	for _, c := range cases {
		active, err := ParseMinWarmWindow(c.spec)
		if err != nil {
			t.Fatalf("parse %q: %v", c.spec, err)
		}
		if got := active(c.time); got != c.want {
			t.Errorf("%q at %s = %v, want %v", c.spec, c.time, got, c.want)
		}
	}
}

func TestParseMinWarmWindowRejects(t *testing.T) {
	for _, spec := range []string{
		"",
		"08:00-18:00",                   // no days
		"Mon-Fri",                       // no span
		"Mon-Fri 8am-6pm",               // not 24h HH:MM
		"Mon-Fri 08:00-08:00",           // empty span
		"Mon-Funday 08:00-18:00",        // bad day
		"Mon-Fri 08:00-18:00 Not/AZone", // bad tz
		"Mon-Fri 08:00-18:00 UTC extra", // trailing garbage
	} {
		if _, err := ParseMinWarmWindow(spec); err == nil {
			t.Errorf("ParseMinWarmWindow(%q) succeeded, want error", spec)
		}
	}
}
