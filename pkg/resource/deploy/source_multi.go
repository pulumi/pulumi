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
	"errors"
	"sync/atomic"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// StackIdentity uniquely identifies a stack in a multistack deployment.
type StackIdentity struct {
	Project tokens.PackageName
	Stack   tokens.QName
}

func (si StackIdentity) String() string {
	return string(si.Stack)
}

// MultiSourceEntry represents a single source (program) in a multistack deployment.
type MultiSourceEntry struct {
	Identity StackIdentity
	Source   Source
}

// MultiSource multiplexes multiple Sources into a single Source.
// It runs N programs concurrently and merges their events.
type MultiSource struct {
	project tokens.PackageName
	entries []MultiSourceEntry
}

// NewMultiSource creates a new MultiSource that multiplexes the given entries into a single Source.
func NewMultiSource(project tokens.PackageName, entries []MultiSourceEntry) *MultiSource {
	return &MultiSource{project: project, entries: entries}
}

func (ms *MultiSource) Close() error {
	var errs []error
	for _, entry := range ms.entries {
		if err := entry.Source.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (ms *MultiSource) Project() tokens.PackageName {
	return ms.project
}

func (ms *MultiSource) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
	iterators := make([]SourceIterator, len(ms.entries))
	for i, entry := range ms.entries {
		iter, err := entry.Source.Iterate(ctx, providers)
		if err != nil {
			// Cancel any iterators we already created.
			for j := 0; j < i; j++ {
				_ = iterators[j].Cancel(ctx)
			}
			return nil, err
		}
		iterators[i] = iter
	}

	msi := &multiSourceIterator{
		entries:   ms.entries,
		iterators: iterators,
		events:    make(chan multiSourceEvent, len(ms.entries)),
		done:      make(chan struct{}),
		remaining: int32(len(ms.entries)),
	}

	// Launch a goroutine per source to pump events into the shared channel.
	for i, iter := range iterators {
		go msi.pump(ms.entries[i].Identity, iter)
	}

	return msi, nil
}

// multiSourceEvent is an internal event carrying the stack identity, the event, and any error.
type multiSourceEvent struct {
	identity StackIdentity
	event    SourceEvent
	err      error
}

// multiSourceIterator merges events from multiple source iterators.
type multiSourceIterator struct {
	entries   []MultiSourceEntry
	iterators []SourceIterator
	events    chan multiSourceEvent
	done      chan struct{}
	remaining int32 // atomic counter of active sources
}

// pump reads events from a single source iterator and sends them to the shared channel.
func (msi *multiSourceIterator) pump(identity StackIdentity, iter SourceIterator) {
	for {
		event, err := iter.Next()
		select {
		case msi.events <- multiSourceEvent{identity: identity, event: event, err: err}:
			if err != nil || event == nil {
				// Source finished (nil event) or errored; stop pumping.
				return
			}
		case <-msi.done:
			return
		}
	}
}

func (msi *multiSourceIterator) Next() (SourceEvent, error) {
	for {
		select {
		case ev := <-msi.events:
			if ev.err != nil {
				return nil, ev.err
			}
			if ev.event == nil {
				// One source finished. If all finished, we're done.
				if atomic.AddInt32(&msi.remaining, -1) == 0 {
					return nil, nil
				}
				continue
			}
			// Wrap the event with stack identity information.
			return annotateEvent(ev.identity, ev.event), nil
		case <-msi.done:
			return nil, nil
		}
	}
}

func (msi *multiSourceIterator) Cancel(ctx context.Context) error {
	close(msi.done)
	var errs []error
	for _, iter := range msi.iterators {
		if err := iter.Cancel(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// annotateEvent wraps a SourceEvent with the stack identity it originated from. For events that
// implement specific interfaces (RegisterResourceEvent, RegisterResourceOutputsEvent,
// ReadResourceEvent), a typed wrapper is returned that preserves the interface.
func annotateEvent(stack StackIdentity, event SourceEvent) SourceEvent {
	switch e := event.(type) {
	case RegisterResourceEvent:
		return &AnnotatedRegisterResourceEvent{Stack: stack, Inner: e}
	case RegisterResourceOutputsEvent:
		return &AnnotatedRegisterResourceOutputsEvent{Stack: stack, Inner: e}
	case ReadResourceEvent:
		return &AnnotatedReadResourceEvent{Stack: stack, Inner: e}
	default:
		return &AnnotatedEvent{Stack: stack, Inner: e}
	}
}

// AnnotatedEvent wraps a SourceEvent with the stack identity it came from.
type AnnotatedEvent struct {
	Stack StackIdentity
	Inner SourceEvent
}

func (a *AnnotatedEvent) event() {}

// AnnotatedRegisterResourceEvent wraps a RegisterResourceEvent with stack identity.
type AnnotatedRegisterResourceEvent struct {
	Stack StackIdentity
	Inner RegisterResourceEvent
}

var _ RegisterResourceEvent = (*AnnotatedRegisterResourceEvent)(nil)

func (a *AnnotatedRegisterResourceEvent) event()                    {}
func (a *AnnotatedRegisterResourceEvent) Goal() *resource.Goal      { return a.Inner.Goal() }
func (a *AnnotatedRegisterResourceEvent) Done(result *RegisterResult) { a.Inner.Done(result) }

// AnnotatedRegisterResourceOutputsEvent wraps a RegisterResourceOutputsEvent with stack identity.
type AnnotatedRegisterResourceOutputsEvent struct {
	Stack StackIdentity
	Inner RegisterResourceOutputsEvent
}

var _ RegisterResourceOutputsEvent = (*AnnotatedRegisterResourceOutputsEvent)(nil)

func (a *AnnotatedRegisterResourceOutputsEvent) event()                       {}
func (a *AnnotatedRegisterResourceOutputsEvent) URN() resource.URN           { return a.Inner.URN() }
func (a *AnnotatedRegisterResourceOutputsEvent) Outputs() resource.PropertyMap { return a.Inner.Outputs() }
func (a *AnnotatedRegisterResourceOutputsEvent) Done()                        { a.Inner.Done() }

// AnnotatedReadResourceEvent wraps a ReadResourceEvent with stack identity.
type AnnotatedReadResourceEvent struct {
	Stack StackIdentity
	Inner ReadResourceEvent
}

var _ ReadResourceEvent = (*AnnotatedReadResourceEvent)(nil)

func (a *AnnotatedReadResourceEvent) event()                                       {}
func (a *AnnotatedReadResourceEvent) ID() resource.ID                              { return a.Inner.ID() }
func (a *AnnotatedReadResourceEvent) Name() string                                 { return a.Inner.Name() }
func (a *AnnotatedReadResourceEvent) Type() tokens.Type                            { return a.Inner.Type() }
func (a *AnnotatedReadResourceEvent) Provider() string                             { return a.Inner.Provider() }
func (a *AnnotatedReadResourceEvent) Parent() resource.URN                         { return a.Inner.Parent() }
func (a *AnnotatedReadResourceEvent) Properties() resource.PropertyMap             { return a.Inner.Properties() }
func (a *AnnotatedReadResourceEvent) Dependencies() []resource.URN                 { return a.Inner.Dependencies() }
func (a *AnnotatedReadResourceEvent) Done(result *ReadResult)                      { a.Inner.Done(result) }
func (a *AnnotatedReadResourceEvent) AdditionalSecretOutputs() []resource.PropertyKey { return a.Inner.AdditionalSecretOutputs() }
func (a *AnnotatedReadResourceEvent) SourcePosition() string                       { return a.Inner.SourcePosition() }
func (a *AnnotatedReadResourceEvent) StackTrace() []resource.StackFrame            { return a.Inner.StackTrace() }

// GetEventStackIdentity extracts the StackIdentity from an annotated event.
// Returns a zero value and false if the event is not annotated.
func GetEventStackIdentity(event SourceEvent) (StackIdentity, bool) {
	switch e := event.(type) {
	case *AnnotatedEvent:
		return e.Stack, true
	case *AnnotatedRegisterResourceEvent:
		return e.Stack, true
	case *AnnotatedRegisterResourceOutputsEvent:
		return e.Stack, true
	case *AnnotatedReadResourceEvent:
		return e.Stack, true
	default:
		return StackIdentity{}, false
	}
}
