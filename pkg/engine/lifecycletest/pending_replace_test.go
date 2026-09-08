// Copyright 2024, Pulumi Corporation.
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

package lifecycletest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// Tests that a delete-before-replace operation:
//
// * that is interrupted during the deletion (e.g. with a failed operation)
// * when there are resources that depend on the resource being replaced
// * and then retried, with the same original program
//
// will:
//
// * successfully replace the resource and not violate any dependencies
func TestPendingReplaceFailureDoesNotViolateSnapshotIntegrity(t *testing.T) {
	t.Parallel()

	// Arrange.
	p := &lt.TestPlan{}
	project := p.GetProject()

	diffsCalled := make(map[string]bool)
	deletesCalled := make(map[string]bool)
	createsCalled := make(map[string]bool)

	replacingADiff := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		diffsCalled[req.URN.Name()] = true
		if req.URN.Name() == "resA" {
			return plugin.DiffResult{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"key"},
				DeleteBeforeReplace: true,
			}, nil
		} else if req.URN.Name() == "resB" {
			return plugin.DiffResult{
				Changes: plugin.DiffSome,
			}, nil
		}

		return plugin.DiffResult{}, nil
	}

	throwingDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deletesCalled[req.URN.Name()] = true
		if req.URN.Name() == "resA" {
			return plugin.DeleteResponse{Status: resource.StatusUnknown}, errors.New("interrupt replace")
		}

		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	trackingDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deletesCalled[req.URN.Name()] = true
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	trackingCreateIDSuffix := "created-id"
	trackingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createsCalled[req.URN.Name()] = true
		return plugin.CreateResponse{
			ID:         resource.ID(fmt.Sprintf("%s-%s", req.URN.Name(), trackingCreateIDSuffix)),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	// Act.

	// Operation 1 -- initialise the state with two resources, one with a
	// dependency on the other.
	upLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		resA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)

		if err == nil {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{resA.URN},
			})
			require.NoError(t, err)
		} else {
			require.Fail(t, "RegisterResource should not return")
		}

		return nil
	})

	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, upLoaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, upSnap.Resources, 3)
	assert.Equal(t, "default", upSnap.Resources[0].URN.Name())
	assert.Equal(t, "resA", upSnap.Resources[1].URN.Name())
	assert.Equal(t, "resB", upSnap.Resources[2].URN.Name())

	// Operation 2 -- return a replacing diff and interrupt it with a failing
	// delete.
	replaceLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   replacingADiff,
				DeleteF: throwingDelete,
			}, nil
		}),
	}

	replaceHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, replaceLoaders...)
	replaceOptions := lt.TestUpdateOptions{T: t, HostF: replaceHostF}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), replaceOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 3)
	assert.Equal(t, "default", replaceSnap.Resources[0].URN.Name())

	assert.Equal(t, "resA", replaceSnap.Resources[1].URN.Name())
	assert.True(t, diffsCalled["resA"], "Diff should be called on resA")
	assert.True(t, deletesCalled["resA"], "Delete should be called on resA as part of replacement of resA")
	assert.False(
		t, replaceSnap.Resources[1].PendingReplacement,
		"resA should not be pending replacement following a failed deletion",
	)

	assert.Equal(t, "resB", replaceSnap.Resources[2].URN.Name())

	// Operation 3 -- attempt the same update again, with the delete not failing
	// this time. We still end up with 3 resources, but A has been replaced.
	diffsCalled = make(map[string]bool)
	deletesCalled = make(map[string]bool)
	createsCalled = make(map[string]bool)
	trackingCreateIDSuffix = "replaced-id"

	retryLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   replacingADiff,
				DeleteF: trackingDelete,
				CreateF: trackingCreate,
			}, nil
		}),
	}

	retryHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, retryLoaders...)
	retryOptions := lt.TestUpdateOptions{T: t, HostF: retryHostF}

	retrySnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), retryOptions, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	require.Len(t, retrySnap.Resources, 3)
	assert.Equal(t, "default", retrySnap.Resources[0].URN.Name())

	assert.True(t, diffsCalled["resA"], "Diff should be called on resA")
	assert.True(t, deletesCalled["resA"], "Delete should be called as part of replacement of resA")
	assert.True(t, createsCalled["resA"], "Create should be called as part of replacement of resA")
	assert.Equal(t, resource.ID("resA-replaced-id"), retrySnap.Resources[1].ID)

	assert.Equal(t, "resB", retrySnap.Resources[2].URN.Name())
	assert.True(t, diffsCalled["resB"], "Diff should be called on resB")
}

