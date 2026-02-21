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
	"sync"
	"sync/atomic"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// MultiSource multiplexes N Sources into a single Source. Each sub-source runs
// concurrently, with events interleaved into a single stream consumed by one Deployment.
// This is the key enabler for single-engine multistack deployment.
type MultiSource struct {
	sources []Source
}

// NewMultiSource creates a MultiSource from the given sources.
func NewMultiSource(sources []Source) *MultiSource {
	return &MultiSource{sources: sources}
}

// Project returns the project name. In multistack mode, each source embeds its own
// project in Goals, so this is only used as a fallback and returns the first source's project.
func (ms *MultiSource) Project() tokens.PackageName {
	if len(ms.sources) > 0 {
		return ms.sources[0].Project()
	}
	return ""
}

// Close closes all sub-sources.
func (ms *MultiSource) Close() error {
	var errs []error
	for _, s := range ms.sources {
		if err := s.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Iterate starts all N source iterators and returns a multiplexing iterator.
func (ms *MultiSource) Iterate(ctx context.Context, providers ProviderSource) (SourceIterator, error) {
	iterators := make([]SourceIterator, len(ms.sources))
	for i, s := range ms.sources {
		iter, err := s.Iterate(ctx, providers)
		if err != nil {
			// Cancel any iterators we already started.
			for j := 0; j < i; j++ {
				_ = iterators[j].Cancel(ctx)
			}
			return nil, err
		}
		iterators[i] = iter
	}

	msi := &multiSourceIterator{
		iterators: iterators,
		events:    make(chan multiSourceEvent, len(ms.sources)),
		remaining: int32(len(ms.sources)),
	}

	// Launch a goroutine per source to pump events into the shared channel.
	for i, iter := range iterators {
		go msi.pump(i, iter)
	}

	return msi, nil
}

// multiSourceEvent carries an event (or error) from a sub-source.
type multiSourceEvent struct {
	event SourceEvent
	err   error
}

// multiSourceIterator merges events from N sub-iterators into one stream.
type multiSourceIterator struct {
	iterators []SourceIterator
	events    chan multiSourceEvent
	remaining int32     // atomic counter of active sources
	canceled  sync.Once // protects against double-cancel
	done      chan struct{}
	doneInit  sync.Once
}

func (msi *multiSourceIterator) getDone() chan struct{} {
	msi.doneInit.Do(func() {
		msi.done = make(chan struct{})
	})
	return msi.done
}

// pump reads events from a single source iterator and sends them to the shared channel.
func (msi *multiSourceIterator) pump(idx int, iter SourceIterator) {
	for {
		event, err := iter.Next()
		if err != nil {
			logging.V(4).Infof("multiSourceIterator: source %d returned error: %v", idx, err)
			msi.events <- multiSourceEvent{err: err}
			return
		}
		if event == nil {
			// Source finished normally.
			logging.V(4).Infof("multiSourceIterator: source %d completed", idx)
			msi.events <- multiSourceEvent{} // signal completion
			return
		}

		select {
		case msi.events <- multiSourceEvent{event: event}:
		case <-msi.getDone():
			return
		}
	}
}

// Next returns the next event from any sub-source. Returns (nil, nil) when all
// sub-sources are done. Returns the first error from any sub-source immediately.
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
			return ev.event, nil
		case <-msi.getDone():
			return nil, nil
		}
	}
}

// Cancel cancels all sub-iterators.
func (msi *multiSourceIterator) Cancel(ctx context.Context) error {
	msi.canceled.Do(func() {
		close(msi.getDone())
	})
	var errs []error
	for _, iter := range msi.iterators {
		if err := iter.Cancel(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
