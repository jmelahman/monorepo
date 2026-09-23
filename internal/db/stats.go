package db

import "fmt"

// StartupDurations returns cold-start wall-clock times, in seconds, derived
// from the process_events trail: each 'healthy' event paired with its most
// recent preceding 'start_attempt' for the same (repo, backend hash). Pairing
// on the nearest prior start_attempt (not the next healthy) yields one duration
// per successful start and stays correct across retries, where several
// start_attempts precede one healthy.
//
// Only starts whose 'healthy' landed within the last withinDays are returned,
// keeping the percentile window representative of current behavior. Durations
// are computed in SQLite via julianday deltas; the caller aggregates.
func (s *Store) StartupDurations(withinDays int) ([]float64, error) {
	if withinDays <= 0 {
		withinDays = 30
	}
	since := fmt.Sprintf("-%d days", withinDays)
	rows, err := s.db.Query(
		`SELECT (julianday(h.occurred_at) - julianday(s.occurred_at)) * 86400.0 AS secs
		   FROM process_events h
		   JOIN process_events s
		     ON s.repo_id = h.repo_id AND s.be_hash = h.be_hash
		    AND s.event = 'start_attempt'
		    AND s.id = (
		        SELECT MAX(s2.id) FROM process_events s2
		         WHERE s2.repo_id = h.repo_id AND s2.be_hash = h.be_hash
		           AND s2.event = 'start_attempt' AND s2.id < h.id
		    )
		  WHERE h.event = 'healthy'
		    AND h.occurred_at >= strftime('%Y-%m-%dT%H:%M:%SZ', 'now', ?)`,
		since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []float64{}
	for rows.Next() {
		var secs float64
		if err := rows.Scan(&secs); err != nil {
			return nil, err
		}
		// A clock skew or out-of-order write can yield a negative delta;
		// drop it rather than poison the percentiles.
		if secs >= 0 {
			out = append(out, secs)
		}
	}
	return out, rows.Err()
}