// Tests that a delete-before-replace operation:
//
// * that is interrupted after the deletion (e.g. with a failed create)
// * and then resumed, with the same original program
//
// will:
//
// * not call delete (again) on the old resource
// * remove the PendingReplacement flag from the resource in state
// * call create on the new resource
// * remove the old resource from the state
func TestPendingReplaceResumeWithSameGoals(t *testing.T) {
	t.Parallel()

	// Arrange.
	p := &lt.TestPlan{}
	project := p.GetProject()

	returnReplaceDiff := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		if req.URN.Name() == "resA" {
			return plugin.DiffResult{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"key"},
				DeleteBeforeReplace: true,
			}, nil
		}

		return plugin.DiffResult{}, nil
	}

	deleteCalled := false
	trackedDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deleteCalled = true
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	createCalled := false
	throwingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			Properties: req.Properties,
			Status:     resource.StatusUnknown,
		}, errors.New("interrupt replace")
	}

	trackedCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			ID:         "created-id",
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	// Act.

	// Operation 1 -- initialise the state with a resource.
	upLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		if expectError {
			require.Fail(t, "RegisterResource should not return")
		} else {
			require.NoError(t, err)
		}
		return nil
	})

	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, upLoaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, upSnap.Resources, 2)
	assert.Equal(t, upSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, upSnap.Resources[1].URN.Name(), "resA")

	// Operation 2 -- return a replacing diff and interrupt it with a failing
	// create.
	expectError = true
	replaceLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: throwingCreate,
			}, nil
		}),
	}

	replaceHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, replaceLoaders...)
	replaceOptions := lt.TestUpdateOptions{T: t, HostF: replaceHostF}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), replaceOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 2)
	assert.Equal(t, replaceSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, replaceSnap.Resources[1].URN.Name(), "resA")
	assert.True(t, deleteCalled, "Delete should be called as part of replacement")
	assert.True(t, replaceSnap.Resources[1].PendingReplacement)

	// Operation 3 -- resume the replacement with the same program.
	expectError = false
	deleteCalled = false
	createCalled = false

	removeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: trackedCreate,
			}, nil
		}),
	}

	removeHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, removeLoaders...)
	removeOptions := lt.TestUpdateOptions{T: t, HostF: removeHostF}

	removeSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), removeOptions, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	// Assert.
	require.Len(t, removeSnap.Resources, 2)
	assert.Equal(t, removeSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, removeSnap.Resources[1].URN.Name(), "resA")
	assert.False(t, deleteCalled, "Delete shouldn't be called a second time when resuming a replacement (same goals)")
	assert.True(t, createCalled, "Create should be called when resuming a replacement (same goals)")
	assert.False(t, removeSnap.Resources[1].PendingReplacement)
}

