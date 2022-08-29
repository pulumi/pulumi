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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTokenSource(t *testing.T) {
	ctx := context.TODO()
	dur := 20 * time.Millisecond
	backend := &testTokenBackend{tokens: map[string]time.Time{}}

	tok0 := backend.NewToken(dur)
	ts, err := newTokenSource(ctx, tok0, dur, backend.Refresh)
	assert.NoError(t, err)
	defer ts.Close()

	for i := 0; i < 32; i++ {
		tok, err := ts.GetToken()
		assert.NoError(t, err) // always fresh

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

type testTokenBackend struct {
	mu      sync.Mutex
	counter int
	tokens  map[string]time.Time
}

func (ts *testTokenBackend) NewToken(duration time.Duration) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	now := time.Now()
	return ts.newTokenInner(now, duration)
}

func (ts *testTokenBackend) newTokenInner(now time.Time, duration time.Duration) string {
	ts.counter++
	tok := ts.TokenName(ts.counter)
	ts.tokens[tok] = now.Add(duration)
	return tok
}

func (ts *testTokenBackend) TokenName(refreshCount int) string {
	return fmt.Sprintf("token-%d", ts.counter)
}

func (ts *testTokenBackend) Refresh(ctx context.Context, duration time.Duration, currentToken string) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	expires, gotCurrentToken := ts.tokens[currentToken]
	if !gotCurrentToken {
		return "", fmt.Errorf("Unknown token: %v", currentToken)
	}
	now := time.Now()
	if now.After(expires) {
		return "", fmt.Errorf("Expired token: %v", currentToken)
	}
	return ts.newTokenInner(now, duration), nil
}
