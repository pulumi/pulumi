// Copyright 2016, Pulumi Corporation.
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
	"errors"
	"runtime"
	"sync"
	"testing"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterResourceErrorsOnMissingPendingNew(t *testing.T) {
	t.Parallel()

	se := &stepExecutor{
		pendingNews: gsync.Map[resource.URN, Step]{},
	}
	urn := resource.URN("urn:pulumi:stack::project::my:example:Foo::foo")
	err := se.ExecuteRegisterResourceOutputs(&mockRegisterResourceOutputsEvent{
		urn: urn,
	})
	// Should error, but not panic since the resource is being registered twice.
	assert.Error(t, err)
}

type stepGate struct {
	entered chan struct{}
	finish  chan struct{}
}

func newStepGate() *stepGate {
	return &stepGate{entered: make(chan struct{}), finish: make(chan struct{})}
}

func (g *stepGate) enter() {
	close(g.entered)
	<-g.finish
}

func (g *stepGate) waitUntilEntered() { <-g.entered }

func (g *stepGate) release() { close(g.finish) }

type pendingWriter struct {
	lock     *sync.RWMutex
	acquired chan struct{}
	release  chan struct{}
	done     chan struct{}
}

func (w *pendingWriter) start() {
	w.acquired = make(chan struct{})
	w.release = make(chan struct{})
	w.done = make(chan struct{})
	go func() {
		w.lock.Lock()
		close(w.acquired)
		<-w.release
		w.lock.Unlock()
		close(w.done)
	}()
}

func (w *pendingWriter) waitUntilPending() {
	// Go's RWMutex blocks new readers once a writer is waiting. TryRLock therefore
	// distinguishes a pending writer from the reader already held by the chain.
	for w.lock.TryRLock() {
		w.lock.RUnlock()
		runtime.Gosched()
	}
}

func (w *pendingWriter) assertNotAcquired(t *testing.T) {
	t.Helper()

	select {
	case <-w.acquired:
		t.Fatal("writer unexpectedly acquired the executor lock")
	default:
	}
}

func (w *pendingWriter) finish(t *testing.T) {
	t.Helper()

	<-w.acquired
	close(w.release)
	<-w.done
}

func TestStepExecutorWriterCannotOvertakeQueuedChain(t *testing.T) {
	t.Parallel()

	executor := &stepExecutor{
		incomingChains: make(chan incomingChain, 1),
		ctx:            t.Context(),
	}

	// The chain has been accepted but no worker has picked it up.
	executor.ExecuteSerial(chain{})
	<-executor.incomingChains

	if executor.workerLock.TryLock() {
		executor.workerLock.Unlock()
		t.Fatal("writer overtook a queued chain")
	}
	// No worker exists in this test, so release the reservation it would own.
	executor.workerLock.RUnlock()
}

func TestStepExecutorReservationSpansStepsWithinChain(t *testing.T) {
	t.Parallel()

	firstURN := resource.URN("urn:pulumi:stack::project::test:index:Resource::first")
	secondURN := resource.URN("urn:pulumi:stack::project::test:index:Resource::second")
	first, second := newStepGate(), newStepGate()
	gates := map[resource.URN]*stepGate{
		firstURN:  first,
		secondURN: second,
	}
	deployment := &Deployment{
		opts: &Options{},
		events: &mockEvents{
			OnResourceStepPreF: func(step Step) (any, error) {
				gates[step.URN()].enter()
				return nil, nil
			},
			OnResourceStepPostF: func(any, Step, resource.Status, error) error {
				return nil
			},
		},
		news: &gsync.Map[resource.URN, *pkgresource.State]{},
	}
	executor := &stepExecutor{
		deployment:     deployment,
		incomingChains: make(chan incomingChain, 1),
		ctx:            t.Context(),
	}

	newSameStep := func(urn resource.URN) Step {
		return &SameStep{
			deployment: deployment,
			old:        &pkgresource.State{URN: urn},
			new:        &pkgresource.State{URN: urn},
		}
	}

	token := executor.ExecuteSerial(chain{
		newSameStep(firstURN),
		newSameStep(secondURN),
	})
	request := <-executor.incomingChains

	go func() {
		executor.executeChain(0, request.Chain)
		close(request.CompletionChan)
	}()

	// Step 1 is running with the chain reservation.
	first.waitUntilEntered()

	// Queue an exclusive writer while step 1 is still blocked.
	writer := &pendingWriter{lock: &executor.workerLock}
	writer.start()
	writer.waitUntilPending()

	// Let step 1 finish. Since the lock is for the whole chain, the writer should not acquire it.
	first.release()

	select {
	case <-second.entered:
		writer.assertNotAcquired(t)
	case <-writer.acquired:
		writer.finish(t)
		t.Fatal("writer acquired the executor lock between steps in the same chain")
	}
	second.release()

	token.Wait(t.Context())
	writer.finish(t)
}

