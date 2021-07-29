// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/DataDog/datadog-agent/pkg/aggregator"
	"github.com/DataDog/datadog-agent/pkg/collector/check"
	"github.com/DataDog/datadog-agent/pkg/collector/runner/expvars"
	"github.com/DataDog/datadog-agent/pkg/collector/runner/tracker"
	"github.com/DataDog/datadog-agent/pkg/metrics"
	"github.com/DataDog/datadog-agent/pkg/util"
	"github.com/DataDog/datadog-agent/pkg/util/log"
)

// Worker is an object that encapsulates the logic to manage a loop of processing
// checks over the provided `PendingCheckChan`
type Worker struct {
	ID                      int
	ChecksTracker           *tracker.RunningChecksTracker
	PendingCheckChan        chan check.Check
	RunnerID                int
	ShouldAddCheckStatsFunc func(id check.ID) bool
}

// Run waits for checks and run them as long as they arrive on the channel
func (w *Worker) Run() {
	log.Debugf("Runner %d, worker %d: Ready to process checks...", w.RunnerID, w.ID)

	for check := range w.PendingCheckChan {
		checkLogger := CheckLogger{Check: check}

		// Add check to tracker if it's not already running
		if !w.ChecksTracker.AddCheck(check) {
			checkLogger.Debug("Check is already running, skipping execution...")
			continue
		}

		expvars.AddRunningCheckCount(1)

		checkLogger.CheckStarted()

		// run the check
		var checkErr error
		t0 := time.Now()

		expvars.SetRunningStats(check.ID(), t0)
		checkErr = check.Run()
		expvars.DeleteRunningStats(check.ID())

		longRunning := check.Interval() == 0

		checkWarnings := check.GetWarnings()

		// use the default sender for the service checks
		sender, err := aggregator.GetDefaultSender()
		if err != nil {
			log.Errorf("Error getting default sender: %v. Not sending status check for %s", err, check)
		}
		serviceCheckTags := []string{fmt.Sprintf("check:%s", check.String())}
		serviceCheckStatus := metrics.ServiceCheckOK

		hostname, _ := util.GetHostname(context.TODO())

		if len(checkWarnings) != 0 {
			expvars.AddWarningsCount(len(checkWarnings))
			serviceCheckStatus = metrics.ServiceCheckWarning
		}

		if checkErr != nil {
			checkLogger.Error(checkErr)
			expvars.AddErrorsCount(1)
			serviceCheckStatus = metrics.ServiceCheckCritical
		}

		if sender != nil && !longRunning {
			sender.ServiceCheck("datadog.agent.check_status", serviceCheckStatus, hostname, serviceCheckTags, "")
			sender.Commit()
		}

		// remove the check from the running list
		w.ChecksTracker.DeleteCheck(check.ID())

		// publish statistics about this run
		expvars.AddRunningCheckCount(-1)
		expvars.AddRunsCount(1)

		if !longRunning || len(checkWarnings) != 0 || checkErr != nil {
			// If the scheduler isn't assigned (it should), just add stats
			// otherwise only do so if the check is in the scheduler
			if w.ShouldAddCheckStatsFunc(check.ID()) {
				sStats, _ := check.GetSenderStats()
				expvars.AddCheckStats(check, time.Since(t0), checkErr, checkWarnings, sStats)
			}
		}

		checkLogger.CheckFinshed()
	}

	log.Debugf("Runner %d, worker %d: Finished processing checks.", w.RunnerID, w.ID)
}
