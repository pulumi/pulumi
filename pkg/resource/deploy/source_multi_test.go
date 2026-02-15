// Copyright 2016-2025, Pulumi Corporation.
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

package deploy

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEvent is a simple SourceEvent for testing.
type mockEvent struct {
	label string
}

func (e *mockEvent) event() {}

// mockRegisterResourceEvent implements RegisterResourceEvent for testing.
type mockRegisterResourceEvent struct {
	goal *resource.Goal
	done chan *RegisterResult
}

func (e *mockRegisterResourceEvent) event()                      {}
func (e *mockRegisterResourceEvent) Goal() *resource.Goal        { return e.goal }
func (e *mockRegisterResourceEvent) Done(result *RegisterResult) { e.done <- result }

// mockSource is a Source backed by a fixed list of events.
type mockSource struct {
	project tokens.PackageName
	events  []SourceEvent
	closed  bool
}

func (ms *mockSource) Close() error {
	ms.closed = true
	return nil
}

func (ms *mockSource) Project() tokens.PackageName {
	return ms.project
}

func (ms *mockSource) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
	return &mockSourceIterator{events: ms.events}, nil
}

// mockSourceIterator iterates over a fixed slice of events.
type mockSourceIterator struct {
	events   []SourceEvent
	index    int
	canceled bool
}

func (msi *mockSourceIterator) Cancel(ctx context.Context) error {
	msi.canceled = true
	return nil
}

func (msi *mockSourceIterator) Next() (SourceEvent, error) {
	if msi.index >= len(msi.events) {
		return nil, nil
	}
	ev := msi.events[msi.index]
	msi.index++
	return ev, nil
}

// errorSource returns an error after producing its events.
type mockErrorSource struct {
	project tokens.PackageName
	events  []SourceEvent
	err     error
}

func (es *mockErrorSource) Close() error                { return nil }
func (es *mockErrorSource) Project() tokens.PackageName { return es.project }

func (es *mockErrorSource) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
	return &mockErrorSourceIterator{events: es.events, err: es.err}, nil
}

type mockErrorSourceIterator struct {
	events []SourceEvent
	index  int
	err    error
}

func (msi *mockErrorSourceIterator) Cancel(ctx context.Context) error { return nil }

func (msi *mockErrorSourceIterator) Next() (SourceEvent, error) {
	if msi.index >= len(msi.events) {
		return nil, msi.err
	}
	ev := msi.events[msi.index]
	msi.index++
	return ev, nil
}

func TestMultiSourceBasic(t *testing.T) {
	t.Parallel()

	src1Events := []SourceEvent{&mockEvent{label: "a1"}, &mockEvent{label: "a2"}}
	src2Events := []SourceEvent{&mockEvent{label: "b1"}, &mockEvent{label: "b2"}}

	src1 := &mockSource{project: "proj-a", events: src1Events}
	src2 := &mockSource{project: "proj-b", events: src2Events}

	ms := NewMultiSource("combined", []MultiSourceEntry{
		{Identity: StackIdentity{Project: "proj-a", Stack: "stack-a"}, Source: src1},
		{Identity: StackIdentity{Project: "proj-b", Stack: "stack-b"}, Source: src2},
	})

	assert.Equal(t, tokens.PackageName("combined"), ms.Project())

	iter, err := ms.Iterate(context.Background(), nil)
	require.NoError(t, err)

	var received []SourceEvent
	for {
		ev, err := iter.Next()
		require.NoError(t, err)
		if ev == nil {
			break
		}
		received = append(received, ev)
	}

	// We should have received exactly 4 events (2 from each source).
	assert.Equal(t, 4, len(received))

	// All events should carry stack identity.
	for _, ev := range received {
		_, ok := GetEventStackIdentity(ev)
		assert.True(t, ok, "expected event to have stack identity")
	}

	require.NoError(t, ms.Close())
	assert.True(t, src1.closed)
	assert.True(t, src2.closed)
}

func TestMultiSourceOneFinishesFirst(t *testing.T) {
	t.Parallel()

	// Source 1 has 1 event, source 2 has 3.
	src1 := &mockSource{project: "proj-a", events: []SourceEvent{&mockEvent{label: "a1"}}}
	src2 := &mockSource{project: "proj-b", events: []SourceEvent{
		&mockEvent{label: "b1"}, &mockEvent{label: "b2"}, &mockEvent{label: "b3"},
	}}

	ms := NewMultiSource("combined", []MultiSourceEntry{
		{Identity: StackIdentity{Project: "proj-a", Stack: "stack-a"}, Source: src1},
		{Identity: StackIdentity{Project: "proj-b", Stack: "stack-b"}, Source: src2},
	})

	iter, err := ms.Iterate(context.Background(), nil)
	require.NoError(t, err)

	var received []SourceEvent
	for {
		ev, err := iter.Next()
		require.NoError(t, err)
		if ev == nil {
			break
		}
		received = append(received, ev)
	}

	// Total events: 1 + 3 = 4.
	assert.Equal(t, 4, len(received))

	require.NoError(t, ms.Close())
}