// Tests that a delete-before-replace operation:
//
// * that is interrupted after the deletion (e.g. with a failed create)
// * and then resumed, with the same original program, using --refresh --run-program
//
// will:
//
// * not call delete (again) on the old resource
// * remove the PendingReplacement flag from the resource in state
// * call create on the new resource
// * remove the old resource from the state
func TestPendingReplaceResumeWithSameGoalsRefreshRunProgram(t *testing.T) {
	t.Parallel()
	p := &lt.TestPlan{}
	project := p.GetProject()

	returnReplaceDiff := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		if req.URN.Name() == "resA" {
			return plugin.DiffResult{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"key"},
				DeleteBeforeReplace: true,
			}, nil
		}

		return plugin.DiffResult{}, nil
	}

	deleteCalled := false
	trackedDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deleteCalled = true
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	createCalled := false
	throwingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			Properties: req.Properties,
			Status:     resource.StatusUnknown,
		}, errors.New("interrupt replace")
	}

	trackedCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			ID:         "created-id",
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	// Operation 1 -- initialise the state with a resource.
	upLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		if expectError {
			require.Fail(t, "RegisterResource should not return")
		} else {
			require.NoError(t, err)
		}
		return nil
	})

	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, upLoaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, upSnap.Resources, 2)
	assert.Equal(t, upSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, upSnap.Resources[1].URN.Name(), "resA")

	// Operation 2 -- return a replacing diff and interrupt it with a failing
	// create.
	expectError = true
	replaceLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: throwingCreate,
			}, nil
		}),
	}

	replaceHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, replaceLoaders...)
	replaceOptions := lt.TestUpdateOptions{T: t, HostF: replaceHostF}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), replaceOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 2)
	assert.Equal(t, replaceSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, replaceSnap.Resources[1].URN.Name(), "resA")
	assert.True(t, deleteCalled, "Delete should be called as part of replacement")
	assert.True(t, replaceSnap.Resources[1].PendingReplacement)

	// Operation 3 -- resume the replacement with the same program.
	expectError = false
	deleteCalled = false
	createCalled = false

	removeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: trackedCreate,
			}, nil
		}),
	}

	removeHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, removeLoaders...)
	removeOptions := lt.TestUpdateOptions{T: t, HostF: removeHostF}

	removeOptions.Refresh = true
	removeOptions.RefreshProgram = true

	removeSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), removeOptions, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	require.Len(t, removeSnap.Resources, 2)
	assert.Equal(t, removeSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, removeSnap.Resources[1].URN.Name(), "resA")
	assert.False(t, deleteCalled, "Delete shouldn't be called a second time when resuming a replacement (same goals)")
	assert.True(t, createCalled, "Create should be called when resuming a replacement (same goals)")
	assert.False(t, removeSnap.Resources[1].PendingReplacement)
}

// Tests that a delete-before-replace operation:
//
// * that is interrupted after the deletion (e.g. with a failed create)
// * and then resumed, with a new program that removes the deleted resource
//
// will:
//
// * not call delete (again) on the old resource
// * remove the old resource from the state
func TestPendingReplaceResumeWithDeletedGoals(t *testing.T) {
	t.Parallel()

	// Arrange.
	p := &lt.TestPlan{}
	project := p.GetProject()

	returnReplaceDiff := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		if req.URN.Name() == "resA" {
			return plugin.DiffResult{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"key"},
				DeleteBeforeReplace: true,
			}, nil
		}

		return plugin.DiffResult{}, nil
	}

	deleteCalled := false
	trackedDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deleteCalled = true
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	createCalled := false
	throwingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			Properties: req.Properties,
			Status:     resource.StatusUnknown,
		}, errors.New("interrupt replace")
	}

	trackedCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			ID:         "created-id",
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	// Act.

	// Operation 1 -- initialise the state with a resource.
	upLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		if expectError {
			require.Fail(t, "RegisterResource should not return")
		} else {
			require.NoError(t, err)
		}

		return nil
	})

	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, upLoaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, upSnap.Resources, 2)
	assert.Equal(t, upSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, upSnap.Resources[1].URN.Name(), "resA")

	// Operation 2 -- return a replacing diff and interrupt it with a failing
	// create.
	expectError = true
	replaceLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: throwingCreate,
			}, nil
		}),
	}

	replaceHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, replaceLoaders...)
	replaceOptions := lt.TestUpdateOptions{T: t, HostF: replaceHostF}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), replaceOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 2)
	assert.Equal(t, replaceSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, replaceSnap.Resources[1].URN.Name(), "resA")
	assert.True(t, deleteCalled, "Delete should be called as part of replacement")
	assert.True(t, replaceSnap.Resources[1].PendingReplacement)

	// Operation 3 -- resume the replacement with a program that removes the
	// resource.
	expectError = false
	deleteCalled = false
	createCalled = false

	removeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: trackedCreate,
			}, nil
		}),
	}

	// Remove the resource from the program before resuming by providing an empty
	// program.
	removeProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		return nil
	})

	removeHostF := deploytest.NewPluginHostF(nil, nil, removeProgramF, nil, nil, removeLoaders...)
	removeOptions := lt.TestUpdateOptions{T: t, HostF: removeHostF}

	removeSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), removeOptions, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	// Assert.
	require.Len(t, removeSnap.Resources, 0)
	assert.False(t, deleteCalled, "Delete shouldn't be called a second time when resuming a replacement (deleted goals)")
	assert.False(t, createCalled, "Create shouldn't be called when resuming a replacement (deleted goals)")
}