type mockRegisterResourceOutputsEvent struct {
	urn resource.URN
}

var _ = RegisterResourceOutputsEvent((*mockRegisterResourceOutputsEvent)(nil))

func (e *mockRegisterResourceOutputsEvent) event() {}

func (e *mockRegisterResourceOutputsEvent) URN() resource.URN { return e.urn }

func (e *mockRegisterResourceOutputsEvent) Outputs() resource.PropertyMap {
	return resource.PropertyMap{}
}

func (e *mockRegisterResourceOutputsEvent) Done() {}

type mockEvents struct {
	OnResourceStepPreF   func(step Step) (any, error)
	OnResourceStepPostF  func(ctx any, step Step, status resource.Status, err error) error
	OnResourceOutputsF   func(step Step) error
	OnPolicyViolationF   func(resource.URN, plugin.AnalyzeDiagnostic)
	OnPolicyRemediationF func(resource.URN, plugin.Remediation, resource.PropertyMap, resource.PropertyMap)
}

func (e *mockEvents) OnResourceStepPre(step Step) (any, error) {
	if e.OnResourceStepPreF != nil {
		return e.OnResourceStepPreF(step)
	}
	panic("unimplemented")
}

func (e *mockEvents) OnResourceStepPost(ctx any, step Step, status resource.Status, err error) error {
	if e.OnResourceStepPostF != nil {
		return e.OnResourceStepPostF(ctx, step, status, err)
	}
	panic("unimplemented")
}

func (e *mockEvents) OnResourceOutputs(step Step) error {
	if e.OnResourceOutputsF != nil {
		return e.OnResourceOutputsF(step)
	}
	panic("unimplemented")
}

func (e *mockEvents) OnPolicyViolation(resource.URN, plugin.AnalyzeDiagnostic) {
	panic("unimplemented")
}

func (e *mockEvents) OnPolicyRemediation(resource.URN, plugin.Remediation, property.Map, property.Map) {
	panic("unimplemented")
}

func (e *mockEvents) OnPolicyAnalyzeSummary(plugin.PolicySummary) {
	panic("unimplemented")
}

func (e *mockEvents) OnPolicyRemediateSummary(plugin.PolicySummary) {
	panic("unimplemented")
}

func (e *mockEvents) OnPolicyAnalyzeStackSummary(plugin.PolicySummary) {
	panic("unimplemented")
}

func (e *mockEvents) OnSnapshotWrite(base *Snapshot) error {
	return nil
}

func (e *mockEvents) OnRebuiltBaseState() error {
	return nil
}

var _ Events = (*mockEvents)(nil)

