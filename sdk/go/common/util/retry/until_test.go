// Copyright 2023-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUntil_exhaustAttempts(t *testing.T) {
	t.Parallel()

	// Make the math for the test easy.
	delay := time.Second
	backoff := 2.0
	maxDelay := 100 * time.Second

	ctx := context.Background()
	errTooManyTries := errors.New("too many tries")
	afterRec := newAfterRecorder(time.Now())

	var attempts int
	_, _, err := (&Retryer{
		After: afterRec.After,
	}).Until(ctx, Acceptor{
		Delay:    &delay,
		Backoff:  &backoff,
		MaxDelay: &maxDelay,
		Accept: func(try int, delay time.Duration) (bool, interface{}, error) {
			if try > 3 {
				return false, nil, errTooManyTries
			}
			attempts++
			return false, nil, nil // operation failed
		},
	})
	assert.ErrorIs(t, err, errTooManyTries)
	assert.Equal(t, []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
	}, afterRec.Sleeps)
	assert.Equal(t, 4, attempts)
}

func TestUntil_contextExpired(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	ok, _, _ := (&Retryer{
		After: newAfterRecorder(time.Now()).After,
	}).Until(ctx, Acceptor{
		Accept: func(try int, delay time.Duration) (bool, interface{}, error) {
			if try > 2 {
				cancel()
			}
			return false, nil, nil
		},
	})
	assert.False(t, ok, "should not have succeeded")
	// This is surprising behavior (no error instead of ctx.Err())
	// but it's risky to change it right now.
	// Ideally, the assertion here would be:
	//   assert.ErrorIs(t, err, context.Canceled)
}

func TestUntil_maxDelay(t *testing.T) {
	t.Parallel()

	// Make the math for the test easy.
	delay := time.Second
	backoff := 2.0
	maxDelay := 10 * time.Second

	ctx := context.Background()
	afterRec := newAfterRecorder(time.Now())
	ok, _, err := (&Retryer{
		After: afterRec.After,
	}).Until(ctx, Acceptor{
		Delay:    &delay,
		Backoff:  &backoff,
		MaxDelay: &maxDelay,
		Accept: func(try int, delay time.Duration) (bool, interface{}, error) {
			// 100 tries should be enough to reach maxDelay.
			if try < 100 {
				return false, nil, nil
			}
			return true, nil, nil
		},
	})
	assert.True(t, ok)
	require.NoError(t, err)

	require.Len(t, afterRec.Sleeps, 100)
	for _, d := range afterRec.Sleeps {
		assert.LessOrEqual(t, d, maxDelay)
	}
}

// afterRecorder implements a time.After variant
// that records all requested sleeps,
// and advances the current time instantly.
type afterRecorder struct {
	mu  sync.Mutex
	now time.Time

	// Sleeps is the list of all sleeps requested.
	Sleeps []time.Duration
}

func newAfterRecorder(now time.Time) *afterRecorder {
	return &afterRecorder{
		now: now,
	}
}

func (r *afterRecorder) After(d time.Duration) <-chan time.Time {
	r.mu.Lock()
	r.Sleeps = append(r.Sleeps, d)
	r.now = r.now.Add(d)
	r.mu.Unlock()

	afterc := make(chan time.Time, 1)
	afterc <- r.now
	return afterc
}
