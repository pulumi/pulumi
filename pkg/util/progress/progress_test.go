// Copyright 2026, Pulumi Corporation.
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

//go:build !js

package progress

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

// syncBuffer is a bytes.Buffer safe for the renderer's goroutine to write while
// the test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// slowReader hands out a few bytes at a time, so a bar is drawn mid-download
// as well as at the end.
type slowReader struct{ remaining int }

func (r *slowReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	time.Sleep(20 * time.Millisecond)
	n := min(len(p), r.remaining, 64<<10)
	r.remaining -= n
	return n, nil
}

func newBar(t *testing.T, g *Group, message string) io.ReadCloser {
	t.Helper()
	const size = 1 << 20
	return g.wrap(io.NopCloser(&slowReader{remaining: size}), size, message, colors.Never, true)
}

func drain(t *testing.T, r io.ReadCloser) {
	t.Helper()
	_, err := io.Copy(io.Discard, r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
}

// assertQuiet fails if anything is written to out over several refresh
// intervals: output the caller prints next must land below the bars.
func assertQuiet(t *testing.T, out *syncBuffer) string {
	t.Helper()
	before := out.String()
	time.Sleep(600 * time.Millisecond)
	assert.Equal(t, before, out.String(), "progress output was written after the last bar closed")
	return before
}

func TestCloseWaitsForTheFinalFrame(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := &Group{out: &out, forceRefresh: true}

	drain(t, newBar(t, g, "Downloading provider aws"))

	assert.Contains(t, assertQuiet(t, &out), "Downloading provider aws")
}

func TestOnlyTheLastBarWaits(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := &Group{out: &out, forceRefresh: true}

	first := newBar(t, g, "Downloading provider aws")
	second := newBar(t, g, "Downloading provider random")

	drain(t, first)
	g.mu.Lock()
	renderer := g.p
	g.mu.Unlock()
	require.NotNil(t, renderer, "the renderer stopped while a bar was still active")

	drain(t, second)
	written := assertQuiet(t, &out)
	assert.Contains(t, written, "Downloading provider aws")
	assert.Contains(t, written, "Downloading provider random")
}

func TestBarsAfterADrainStillRender(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := &Group{out: &out, forceRefresh: true}

	drain(t, newBar(t, g, "Downloading provider aws"))
	drain(t, newBar(t, g, "Unpacking provider aws"))

	assert.Contains(t, assertQuiet(t, &out), "Unpacking provider aws")
}