// Tests that a delete-before-replace operation:
//
//   - that is interrupted after the deletion (e.g. with a failed create)
//   - and then resumed, with a new program that updates the deleted resource with
//     a diff that _does not_ require replacement
//
// will:
//
// * not call delete (again) on the old resource
// * remove the PendingReplacement flag from the resource in state
// * call create on the new resource with the updated goals
// * remove the old resource from the state
//
// thereby _actually_ replacing the resource despite the new diff in order to
// complete the interrupted replacement.
func TestPendingReplaceResumeWithUpdatedGoals(t *testing.T) {
	t.Parallel()

	// Arrange.
	p := &lt.TestPlan{}
	project := p.GetProject()

	returnReplaceDiff := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		if req.URN.Name() == "resA" {
			return plugin.DiffResult{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"key"},
				DeleteBeforeReplace: true,
			}, nil
		}

		return plugin.DiffResult{}, nil
	}

	returnNonReplaceDiff := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		if req.URN.Name() == "resA" {
			return plugin.DiffResult{
				Changes:             plugin.DiffSome,
				DeleteBeforeReplace: true,
			}, nil
		}

		return plugin.DiffResult{}, nil
	}

	deleteCalled := false
	trackedDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deleteCalled = true
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	createCalled := false
	throwingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			Properties: req.Properties,
			Status:     resource.StatusUnknown,
		}, errors.New("interrupt replace")
	}

	trackedCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createCalled = true
		return plugin.CreateResponse{
			ID:         "created-id",
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	// Act.

	// Operation 1 -- initialise the state with a resource.
	upLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		if expectError {
			require.Fail(t, "RegisterResource should not return")
		} else {
			require.NoError(t, err)
		}
		return nil
	})

	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, upLoaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, upSnap.Resources, 2)
	assert.Equal(t, upSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, upSnap.Resources[1].URN.Name(), "resA")

	// Operation 2 -- return a replacing diff and interrupt it with a failing
	// create.
	expectError = true
	replaceLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: throwingCreate,
			}, nil
		}),
	}

	replaceHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, replaceLoaders...)
	replaceOptions := lt.TestUpdateOptions{T: t, HostF: replaceHostF}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), replaceOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 2)
	assert.Equal(t, replaceSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, replaceSnap.Resources[1].URN.Name(), "resA")
	assert.True(t, deleteCalled, "Delete should be called as part of replacement")
	assert.True(t, replaceSnap.Resources[1].PendingReplacement)

	// Operation 3 -- resume the replacement with a program that triggers a
	// non-replacing diff (for the purposes of the test we do this by mocking the
	// Diff call rather than updating the program, but the effect should be the
	// same).
	expectError = false
	deleteCalled = false
	createCalled = false

	removeLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   returnNonReplaceDiff,
				DeleteF: trackedDelete,
				CreateF: trackedCreate,
			}, nil
		}),
	}

	removeHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, removeLoaders...)
	removeOptions := lt.TestUpdateOptions{T: t, HostF: removeHostF}

	removeSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), removeOptions, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	// Assert.
	require.Len(t, removeSnap.Resources, 2)
	assert.Equal(t, removeSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, removeSnap.Resources[1].URN.Name(), "resA")
	assert.False(t, deleteCalled, "Delete shouldn't be called a second time when resuming a replacement (updated goals)")
	assert.True(t, createCalled, "Create should be called when resuming a replacement (updated goals)")
	assert.False(t, removeSnap.Resources[1].PendingReplacement)
}

