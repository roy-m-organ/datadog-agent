// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package expvars

import (
	"expvar"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/collector/check"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

const (
	checksExpvarKey        = "Checks"
	errorsExpvarKey        = "Errors"
	runningChecksExpvarKey = "RunningChecks"
	runnerExpvarKey        = "runner"
	runsExpvarKey          = "Runs"
	runningExpvarKey       = "running"
	warningsExpvarKey      = "Warnings"
	workersExpvarKey       = "Workers"
)

var (
	expvarRunnerStats        *expvar.Map
	expvarRunningChecksStats *expvar.Map
	expvarCheckStats         *expvarRunnerCheckStats
)

// expvarRunnerCheckStats holds the stats from the running checks
type expvarRunnerCheckStats struct {
	stats     map[string]map[check.ID]*check.Stats
	statsLock sync.RWMutex
}

func init() {
	expvarRunnerStats = expvar.NewMap(runnerExpvarKey)

	expvarRunningChecksStats = &expvar.Map{}
	expvarRunnerStats.Set(runningExpvarKey, expvarRunningChecksStats)

	expvarRunnerStats.Set(checksExpvarKey, expvar.Func(expCheckStatsFunc))

	expvarCheckStats = &expvarRunnerCheckStats{
		stats: make(map[string]map[check.ID]*check.Stats),
	}
}

// Functions relating to check run stats (`expvarCheckStats`)

func expCheckStatsFunc() interface{} {
	return GetCheckStats
}

// ResetAllCheckStats clears all work stats collected so far (useful in testing)
func ResetAllCheckStats() {
	log.Warnf("Resetting all check stats")

	expvarCheckStats.statsLock.Lock()
	defer expvarCheckStats.statsLock.Unlock()

	for key, _ := range expvarCheckStats.stats {
		delete(expvarCheckStats.stats, key)
	}
}

// GetCheckStats returns the check stats map
func GetCheckStats() map[string]map[check.ID]*check.Stats {
	expvarCheckStats.statsLock.RLock()
	defer expvarCheckStats.statsLock.RUnlock()

	// Because the returned maps will be used after the lock is released, and
	// thus when they might be further modified, we must clone them here.  The
	// map values (`check.Stats`) are threadsafe and need not be cloned.

	cloned := make(map[string]map[check.ID]*check.Stats)
	for k, v := range expvarCheckStats.stats {
		innerCloned := make(map[check.ID]*check.Stats)
		for innerK, innerV := range v {
			innerCloned[innerK] = innerV
		}
		cloned[k] = innerCloned
	}

	return cloned
}

// AddCheckStats adds runtime stats to the check's expvars
func AddCheckStats(
	c check.Check,
	execTime time.Duration,
	err error,
	warnings []error,
	mStats check.SenderStats,
) {

	var s *check.Stats

	expvarCheckStats.statsLock.Lock()
	defer expvarCheckStats.statsLock.Unlock()

	log.Tracef("Adding stats for %s", string(c.ID()))

	stats, found := expvarCheckStats.stats[c.String()]
	if !found {
		stats = make(map[check.ID]*check.Stats)
		expvarCheckStats.stats[c.String()] = stats
	}

	s, found = stats[c.ID()]
	if !found {
		s = check.NewStats(c)
		stats[c.ID()] = s
	}

	s.Add(execTime, err, warnings, mStats)
}

// RemoveCheckStats removes a check from the check stats map
func RemoveCheckStats(checkID check.ID) {
	expvarCheckStats.statsLock.Lock()
	defer expvarCheckStats.statsLock.Unlock()

	log.Debugf("Removing stats for %s", string(checkID))

	checkName := strings.Split(string(checkID), ":")[0]
	stats, found := expvarCheckStats.stats[checkName]

	if !found {
		log.Warnf("Stats for check %s not found", string(checkID))
		return
	}

	delete(stats, checkID)

	if len(stats) == 0 {
		delete(expvarCheckStats.stats, checkName)
	}
}

// CheckStats returns the check stats of a check, if they can be found
func CheckStats(id check.ID) (*check.Stats, bool) {
	name := strings.Split(string(id), ":")[0]

	expvarCheckStats.statsLock.RLock()
	defer expvarCheckStats.statsLock.RUnlock()

	stats, nameFound := expvarCheckStats.stats[name]

	if !nameFound {
		return nil, false
	}

	check, checkFound := stats[id]
	if !checkFound {
		// This in theory should never happen
		log.Warnf("Check %s is in stats map but the object is missing the check itself")
		return nil, false
	}

	return check, true
}

// Functions relating to running checks state map (`expvarRunningChecksStats`)

// SetRunningStats sets the start time of a running check
func SetRunningStats(id check.ID, t time.Time) {
	expvarRunningChecksStats.Set(string(id), timestamp(t))
}

// DeleteRunningStats clears the start time of a check when it's complete
func DeleteRunningStats(id check.ID) {
	expvarRunningChecksStats.Delete(string(id))
}

// Functions relating to top-level runner expvars (`expvarRunnerStats`)

// AddWorkerCount is used to increment and decrement the 'Worker' expvar
func AddWorkerCount(amount int) {
	expvarRunnerStats.Add(workersExpvarKey, int64(amount))
}

// AddRunningCheckCount is used to increment and decrement the 'RunningChecks' expvar
func AddRunningCheckCount(amount int) {
	expvarRunnerStats.Add(runningChecksExpvarKey, int64(amount))
}

// AddRunsCount is used to increment and decrement the 'Runs' expvar
func AddRunsCount(amount int) {
	expvarRunnerStats.Add(runsExpvarKey, int64(amount))
}

// AddWarningsCount is used to increment the 'Warnings' expvar
func AddWarningsCount(amount int) {
	expvarRunnerStats.Add(warningsExpvarKey, int64(amount))
}

// AddErrorsCount is used to increment the 'Errors' expvar
func AddErrorsCount(amount int) {
	expvarRunnerStats.Add(errorsExpvarKey, int64(amount))
}
