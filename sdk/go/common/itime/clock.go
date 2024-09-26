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

import "time"

// Clock is an interface that abstracts time.Now and time.NewTicker for testing.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// NewTicker returns a new Ticker that will fire every d.
	NewTicker(d time.Duration) Ticker
}

// Ticker is an interface that abstracts time.Ticker for testing.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct {
	t *time.Ticker
}

func (rt realTicker) C() <-chan time.Time {
	return rt.t.C
}

func (rt realTicker) Stop() {
	rt.t.Stop()
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func (realClock) NewTicker(d time.Duration) Ticker {
	return realTicker{time.NewTicker(d)}
}

// NewRealClock returns a Clock that uses the system clock.
func NewRealClock() Clock {
	return realClock{}
}