// Regression test for https://github.com/pulumi/pulumi/issues/17111
func TestInteruptedPendingReplace(t *testing.T) {
	t.Parallel()

	// Arrange.
	p := &lt.TestPlan{}
	project := p.GetProject()

	// Act.

	// Operation 1 -- initialise the state with two resources.
	provider := func() plugin.Provider { return &deploytest.Provider{} }
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return provider(), nil
		}),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dbr := true
		a, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			DeleteBeforeReplace: &dbr,
		})

		if err == nil {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Dependencies: []resource.URN{a.URN},
			})
			require.NoError(t, err)
		} else {
			require.Fail(t, "RegisterResource should not return")
		}

		return nil
	})

	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)

	require.Len(t, upSnap.Resources, 3)
	assert.Equal(t, upSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, upSnap.Resources[1].URN.Name(), "resA")
	assert.Equal(t, upSnap.Resources[2].URN.Name(), "resB")

	// Operation 2 -- return a diff and interrupt it with a failing
	// create.
	provider = func() plugin.Provider {
		return &deploytest.Provider{
			DiffF: func(ctx context.Context, dr plugin.DiffRequest) (plugin.DiffResult, error) {
				if dr.URN.Name() == "resA" {
					return plugin.DiffResult{
						Changes:     plugin.DiffSome,
						ReplaceKeys: []resource.PropertyKey{"key"},
					}, nil
				}
				return plugin.DiffResult{
					Changes: plugin.DiffNone,
				}, nil
			},
			CreateF: func(ctx context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
				if cr.URN.Name() == "resA" {
					return plugin.CreateResponse{}, errors.New("interrupt replace")
				}
				return plugin.CreateResponse{}, nil
			},
		}
	}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), upOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 3)
	assert.Equal(t, replaceSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, replaceSnap.Resources[1].URN.Name(), "resA")
	assert.True(t, replaceSnap.Resources[1].PendingReplacement)
	assert.Equal(t, replaceSnap.Resources[2].URN.Name(), "resB")

	// Operation 3 -- try and resume the replacement with the same program, but fail the create again.

	secondReplaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), upOptions, false, p.BackendClient, nil, "2")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, secondReplaceSnap.Resources, 3)
	assert.Equal(t, secondReplaceSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, secondReplaceSnap.Resources[1].URN.Name(), "resA")
	assert.True(t, secondReplaceSnap.Resources[1].PendingReplacement)
	assert.Equal(t, secondReplaceSnap.Resources[2].URN.Name(), "resB")

	// Operation 4 -- try and resume the replacement with the same program, and let the create succeed.

	provider = func() plugin.Provider {
		return &deploytest.Provider{
			DiffF: func(ctx context.Context, dr plugin.DiffRequest) (plugin.DiffResult, error) {
				if dr.URN.Name() == "resA" {
					return plugin.DiffResult{
						Changes:     plugin.DiffSome,
						ReplaceKeys: []resource.PropertyKey{"key"},
					}, nil
				}
				return plugin.DiffResult{
					Changes: plugin.DiffNone,
				}, nil
			},
			CreateF: func(ctx context.Context, cr plugin.CreateRequest) (plugin.CreateResponse, error) {
				return plugin.CreateResponse{
					ID:         "created-id",
					Properties: cr.Properties,
				}, nil
			},
		}
	}

	secondUpSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, secondReplaceSnap), upOptions, false, p.BackendClient, nil, "3")
	require.NoError(t, err)

	require.Len(t, secondUpSnap.Resources, 3)
	assert.Equal(t, secondUpSnap.Resources[0].URN.Name(), "default")
	assert.Equal(t, secondUpSnap.Resources[1].URN.Name(), "resA")
	assert.False(t, secondUpSnap.Resources[1].PendingReplacement)
	assert.Equal(t, secondUpSnap.Resources[2].URN.Name(), "resB")
}