func TestStepExecutor(t *testing.T) {
	t.Parallel()
	t.Run("ExecuteRegisterResourceOutputs", func(t *testing.T) {
		t.Parallel()
		t.Run("no plan for resource", func(t *testing.T) {
			t.Parallel()

			se := &stepExecutor{
				deployment: &Deployment{
					opts: &Options{},
					plan: &Plan{},
				},
				pendingNews: gsync.Map[resource.URN, Step]{},
			}
			notInPlan := resource.NewURN("test", "test", "", "test", "not-in-plan")
			se.pendingNews.Store(notInPlan, &CreateStep{new: &pkgresource.State{}})
			assert.ErrorContains(t, se.ExecuteRegisterResourceOutputs(&registerResourceOutputsEvent{
				urn: notInPlan,
			}), "no plan for resource")
		})
		t.Run("resource should already have a plan", func(t *testing.T) {
			t.Parallel()

			se := &stepExecutor{
				deployment: &Deployment{
					opts: &Options{
						GeneratePlan: true,
					},
					newPlans: &resourcePlans{},
				},
				pendingNews: gsync.Map[resource.URN, Step]{},
			}
			notInPlan := resource.NewURN("test", "test", "", "test", "not-in-plan")
			se.pendingNews.Store(notInPlan, &CreateStep{new: &pkgresource.State{}})
			assert.ErrorContains(t, se.ExecuteRegisterResourceOutputs(&registerResourceOutputsEvent{
				urn: notInPlan,
			}), "resource should already have a plan")
		})
		t.Run("error in resource outputs", func(t *testing.T) {
			t.Parallel()

			var cancelCalled bool
			se := &stepExecutor{
				cancel: func() {
					cancelCalled = true
				},
				deployment: &Deployment{
					ctx: &plugin.Context{
						Diag: &deploytest.NoopSink{},
					},
					opts: &Options{},
					events: &mockEvents{
						OnResourceOutputsF: func(step Step) error {
							return errors.New("expected error")
						},
					},
				},
				pendingNews: gsync.Map[resource.URN, Step]{},
			}
			notInPlan := resource.NewURN("test", "test", "", "test", "not-in-plan")
			se.pendingNews.Store(notInPlan, &CreateStep{new: &pkgresource.State{
				URN: "urn:pulumi:some-urn",
			}})
			// Does not error.
			require.NoError(t, se.ExecuteRegisterResourceOutputs(&registerResourceOutputsEvent{
				urn: notInPlan,
			}))
			assert.True(t, cancelCalled)
		})
	})
	t.Run("executeStep", func(t *testing.T) {
		t.Run("error in onResourceStepPre", func(t *testing.T) {
			t.Parallel()

			expectedErr := errors.New("expected error")
			se := &stepExecutor{
				deployment: &Deployment{
					ctx: &plugin.Context{
						Diag: &deploytest.NoopSink{},
					},
					opts: &Options{},
					events: &mockEvents{
						OnResourceStepPreF: func(step Step) (any, error) {
							return nil, expectedErr
						},
					},
				},
				pendingNews: gsync.Map[resource.URN, Step]{},
			}
			se.pendingNews.Store(resource.URN("not-in-plan"), &CreateStep{new: &pkgresource.State{}})
			assert.ErrorIs(t, se.executeStep(0, &CreateStep{
				new: &pkgresource.State{URN: "urn:pulumi:some-urn"},
			}), expectedErr)
		})
		t.Run("disallow mark id secret", func(t *testing.T) {
			t.Parallel()

			expectedErr := errors.New("expected error")
			se := &stepExecutor{
				deployment: &Deployment{
					ctx: &plugin.Context{
						Diag: &deploytest.NoopSink{},
					},
					opts: &Options{},
					events: &mockEvents{
						OnResourceStepPreF: func(step Step) (any, error) {
							return nil, nil
						},
						OnResourceStepPostF: func(
							ctx any, step Step, status resource.Status, err error,
						) error {
							return expectedErr
						},
					},
					goals: &gsync.Map[resource.URN, *pkgresource.Goal]{},
					news:  &gsync.Map[resource.URN, *pkgresource.State]{},
				},
				pendingNews: gsync.Map[resource.URN, Step]{},
			}
			step := &CreateStep{
				new: &pkgresource.State{
					URN: "urn:pulumi:some-urn",
					AdditionalSecretOutputs: []resource.PropertyKey{
						"id",
						"non-existent-property",
					},
				},
				provider: &deploytest.Provider{},
			}
			assert.ErrorContains(t, se.executeStep(0, step), "post-step event returned an error")
		})
	})
}
