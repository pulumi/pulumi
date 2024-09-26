// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package itime

import (
	"slices"
	"sync"
	"time"
)

// TestClock is a Clock that can be manually advanced in time for testing. Unlike tickers from the real clock, test
// tickers do not drop ticks if the channel is not read in time. They are buffered to thousands of ticks to allow easier
// test writing.
type TestClock struct {
	now time.Time

	mu      sync.Mutex
	tickers []*testTicker
}

type testTicker struct {
	c     chan time.Time
	d     time.Duration
	t     time.Time
	clock *TestClock
}

func (tt *testTicker) C() <-chan time.Time {
	return tt.c
}

func (tt *testTicker) Stop() {
	tt.clock.mu.Lock()
	defer tt.clock.mu.Unlock()
	i := slices.Index(tt.clock.tickers, tt)
	tt.clock.tickers = slices.Delete(tt.clock.tickers, i, i+1)
}

func (tc *TestClock) Now() time.Time {
	return tc.now
}

func (tc *TestClock) NewTicker(d time.Duration) Ticker {
	ticker := &testTicker{
		// Buffered channel to avoid blocking in Advance
		c:     make(chan time.Time, 1000),
		t:     tc.now,
		d:     d,
		clock: tc,
	}
	tc.mu.Lock()
	tc.tickers = append(tc.tickers, ticker)
	// testTimers need sorting by duration
	slices.SortFunc(tc.tickers, func(a, b *testTicker) int {
		if a.d.Nanoseconds() < b.d.Nanoseconds() {
			return -1
		}
		if a.d.Nanoseconds() > b.d.Nanoseconds() {
			return 1
		}
		return 0
	})
	tc.mu.Unlock()
	return ticker
}

// Advance advances the clock by the given duration, firing any tickers that are due.
func (tc *TestClock) Advance(d time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.now = tc.now.Add(d)

	// keep ticking till everything has caught up
	anyTicks := true
	for anyTicks {
		anyTicks = false
		for _, ticker := range tc.tickers {
			next := ticker.t.Add(ticker.d)
			if !tc.now.Before(next) { // next <= now
				// Need to fire this ticker at least once
				ticker.c <- next
				ticker.t = next
				anyTicks = true
			}
		}
	}
}

// NewTestClock returns a new TestClock that starts at the given time.
func NewTestClock(now time.Time) *TestClock {
	return &TestClock{
		now: now,
	}
}
