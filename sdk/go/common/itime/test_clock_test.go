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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRealClock(t *testing.T) {
	t.Parallel()

	clock := NewRealClock()
	cnow := clock.Now()
	tnow := time.Now()

	if cnow.Sub(tnow) > time.Second {
		t.Fatalf("RealClock.Now() returned a time that was off by more than a second")
	}
}

func TestTestClock(t *testing.T) {
	t.Parallel()

	now := time.Now()
	clock := NewTestClock(now)

	assert.Equal(t, now, clock.Now())

	clock.Advance(time.Second)
	assert.Equal(t, now.Add(time.Second), clock.Now())
}

func TestSingleTicker(t *testing.T) {
	t.Parallel()

	now := time.Now()
	clock := NewTestClock(now)
	ticker := clock.NewTicker(time.Second)

	select {
	case <-ticker.C():
		t.Fatalf("Expected ticker to not fire immediately")
	default:
	}

	clock.Advance(time.Second)
	select {
	case <-ticker.C():
	default:
		t.Fatalf("Expected ticker to fire after advancing clock")
	}

	ticker.Stop()
	clock.Advance(time.Second)
	select {
	case <-ticker.C():
		t.Fatalf("Expected ticker to not fire after stopping")
	default:
	}
}

func TestMultipleTickers(t *testing.T) {
	t.Parallel()

	now := time.Now()
	clock := NewTestClock(now)
	ticker1 := clock.NewTicker(time.Second)
	ticker2 := clock.NewTicker(2 * time.Second)

	wait1 := func() {
		select {
		case <-ticker1.C():
		default:
			t.Fatal("Expected ticker1 to fire")
		}
	}
	wait2 := func() {
		select {
		case <-ticker2.C():
		default:
			t.Fatal("Expected ticker2 to fire")
		}
	}

	clock.Advance(time.Second)
	wait1()

	clock.Advance(time.Second)
	wait1()
	wait2()

	clock.Advance(time.Second * 2)
	wait1()
	wait1()
	wait2()
}
