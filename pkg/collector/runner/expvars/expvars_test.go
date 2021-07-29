// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package expvars

import (
	"expvar"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/DataDog/datadog-agent/pkg/collector/check"
)

// Helper methods

func setUp() {
	ResetAllCheckStats()
}

func getRunnerExpvarMap(t assert.TestingT) *expvar.Map {
	runnerMapExpvar := expvar.Get("runner")
	if !assert.NotNil(t, runnerMapExpvar) {
		assert.FailNow(t, "Runner main key not found in expvars!")
	}

	return runnerMapExpvar.(*expvar.Map)
}

func getRunningChecksExpvarMap(t assert.TestingT) *expvar.Map {
	runnerMap := getRunnerExpvarMap(t)

	runningChecksExpvar := runnerMap.Get("running")
	if !assert.NotNil(t, runningChecksExpvar) {
		assert.FailNow(t, "List of running checks could not be found!")
	}

	return runningChecksExpvar.(*expvar.Map)
}

func getExpvarMapKeys(m *expvar.Map) []string {
	keys := make([]string, 0)

	m.Do(func(kv expvar.KeyValue) {
		keys = append(keys, kv.Key)
	})

	return keys
}

func assertKeyNotSet(t assert.TestingT, key string) {
	runnerMap := getRunnerExpvarMap(t)
	intExpvar := runnerMap.Get(key)
	if !assert.Nil(t, intExpvar) {
		assert.FailNow(t, fmt.Sprintf("Variable '%s' should not have been initially set!", key))
	}
}

func getRunnerExpvarInt(t assert.TestingT, key string) int {
	runnerMap := getRunnerExpvarMap(t)
	intExpvar := runnerMap.Get(key)
	if !assert.NotNil(t, intExpvar) {
		assert.FailNow(t, fmt.Sprintf("Variable '%s' not found in expvars!", key))
	}

	return int(intExpvar.(*expvar.Int).Value())
}

func changeAndAssertExpvarValue(t assert.TestingT, key string, f func(int), amount int, expectedVal int) {
	f(amount)

	actualValue := getRunnerExpvarInt(t, key)

	if !assert.Equal(t, expectedVal, actualValue) {
		assert.FailNow(
			t,
			fmt.Sprintf("Variable '%s' did not have the expected value of '%d' (was: '%d')!",
				key,
				expectedVal,
				actualValue,
			))
	}
}

type testCheck struct {
	check.StubCheck
	sync.Mutex
	doErr  bool
	hasRun bool
	id     string
	done   chan interface{}
}

func (c *testCheck) String() string { return c.id }
func (c *testCheck) ID() check.ID   { return check.ID(c.id) }

func newTestCheck(id string) *testCheck {
	return &testCheck{
		doErr: false,
		id:    id,
		done:  make(chan interface{}, 1),
	}
}

// Tests

func TestExpvarsInitialState(t *testing.T) {
	setUp()

	runnerMap := getRunnerExpvarMap(t)

	runningChecks := getRunningChecksExpvarMap(t)
	assert.Equal(t, 0, len(getExpvarMapKeys(runningChecks)))

	checks := runnerMap.Get("Checks")
	if !assert.NotNil(t, checks) {
		return
	}
	assert.Equal(t, "", checks.String())
}

func TestExpvarsInitialInternalState(t *testing.T) {
	setUp()
	assert.Equal(t, 0, len(GetCheckStats()))
}

func TestExpvarsResetAllCheckStats(t *testing.T) {
}

func TestExpvarsAddWorkStats(t *testing.T) {
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < 50; i++ {
		// Copy the index value since loop reuses pointers
		idx := i
		go func() {
			wg.Add(1)
			defer wg.Done()

			testCheck := newTestCheck(fmt.Sprintf("testcheck %d", idx))
			duration := time.Duration(idx)
			err := fmt.Errorf("error %d", idx)
			warnings := []error{
				fmt.Errorf("warning %d", idx),
				fmt.Errorf("warning2 %d", idx),
			}
			stats := check.SenderStats{}

			<-start

			AddCheckStats(testCheck, duration, err, warnings, stats)
		}()
	}

	// Start all goroutines
	close(start)

	wg.Wait()

	assert.Equal(t, 0, len(getExpvarMapKeys(getRunnerExpvarMap(t))))

	assert.FailNow(t, "WIP WIP WIP WIP WIP")
}

func TestExpvarsGetChecksStats(t *testing.T) {
	assert.FailNow(t, "IMPLEMENT ME")
}

func TestExpvarsGetChecksStatsClone(t *testing.T) {
	assert.FailNow(t, "IMPLEMENT ME")
}

func TestExpvarsRemoveCheckStats(t *testing.T) {
	// Remove check stats, add check stats
	assert.FailNow(t, "IMPLEMENT ME")
}

func TestExpvarsCheckStats(t *testing.T) {
	// "CheckStats()". Maybe join w/ above test?
	assert.FailNow(t, "IMPLEMENT ME")
}

func TestExpvarsRunningStats(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if !assert.Nil(t, err) {
		return
	}

	runningChecksMap := getRunningChecksExpvarMap(t)
	assert.Equal(t, 0, len(getExpvarMapKeys(runningChecksMap)))

	for idx := 0; idx < 10; idx++ {
		checkName := fmt.Sprintf("mycheck %d", idx)
		expectedTimestamp := time.Unix(int64(1234567890+idx), 0).In(loc)

		SetRunningStats(check.ID(checkName), expectedTimestamp)

		actualTimestamp := runningChecksMap.Get(checkName)
		assert.Equal(t, timestamp(expectedTimestamp).String(), actualTimestamp.String())
		assert.Equal(t, idx+1, len(getExpvarMapKeys(runningChecksMap)))
	}

	for idx := 0; idx < 10; idx++ {
		checkName := fmt.Sprintf("mycheck %d", idx)
		DeleteRunningStats(check.ID(checkName))

		checkTimestamp := runningChecksMap.Get(checkName)
		if !assert.Nil(t, checkTimestamp) {
			return
		}
		assert.Equal(t, 10-idx-1, len(getExpvarMapKeys(runningChecksMap)))
	}
}

func TestExpvarsToplevelKeys(t *testing.T) {
	setUp()

	for keyName, f := range map[string]func(int){
		"Errors":        AddErrorsCount,
		"Runs":          AddRunsCount,
		"RunningChecks": AddRunningCheckCount,
		"Warnings":      AddWarningsCount,
		"Workers":       AddWorkerCount,
	} {

		assertKeyNotSet(t, keyName)

		changeAndAssertExpvarValue(t, keyName, f, 5, 5)
		changeAndAssertExpvarValue(t, keyName, f, 1, 6)
		changeAndAssertExpvarValue(t, keyName, f, -5, 1)
		changeAndAssertExpvarValue(t, keyName, f, 3, 4)
	}
}
