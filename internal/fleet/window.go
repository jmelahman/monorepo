package fleet

// A wall-clock window for the min-warm floor: "Mon-Fri 08:00-18:00
// America/Chicago" keeps the floor active during working hours and lets the
// fleet drain to zero overnight. Parsing lives here because the fleet
// registry is what consumes the predicate (SetMinWarmWindow); the CLI only
// forwards the string.

import (
	"fmt"
	"strings"
	"time"
)

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday,
	"wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday,
	"sat": time.Saturday,
}

// ParseMinWarmWindow parses "<days> <start>-<end> [tz]" into an active-time
// predicate:
//
//   - days: a name ("Mon"), a list ("Mon,Wed,Fri"), or a range ("Mon-Fri" —
//     wrapping allowed, "Fri-Mon" means Fri,Sat,Sun,Mon)
//   - start/end: 24h "HH:MM"; start > end wraps midnight ("18:00-02:00")
//   - tz: an IANA zone name; omitted means UTC (the fleet's hosts run UTC,
//     so an unqualified window means what the machines mean)
//
// The predicate evaluates the day in the window's zone, so an overnight-
// wrapping window belongs to the day it starts on.
func ParseMinWarmWindow(s string) (func(time.Time) bool, error) {
	fields := strings.Fields(s)
	if len(fields) < 2 || len(fields) > 3 {
		return nil, fmt.Errorf("min-warm window %q: want \"<days> <HH:MM-HH:MM> [tz]\"", s)
	}
	days, err := parseDays(fields[0])
	if err != nil {
		return nil, fmt.Errorf("min-warm window %q: %w", s, err)
	}
	start, end, err := parseSpan(fields[1])
	if err != nil {
		return nil, fmt.Errorf("min-warm window %q: %w", s, err)
	}
	loc := time.UTC
	if len(fields) == 3 {
		if loc, err = time.LoadLocation(fields[2]); err != nil {
			return nil, fmt.Errorf("min-warm window %q: %w", s, err)
		}
	}
	return func(t time.Time) bool {
		t = t.In(loc)
		minute := t.Hour()*60 + t.Minute()
		if start <= end {
			return days[t.Weekday()] && minute >= start && minute < end
		}
		// Overnight wrap: the early-morning tail belongs to the day the
		// window STARTED on, so Fri 18:00-02:00 covers Sat 01:00 iff Fri is
		// an active day.
		if minute >= start {
			return days[t.Weekday()]
		}
		if minute < end {
			return days[t.Add(-24*time.Hour).Weekday()]
		}
		return false
	}, nil
}

// parseDays reads a day name, comma list, or wrapping range into a set.
func parseDays(s string) (map[time.Weekday]bool, error) {
	set := map[time.Weekday]bool{}
	if from, to, ok := strings.Cut(s, "-"); ok {
		f, fok := weekdays[strings.ToLower(from)]
		t, tok := weekdays[strings.ToLower(to)]
		if !fok || !tok {
			return nil, fmt.Errorf("bad day range %q", s)
		}
		for d := f; ; d = (d + 1) % 7 {
			set[d] = true
			if d == t {
				return set, nil
			}
		}
	}
	for _, name := range strings.Split(s, ",") {
		d, ok := weekdays[strings.ToLower(name)]
		if !ok {
			return nil, fmt.Errorf("bad day %q", name)
		}
		set[d] = true
	}
	return set, nil
}

// parseSpan reads "HH:MM-HH:MM" into minutes-of-day. start == end is
// rejected: it reads as either always or never, and whichever the caller
// meant, spelling it out beats guessing.
func parseSpan(s string) (start, end int, err error) {
	from, to, ok := strings.Cut(s, "-")
	if !ok {
		return 0, 0, fmt.Errorf("bad time span %q", s)
	}
	if start, err = parseHHMM(from); err != nil {
		return 0, 0, err
	}
	if end, err = parseHHMM(to); err != nil {
		return 0, 0, err
	}
	if start == end {
		return 0, 0, fmt.Errorf("empty time span %q", s)
	}
	return start, end, nil
}

func parseHHMM(s string) (int, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("bad time %q (want 24h HH:MM)", s)
	}
	return h*60 + m, nil
}
