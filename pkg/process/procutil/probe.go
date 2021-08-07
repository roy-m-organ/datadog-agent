package procutil

import (
	"time"
)

// Probe fetches process related info on current host
type Probe interface {
	Close()
	StatsForPIDs(pids []int32, now time.Time) (map[int32]*Stats, error)
	ProcessesByPID(now time.Time) (map[int32]*Process, error)
	StatsWithPermByPID(pids []int32) (map[int32]*StatsWithPerm, error)
	SetReturnZeroPermStats(enabled bool)
	SetWithPermission(enabled bool)
}

// Option is config options callback for system-probe
type Option func(p Probe)

// WithReturnZeroPermStats configures whether StatsWithPermByPID() returns StatsWithPerm that
// has zero values on all fields
func WithReturnZeroPermStats(enabled bool) Option {
	return func(p Probe) {
		p.SetReturnZeroPermStats(enabled)
	}
}

// WithPermission configures if process collection should fetch fields
// that require elevated permission or not
func WithPermission(enabled bool) Option {
	return func(p Probe) {
		p.SetWithPermission(enabled)
	}
}