func TestMultiSourceCancel(t *testing.T) {
	t.Parallel()

	// Use a blocking source that waits on a channel so we can test cancellation.
	blockCh := make(chan struct{})
	src1 := &mockSource{project: "proj-a", events: []SourceEvent{&mockEvent{label: "a1"}}}
	src2 := &blockingSource{project: "proj-b", block: blockCh}

	ms := NewMultiSource("combined", []MultiSourceEntry{
		{Identity: StackIdentity{Project: "proj-a", Stack: "stack-a"}, Source: src1},
		{Identity: StackIdentity{Project: "proj-b", Stack: "stack-b"}, Source: src2},
	})

	iter, err := ms.Iterate(context.Background(), nil)
	require.NoError(t, err)

	// Cancel the iterator.
	err = iter.Cancel(context.Background())
	require.NoError(t, err)

	// After cancel, Next should return nil.
	ev, err := iter.Next()
	require.NoError(t, err)
	assert.Nil(t, ev)

	require.NoError(t, ms.Close())
}

func TestMultiSourceError(t *testing.T) {
	t.Parallel()

	expectedErr := fmt.Errorf("source failed")
	src1 := &mockErrorSource{project: "proj-a", events: []SourceEvent{&mockEvent{label: "a1"}}, err: expectedErr}
	src2 := &mockSource{project: "proj-b", events: []SourceEvent{&mockEvent{label: "b1"}}}

	ms := NewMultiSource("combined", []MultiSourceEntry{
		{Identity: StackIdentity{Project: "proj-a", Stack: "stack-a"}, Source: src1},
		{Identity: StackIdentity{Project: "proj-b", Stack: "stack-b"}, Source: src2},
	})

	iter, err := ms.Iterate(context.Background(), nil)
	require.NoError(t, err)

	// Keep reading until we see an error.
	var sawError bool
	for i := 0; i < 10; i++ {
		ev, err := iter.Next()
		if err != nil {
			assert.ErrorIs(t, err, expectedErr)
			sawError = true
			break
		}
		if ev == nil {
			break
		}
	}
	assert.True(t, sawError, "expected to see an error from the multi source")

	require.NoError(t, ms.Close())
}

func TestMultiSourceAnnotation(t *testing.T) {
	t.Parallel()

	goal := &resource.Goal{
		Type: "test:index:Resource",
		Name: "myres",
	}
	regEvent := &mockRegisterResourceEvent{goal: goal, done: make(chan *RegisterResult, 1)}
	plainEvent := &mockEvent{label: "plain"}

	src1 := &mockSource{project: "proj-a", events: []SourceEvent{regEvent}}
	src2 := &mockSource{project: "proj-b", events: []SourceEvent{plainEvent}}

	stackA := StackIdentity{Project: "proj-a", Stack: "stack-a"}
	stackB := StackIdentity{Project: "proj-b", Stack: "stack-b"}

	ms := NewMultiSource("combined", []MultiSourceEntry{
		{Identity: stackA, Source: src1},
		{Identity: stackB, Source: src2},
	})

	iter, err := ms.Iterate(context.Background(), nil)
	require.NoError(t, err)

	var received []SourceEvent
	for {
		ev, err := iter.Next()
		require.NoError(t, err)
		if ev == nil {
			break
		}
		received = append(received, ev)
	}

	assert.Equal(t, 2, len(received))

	// Verify that each event carries the correct stack identity.
	identities := make(map[tokens.QName]bool)
	for _, ev := range received {
		si, ok := GetEventStackIdentity(ev)
		require.True(t, ok)
		identities[si.Stack] = true

		// If this is a register resource event, verify the Goal is accessible.
		if rre, ok := ev.(*AnnotatedRegisterResourceEvent); ok {
			assert.Equal(t, stackA, rre.Stack)
			assert.Equal(t, goal, rre.Goal())
			// Verify Done still works.
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				rre.Done(&RegisterResult{Result: ResultStateSuccess})
			}()
			result := <-regEvent.done
			assert.Equal(t, ResultStateSuccess, result.Result)
			wg.Wait()
		}
	}

	assert.True(t, identities["stack-a"], "expected event from stack-a")
	assert.True(t, identities["stack-b"], "expected event from stack-b")

	require.NoError(t, ms.Close())
}

func TestMultiSourceStackIdentityString(t *testing.T) {
	t.Parallel()

	si := StackIdentity{Project: "myproject", Stack: "dev"}
	assert.Equal(t, "dev", si.String())
}

// blockingSource is a source whose iterator blocks until the channel is closed.
type blockingSource struct {
	project tokens.PackageName
	block   chan struct{}
}

func (bs *blockingSource) Close() error                { return nil }
func (bs *blockingSource) Project() tokens.PackageName { return bs.project }

func (bs *blockingSource) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
	return &blockingSourceIterator{block: bs.block}, nil
}

type blockingSourceIterator struct {
	block    chan struct{}
	canceled bool
}

func (bsi *blockingSourceIterator) Cancel(ctx context.Context) error {
	bsi.canceled = true
	return nil
}

func (bsi *blockingSourceIterator) Next() (SourceEvent, error) {
	<-bsi.block
	return nil, nil
}
