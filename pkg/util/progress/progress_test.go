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
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbauerster/mpb/v8"

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

// readAndClose reads r to the end and closes it, as a download would.
func readAndClose(r io.ReadCloser) error {
	_, err := io.Copy(io.Discard, r)
	return errors.Join(err, r.Close())
}

// drain reads r to the end and closes it, failing the test if that hangs.
func drain(t *testing.T, r io.ReadCloser) {
	t.Helper()
	within(t, 5*time.Second, "reading and closing the bar", func() error { return readAndClose(r) })
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

// within runs fn and fails the test if it hasn't returned after d, rather
// than letting a hang stall the whole test run. fn runs on another goroutine,
// so it reports failure by returning an error.
func within(t *testing.T, d time.Duration, what string, fn func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		require.NoError(t, err, what)
	case <-time.After(d):
		t.Fatalf("%s did not return within %v", what, d)
	}
}

func TestCloseWaitsForTheFinalFrame(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := newGroup(&out, mpb.WithAutoRefresh())

	drain(t, newBar(t, g, "Downloading provider aws"))

	assert.Contains(t, assertQuiet(t, &out), "Downloading provider aws")
}

func TestOnlyTheLastBarWaits(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := newGroup(&out, mpb.WithAutoRefresh())

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
	g := newGroup(&out, mpb.WithAutoRefresh())

	drain(t, newBar(t, g, "Downloading provider aws"))
	drain(t, newBar(t, g, "Unpacking provider aws"))

	assert.Contains(t, assertQuiet(t, &out), "Unpacking provider aws")
}

// An error part-way through a download closes the bar before it is full.
func TestCloseBeforeTheBarIsFull(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := newGroup(&out, mpb.WithAutoRefresh())

	r := newBar(t, g, "Downloading provider aws")
	_, err := io.ReadFull(r, make([]byte, 128<<10))
	require.NoError(t, err)
	within(t, 2*time.Second, "Close", r.Close)

	assertQuiet(t, &out)
}

// A bar that is never closed (an error path that skips Close) keeps the group
// from draining, as before this change, but must not block other bars.
func TestAnUnclosedBarDoesNotBlockOthers(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := newGroup(&out, mpb.WithAutoRefresh())

	leaked := newBar(t, g, "Downloading provider aws")
	other := newBar(t, g, "Downloading provider random")
	drain(t, other)

	g.mu.Lock()
	renderer := g.p
	g.mu.Unlock()
	require.NotNil(t, renderer, "the renderer stopped while a bar was still open")

	// Release the leaked bar so the test leaves no renderer running.
	within(t, 5*time.Second, "closing the leaked bar", leaked.Close)
}

// New bars can start while the previous renderer is still drawing its last
// frame; neither side may deadlock.
func TestNewBarsDuringADrain(t *testing.T) {
	t.Parallel()

	var out syncBuffer
	g := newGroup(&out, mpb.WithAutoRefresh())

	within(t, 30*time.Second, "overlapping bars", func() error {
		var wg sync.WaitGroup
		errs := make([]error, 8)
		for i := range errs {
			wg.Go(func() {
				// Stagger starts so some land while an earlier batch drains.
				time.Sleep(time.Duration(i) * 40 * time.Millisecond)
				errs[i] = readAndClose(newBar(t, g, "Downloading provider aws"))
			})
		}
		wg.Wait()
		return errors.Join(errs...)
	})

	assertQuiet(t, &out)
	g.mu.Lock()
	defer g.mu.Unlock()
	require.Zero(t, g.active)
	require.Nil(t, g.p)
}