func TestPendingReplaceDependentDeleteNotRetried(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	replaceOnAChanged := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
			return plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ReplaceKeys: []resource.PropertyKey{"A"},
			}, nil
		}
		return plugin.DiffResult{}, nil
	}

	deletesCalled := make(map[string]bool)
	createsCalled := make(map[string]bool)

	throwingDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deletesCalled[req.URN.Name()] = true
		if req.URN.Name() == "resA" {
			return plugin.DeleteResponse{Status: resource.StatusUnknown}, errors.New("interrupt replace")
		}
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	trackingDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deletesCalled[req.URN.Name()] = true
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	trackingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createsCalled[req.URN.Name()] = true
		return plugin.CreateResponse{
			ID:         resource.ID(req.URN.Name() + "-created-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	inputsA := resource.NewPropertyMapFromMap(map[string]any{"A": "foo"})
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dbr := true
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:              inputsA,
			DeleteBeforeReplace: &dbr,
		})
		if err == nil {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
				Inputs:       resource.NewPropertyMapFromMap(map[string]any{"A": "value"}),
				Dependencies: []resource.URN{respA.URN},
				PropertyDeps: map[resource.PropertyKey][]resource.URN{"A": {respA.URN}},
			})
			require.NoError(t, err)
		} else {
			require.Fail(t, "RegisterResource should not return")
		}
		return nil
	})

	// Operation 1 -- initialise the state with two resources, one whose input property depends on the
	// other.
	upLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, upLoaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Len(t, upSnap.Resources, 3)

	// Operation 2 -- change resA so that both it and its dependent resB are replaced, and interrupt the
	// operation by failing resA's delete after resB's delete has succeeded.
	inputsA["A"] = resource.NewProperty("bar")

	replaceLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   replaceOnAChanged,
				DeleteF: throwingDelete,
			}, nil
		}),
	}
	replaceHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, replaceLoaders...)
	replaceOptions := lt.TestUpdateOptions{T: t, HostF: replaceHostF}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), replaceOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 3)
	assert.Equal(t, "resA", replaceSnap.Resources[1].URN.Name())
	assert.False(t, replaceSnap.Resources[1].PendingReplacement)
	assert.Equal(t, "resB", replaceSnap.Resources[2].URN.Name())
	assert.True(t, replaceSnap.Resources[2].PendingReplacement)
	assert.True(t, deletesCalled["resB"], "Delete should be called on resB as part of replacement of resA")

	// Operation 3 -- retry with the same program. resB's delete must not be retried since it is
	// already pending replacement.
	deletesCalled = make(map[string]bool)
	createsCalled = make(map[string]bool)

	retryLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   replaceOnAChanged,
				DeleteF: trackingDelete,
				CreateF: trackingCreate,
			}, nil
		}),
	}
	retryHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, retryLoaders...)
	retryOptions := lt.TestUpdateOptions{T: t, HostF: retryHostF}

	retrySnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), retryOptions, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	assert.True(t, deletesCalled["resA"], "Delete should be called on resA when the replacement is retried")
	assert.False(t, deletesCalled["resB"], "Delete shouldn't be called again on resB, which is pending replacement")
	assert.True(t, createsCalled["resA"], "Create should be called on resA when the replacement is retried")
	assert.True(t, createsCalled["resB"], "Create should be called on resB when the replacement is retried")

	require.Len(t, retrySnap.Resources, 3)
	assert.Equal(t, "resA", retrySnap.Resources[1].URN.Name())
	assert.False(t, retrySnap.Resources[1].PendingReplacement)
	assert.Equal(t, "resB", retrySnap.Resources[2].URN.Name())
	assert.False(t, retrySnap.Resources[2].PendingReplacement)
}

