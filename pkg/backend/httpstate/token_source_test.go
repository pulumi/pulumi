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
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenSource(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Flaky on Windows CI workers due to the use of timer+Sleep")
	}
	t.Parallel()

	ctx := context.Background()
	dur := 20 * time.Millisecond
	backend := &testTokenBackend{tokens: map[string]time.Time{}}

	tok0, tok0Expires := backend.NewToken(dur)
	ts, err := newTokenSource(ctx, tok0, tok0Expires, dur, backend.Refresh)
	assert.NoError(t, err)
	defer ts.Close()

	for i := 0; i < 32; i++ {
		tok, err := ts.GetToken(ctx)
		assert.NoError(t, err)
		assert.NoError(t, backend.VerifyToken(tok))
		t.Logf("STEP: %d, TOKEN: %s", i, tok)

		// tok0 initially
		if i == 0 {
			assert.Equal(t, tok0, tok)
		}

		// definitely a fresh token by step 16
		// allow some leeway due to time.Sleep concurrency
		if i > 16 {
			assert.NotEqual(t, tok0, tok)
		}

		time.Sleep(dur / 16)
	}
}

func TestTokenSourceWithQuicklyExpiringInitialToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Flaky on Windows CI workers due to the use of timer+Sleep")
	}
	t.Parallel()

	ctx := context.Background()
	dur := 20 * time.Millisecond
	backend := &testTokenBackend{tokens: map[string]time.Time{}}

	tok0, tok0Expires := backend.NewToken(dur / 10)
	ts, err := newTokenSource(ctx, tok0, tok0Expires, dur, backend.Refresh)
	assert.NoError(t, err)
	defer ts.Close()

	for i := 0; i < 8; i++ {
		tok, err := ts.GetToken(ctx)
		assert.NoError(t, err)
		assert.NoError(t, backend.VerifyToken(tok))
		t.Logf("STEP: %d, TOKEN: %s", i, tok)
		time.Sleep(dur / 16)
	}
}

type testTokenBackend struct {
	mu      sync.Mutex
	counter int
	tokens  map[string]time.Time
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
	now := time.Now()
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
	now := time.Now()
	expires, gotCurrentToken := ts.tokens[token]
	if !gotCurrentToken {
		return fmt.Errorf("Unknown token: %v", token)
	}

	if now.After(expires) {
		return fmt.Errorf("Expired token %v (%v past expiration)",
			token, now.Sub(expires))
	}
	return nil
}
