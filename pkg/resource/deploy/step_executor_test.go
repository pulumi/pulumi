// Copyright 2016-2023, Pulumi Corporation.
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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
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
	OnResourceStepPreF   func(step Step) (interface{}, error)
	OnResourceStepPostF  func(ctx interface{}, step Step, status resource.Status, err error) error
	OnResourceOutputsF   func(step Step) error
	OnPolicyViolationF   func(resource.URN, plugin.AnalyzeDiagnostic)
	OnPolicyRemediationF func(resource.URN, plugin.Remediation, resource.PropertyMap, resource.PropertyMap)
}

func (e *mockEvents) OnResourceStepPre(step Step) (interface{}, error) {
	if e.OnResourceStepPreF != nil {
		return e.OnResourceStepPreF(step)
	}
	panic("unimplemented")
}

func (e *mockEvents) OnResourceStepPost(ctx interface{}, step Step, status resource.Status, err error) error {
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

func (e *mockEvents) OnPolicyRemediation(resource.URN, plugin.Remediation, resource.PropertyMap, resource.PropertyMap) {
	panic("unimplemented")
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
			se.pendingNews.Store(notInPlan, &CreateStep{new: &resource.State{}})
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
			se.pendingNews.Store(notInPlan, &CreateStep{new: &resource.State{}})
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
			se.pendingNews.Store(notInPlan, &CreateStep{new: &resource.State{}})
			// Does not error.
			assert.NoError(t, se.ExecuteRegisterResourceOutputs(&registerResourceOutputsEvent{
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
						OnResourceStepPreF: func(step Step) (interface{}, error) {
							return nil, expectedErr
						},
					},
				},
				pendingNews: gsync.Map[resource.URN, Step]{},
			}
			se.pendingNews.Store(resource.URN("not-in-plan"), &CreateStep{new: &resource.State{}})
			assert.ErrorIs(t, se.executeStep(0, &CreateStep{
				new: &resource.State{URN: "some-urn"},
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
						OnResourceStepPreF: func(step Step) (interface{}, error) {
							return nil, nil
						},
						OnResourceStepPostF: func(
							ctx interface{}, step Step, status resource.Status, err error,
						) error {
							return expectedErr
						},
					},
					goals: &gsync.Map[resource.URN, *resource.Goal]{},
				},
				pendingNews: gsync.Map[resource.URN, Step]{},
			}
			step := &CreateStep{
				new: &resource.State{
					URN: "some-urn",
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
