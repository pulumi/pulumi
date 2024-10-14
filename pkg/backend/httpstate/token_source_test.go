// Copyright 2016-2022, Pulumi Corporation.
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

package httpstate

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dur := 20 * time.Millisecond
	clock := clockwork.NewFakeClock()
	backend := &testTokenBackend{tokens: map[string]time.Time{}, clock: clock, t: t}

	tok0, tok0Expires := backend.NewToken(dur)
	ts, err := newTokenSource(ctx, clock, tok0, tok0Expires, dur, backend.Refresh)
	assert.NoError(t, err)
	defer ts.Close()

	for i := 0; i < 64; i++ {
		tok, err := ts.GetToken(ctx)
		assert.NoError(t, err)
		assert.NoError(t, backend.VerifyToken(tok))
		t.Logf("STEP: %d, TOKEN: %s", i, tok)

		// tok0 initially
		if i == 0 {
			assert.Equal(t, tok0, tok)
		}

		// definitely a fresh token by step 32
		// allow some leeway due to time.Sleep concurrency
		if i > 32 {
			assert.NotEqual(t, tok0, tok)
		}

		clock.Advance(dur / 16)
	}
}

func TestTokenSourceWithQuicklyExpiringInitialToken(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dur := 80 * time.Millisecond
	clock := clockwork.NewFakeClock()
	backend := &testTokenBackend{tokens: map[string]time.Time{}, clock: clock, t: t}

	tok0, tok0Expires := backend.NewToken(dur / 10) // token expires after 8ms
	ts, err := newTokenSource(ctx, clock, tok0, tok0Expires, dur, backend.Refresh)
	require.NoError(t, err)
	defer ts.Close()

	for i := 0; i < 80; i++ {
		tok, err := ts.GetToken(ctx)
		require.NoError(t, err)
		require.NoError(t, backend.VerifyToken(tok))
		t.Logf("STEP: %d, TOKEN: %s", i, tok)
		clock.Advance(dur / 80)
	}
}

type testTokenBackend struct {
	mu                  sync.Mutex
	counter             int
	tokens              map[string]time.Time
	clock               clockwork.Clock
	networkErrorCounter int
	t                   *testing.T
}

func (ts *testTokenBackend) NewToken(duration time.Duration) (string, time.Time) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.newTokenInner(duration)
}

func (ts *testTokenBackend) Refresh(
	ctx context.Context,
	duration time.Duration,
	currentToken string,
) (string, time.Time, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Simulate some network errors.  We retry getting the token
	// after half the token duration has elapsed, and we run the
	// refresh function every 1/8th of the token duration. This
	// means we have 4 retries before the token expires. We want
	// to simulate some network errors, but not so many that we
	// can hit the retry limit, and the test flakes.
	if ts.networkErrorCounter < 2 && rand.Float32() < 0.1 { //nolint:gosec // test is not security sensitive
		ts.networkErrorCounter++
		ts.t.Log("network error")
		return "", time.Time{}, errors.New("network error")
	}

	ts.networkErrorCounter = 0

	if err := ts.verifyTokenInner(currentToken); err != nil {
		return "", time.Time{}, err
	}
	tok, expires := ts.newTokenInner(duration)
	return tok, expires, nil
}

func (ts *testTokenBackend) TokenName(refreshCount int) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.tokenNameInner(refreshCount)
}

func (ts *testTokenBackend) VerifyToken(token string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.verifyTokenInner(token)
}

func (ts *testTokenBackend) newTokenInner(duration time.Duration) (string, time.Time) {
	now := ts.clock.Now()
	ts.counter++
	tok := ts.tokenNameInner(ts.counter)
	expires := now.Add(duration)
	ts.tokens[tok] = now.Add(duration)
	return tok, expires
}

func (ts *testTokenBackend) tokenNameInner(refreshCount int) string {
	return fmt.Sprintf("token-%d", ts.counter)
}

func (ts *testTokenBackend) verifyTokenInner(token string) error {
	now := ts.clock.Now()
	expires, gotCurrentToken := ts.tokens[token]
	if !gotCurrentToken {
		return expiredTokenError{fmt.Errorf("unknown token: %v", token)}
	}

	if now.After(expires) {
		return expiredTokenError{fmt.Errorf("expired token %v (%v past expiration)",
			token, now.Sub(expires))}
	}
	return nil
}
