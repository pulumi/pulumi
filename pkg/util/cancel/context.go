// Copyright 2016-2018, Pulumi Corporation.
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

package cancel

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Context provides the ability to observe cancellation and termination requests from a Source. A termination request
// automatically triggers a corresponding cancellation request. This can be used to implement cancellation with two
// priority levels.
type Context struct {
	terminate context.Context
	cancel    context.Context
}

// Source provides the ability to deliver cancellation and termination requests to a Context. A termination request
// automatically triggers a corresponding cancellation request. This can be used to implement cancellation with two
// priority levels.
type Source struct {
	context *Context

	terminate context.CancelFunc
	cancel    context.CancelFunc
}

// NewContext creates a new cancellation context and source parented to the given context. The returned cancellation
// context will be terminated when the supplied root context is canceled.
func NewContext(ctx context.Context) (*Context, *Source) {
	contract.Require(ctx != nil, "ctx")

	// Set up two new cancellable contexts: one for termination and one for cancellation. The cancellation context is a
	// child context of the termination context and will therefore be automatically cancelled when termination is
	// requested. Both are children of the supplied context--cancelling the supplied context will cause termination.
	terminationContext, terminate := context.WithCancel(ctx)
	cancellationContext, cancel := context.WithCancel(terminationContext)

	c := &Context{
		terminate: terminationContext,
		cancel:    cancellationContext,
	}
	s := &Source{
		context:   c,
		terminate: terminate,
		cancel:    cancel,
	}
	return c, s
}

// Canceled returns a channel that will be closed when the context is canceled or terminated.
func (c *Context) Canceled() <-chan struct{} {
	return c.cancel.Done()
}

// CancelErr returns a non-nil error iff the context has been canceled or terminated.
func (c *Context) CancelErr() error {
	return c.cancel.Err()
}

// Terminated returns a channel that will be closed when the context is terminated.
func (c *Context) Terminated() <-chan struct{} {
	return c.terminate.Done()
}

// TerminateErr returns a non-nil error iff the context has been terminated.
func (c *Context) TerminateErr() error {
	return c.terminate.Err()
}

// Context returns the Context to which this source will deliver cancellation and termination requests.
func (s *Source) Context() *Context {
	return s.context
}

// Cancel cancels this source's context.
func (s *Source) Cancel() {
	s.cancel()
}

// Terminate terminates this source's context (which also cancels this context).
func (s *Source) Terminate() {
	s.terminate()
}