func TestPendingReplaceDependentResumeAfterReplacement(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	project := p.GetProject()

	replaceOnAChanged := func(
		_ context.Context,
		req plugin.DiffRequest,
	) (plugin.DiffResult, error) {
		if !req.OldOutputs["A"].DeepEquals(req.NewInputs["A"]) {
			return plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ReplaceKeys: []resource.PropertyKey{"A"},
			}, nil
		}
		return plugin.DiffResult{}, nil
	}

	deletesCalled := make(map[string]bool)
	createsCalled := make(map[string]bool)

	trackingDelete := func(
		_ context.Context,
		req plugin.DeleteRequest,
	) (plugin.DeleteResponse, error) {
		deletesCalled[req.URN.Name()] = true
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}

	throwingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createsCalled[req.URN.Name()] = true
		if req.URN.Name() == "resB" {
			return plugin.CreateResponse{Status: resource.StatusUnknown}, errors.New("interrupt replace")
		}
		return plugin.CreateResponse{
			ID:         resource.ID(req.URN.Name() + "-created-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	trackingCreate := func(
		_ context.Context,
		req plugin.CreateRequest,
	) (plugin.CreateResponse, error) {
		createsCalled[req.URN.Name()] = true
		return plugin.CreateResponse{
			ID:         resource.ID(req.URN.Name() + "-created-id"),
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}

	inputsA := resource.NewPropertyMapFromMap(map[string]any{"A": "foo"})
	expectError := false
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		dbr := true
		respA, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:              inputsA,
			DeleteBeforeReplace: &dbr,
		})
		require.NoError(t, err)
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, deploytest.ResourceOptions{
			Inputs:       resource.NewPropertyMapFromMap(map[string]any{"A": "value"}),
			Dependencies: []resource.URN{respA.URN},
			PropertyDeps: map[resource.PropertyKey][]resource.URN{"A": {respA.URN}},
		})
		if expectError {
			require.Fail(t, "RegisterResource should not return")
		} else {
			require.NoError(t, err)
		}
		return nil
	})

	// Operation 1 -- initialise the state with two resources, one whose input property depends on the
	// other.
	upLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	upHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, upLoaders...)
	upOptions := lt.TestUpdateOptions{T: t, HostF: upHostF}

	upSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, nil), upOptions, false, p.BackendClient, nil, "0")
	require.NoError(t, err)
	require.Len(t, upSnap.Resources, 3)

	// Operation 2 -- change resA so that both it and its dependent resB are replaced, and interrupt the
	// operation by failing resB's create after resA's replacement has completed.
	inputsA["A"] = resource.NewProperty("bar")
	expectError = true

	replaceLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   replaceOnAChanged,
				DeleteF: trackingDelete,
				CreateF: throwingCreate,
			}, nil
		}),
	}
	replaceHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, replaceLoaders...)
	replaceOptions := lt.TestUpdateOptions{T: t, HostF: replaceHostF}

	replaceSnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, upSnap), replaceOptions, false, p.BackendClient, nil, "1")
	assert.ErrorContains(t, err, "interrupt replace")

	require.Len(t, replaceSnap.Resources, 3)
	assert.Equal(t, "resA", replaceSnap.Resources[1].URN.Name())
	assert.Equal(t, resource.ID("resA-created-id"), replaceSnap.Resources[1].ID)
	assert.False(t, replaceSnap.Resources[1].PendingReplacement)
	assert.Equal(t, "resB", replaceSnap.Resources[2].URN.Name())
	assert.True(t, replaceSnap.Resources[2].PendingReplacement)

	// Operation 3 -- retry with the same program. resB's delete must not be retried since it is
	// already pending replacement; only its replacement needs to be created.
	expectError = false
	deletesCalled = make(map[string]bool)
	createsCalled = make(map[string]bool)

	retryLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF:   replaceOnAChanged,
				DeleteF: trackingDelete,
				CreateF: trackingCreate,
			}, nil
		}),
	}
	retryHostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, retryLoaders...)
	retryOptions := lt.TestUpdateOptions{T: t, HostF: retryHostF}

	retrySnap, err := lt.TestOp(Update).
		RunStep(project, p.GetTarget(t, replaceSnap), retryOptions, false, p.BackendClient, nil, "2")
	require.NoError(t, err)

	assert.False(t, deletesCalled["resA"], "Delete shouldn't be called on resA, which was already replaced")
	assert.False(t, deletesCalled["resB"], "Delete shouldn't be called again on resB, which is pending replacement")
	assert.False(t, createsCalled["resA"], "Create shouldn't be called on resA, which was already replaced")
	assert.True(t, createsCalled["resB"], "Create should be called on resB when the replacement is resumed")

	require.Len(t, retrySnap.Resources, 3)
	assert.Equal(t, "resA", retrySnap.Resources[1].URN.Name())
	assert.False(t, retrySnap.Resources[1].PendingReplacement)
	assert.Equal(t, "resB", retrySnap.Resources[2].URN.Name())
	assert.False(t, retrySnap.Resources[2].PendingReplacement)
	assert.False(t, retrySnap.Resources[2].Delete)
}
