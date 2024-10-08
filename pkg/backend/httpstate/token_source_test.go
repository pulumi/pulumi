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
)

func TestTokenSource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dur := 20 * time.Millisecond
	clock := clockwork.NewFakeClock()
	backend := &testTokenBackend{tokens: map[string]time.Time{}, clock: clock}

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
	dur := 40 * time.Millisecond
	clock := clockwork.NewFakeClock()
	backend := &testTokenBackend{tokens: map[string]time.Time{}, clock: clock}

	tok0, tok0Expires := backend.NewToken(dur / 10)
	ts, err := newTokenSource(ctx, clock, tok0, tok0Expires, dur, backend.Refresh)
	assert.NoError(t, err)
	defer ts.Close()

	for i := 0; i < 4; i++ {
		tok, err := ts.GetToken(ctx)
		assert.NoError(t, err)
		assert.NoError(t, backend.VerifyToken(tok))
		t.Logf("STEP: %d, TOKEN: %s", i, tok)
		clock.Advance(dur / 16)
	}
}

type testTokenBackend struct {
	mu      sync.Mutex
	counter int
	tokens  map[string]time.Time
	clock   clockwork.Clock
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

	// Simulate some network errors
	if rand.Float32() < 0.1 { //nolint:gosec // test is not security sensitive
		return "", time.Time{}, errors.New("network error")
	}

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
