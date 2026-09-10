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

package deploy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type staticReferenceLoader struct {
	pkg *schema.Package
}

func (l staticReferenceLoader) LoadPackage(string, *semver.Version) (*schema.Package, error) {
	return l.pkg, nil
}

func (l staticReferenceLoader) LoadPackageV2(context.Context, *schema.PackageDescriptor) (*schema.Package, error) {
	return l.pkg, nil
}

func (l staticReferenceLoader) LoadPackageReference(string, *semver.Version) (schema.PackageReference, error) {
	return l.pkg.Reference(), nil
}

func (l staticReferenceLoader) LoadPackageReferenceV2(
	context.Context, *schema.PackageDescriptor,
) (schema.PackageReference, error) {
	return l.pkg.Reference(), nil
}

// fakeSource turns a CompletionSource into the source-runner shape NewMuxSource expects, so tests can drive
// completion timing precisely.
func fakeSource(cs *promise.CompletionSource[struct{}]) func(string) *promise.Promise[struct{}] {
	return func(string) *promise.Promise[struct{}] {
		return cs.Promise()
	}
}

func TestSnippetRunInfo(t *testing.T) {
	t.Parallel()

	descriptors := map[string]*schema.PackageDescriptor{
		"pkg": {Name: "pkg"},
	}
	info := &pulumirpc.DeploymentInfo{
		Project:          "project",
		Stack:            "stack",
		Organization:     "organization",
		Config:           map[string]string{"project:key": "value"},
		ConfigSecretKeys: []string{"project:secret"},
		DryRun:           true,
		Parallel:         42,
	}

	runInfo := snippetRunInfo(info, "/root", "/work", descriptors)

	require.Equal(t, info.Project, runInfo.Project)
	require.Equal(t, info.Stack, runInfo.Stack)
	require.Equal(t, info.Organization, runInfo.Organization)
	require.Equal(t, "/root", runInfo.RootDirectory)
	require.Equal(t, "/work", runInfo.WorkingDir)
	require.Equal(t, info.Config, runInfo.Config)
	require.Equal(t, info.ConfigSecretKeys, runInfo.ConfigSecrets)
	require.True(t, runInfo.DryRun)
	require.Equal(t, info.Parallel, runInfo.Parallel)
	require.Equal(t, descriptors, runInfo.PackageDescriptors)
}

func TestSnippetReferenceValue(t *testing.T) {
	t.Parallel()

	urn := resource.URN("urn:pulumi:stack::project::pkg:index:parent$pkg:index:res::name")
	value := snippetReferenceValue(urn, URNRegistration{
		ID: "id",
		Outputs: resource.PropertyMap{
			"output": resource.NewProperty("value"),
		},
	})

	output := value.OutputValue()
	require.True(t, output.Known)
	require.Equal(t, []resource.URN{urn}, output.Dependencies)
	require.Equal(t, resource.PropertyMap{
		"output": resource.NewProperty("value"),
		"urn":    resource.NewProperty(string(urn)),
		"id":     resource.NewProperty("id"),
		"__name": resource.NewProperty("name"),
		"__type": resource.NewProperty("pkg:index:res"),
	}, output.Element.ObjectValue())
}

func TestSnippetSourceRequiresObserver(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		NewSnippetSource(t.Context(), resource.Snippet{}, nil, "", "", nil)
	})
}

func TestSnippetSource_CancellationStopsReferenceWait(t *testing.T) {
	t.Parallel()

	pkg, err := schema.ImportSpec(schema.PackageSpec{
		Name: "pkg",
		Resources: map[string]schema.ResourceSpec{
			"pkg:index:res": {
				InputProperties: map[string]schema.PropertySpec{
					"message": {TypeSpec: schema.TypeSpec{Type: "string"}},
				},
				RequiredInputs: []string{"message"},
			},
		},
	}, nil, schema.NewNullLoader(), schema.ValidationOptions{})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	observer := NewRegistrationObserver()
	producer := observer.NewSource()
	source := NewSnippetSource(ctx, resource.Snippet{
		Name:       "consumer",
		Type:       "pkg:index:res",
		Descriptor: resource.PackageDescriptor{Name: "pkg"},
		References: map[string]string{
			"producer": "urn:pulumi:stack::project::pkg:index:res::producer",
		},
		Code: "message = producer.message",
	}, staticReferenceLoader{pkg: pkg}, "", "", observer)
	observer.SourcesReady()

	result := source("")
	require.Eventually(t, func() bool {
		observer.mu.Lock()
		defer observer.mu.Unlock()
		for _, waitingFor := range observer.sources {
			if waitingFor != "" {
				return true
			}
		}
		return false
	}, time.Second, 10*time.Millisecond)

	cancel()
	_, err = result.Result(t.Context())
	require.ErrorIs(t, err, context.Canceled)
	producer.Done()
}

func TestMuxSource_AllSucceed(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}
	bCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS), fakeSource(bCS))
	out := mux("ignored")

	// Fulfill in arbitrary order.
	aCS.Fulfill(struct{}{})
	mainCS.Fulfill(struct{}{})
	bCS.Fulfill(struct{}{})

	_, err := out.Result(t.Context())
	require.NoError(t, err)
}

func TestMuxSource_SingleErrorPropagatesAsIs(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS))
	out := mux("ignored")

	bang := errors.New("snippet exploded")
	aCS.Reject(bang)
	mainCS.Fulfill(struct{}{})

	_, err := out.Result(t.Context())
	require.ErrorIs(t, err, bang, "the single error should pass through unwrapped")
}

func TestMuxSource_MultipleErrorsJoin(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}
	bCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS), fakeSource(bCS))
	out := mux("ignored")

	mainErr := errors.New("program failed")
	bErr := errors.New("snippet b failed")
	mainCS.Reject(mainErr)
	aCS.Fulfill(struct{}{})
	bCS.Reject(bErr)

	_, err := out.Result(t.Context())
	require.Error(t, err)
	require.ErrorIs(t, err, mainErr)
	require.ErrorIs(t, err, bErr)
}

// TestMuxSource_WaitsForAll asserts that even when one source finishes early the mux still blocks until
// the others complete; the returned promise must not resolve until everything has settled.
func TestMuxSource_WaitsForAll(t *testing.T) {
	t.Parallel()

	mainCS := &promise.CompletionSource[struct{}]{}
	aCS := &promise.CompletionSource[struct{}]{}

	mux := NewMuxSource(t.Context(), nil, fakeSource(mainCS), fakeSource(aCS))
	out := mux("ignored")

	mainCS.Fulfill(struct{}{})

	// With one of two sources still pending the mux promise must not be resolved yet.
	if _, _, ok := out.TryResult(); ok {
		t.Fatal("mux should not be resolved while a sub-source is still pending")
	}

	aCS.Fulfill(struct{}{})
	_, err := out.Result(t.Context())
	require.NoError(t, err)
}

// TestMuxSource_CancelContextStopsWaiting verifies that cancelling the constructor ctx causes the mux to
// fulfill (with the ctx error) without all sub-sources having to complete.
func TestMuxSource_CancelContextStopsWaiting(t *testing.T) {
	t.Parallel()

	muxCtx, cancel := context.WithCancel(t.Context())

	mainCS := &promise.CompletionSource[struct{}]{}
	hangCS := &promise.CompletionSource[struct{}]{} // never resolved

	mux := NewMuxSource(muxCtx, nil, fakeSource(mainCS), fakeSource(hangCS))
	out := mux("ignored")

	mainCS.Fulfill(struct{}{})

	// While hangCS is pending and ctx is alive, mux should still be waiting.
	if _, _, ok := out.TryResult(); ok {
		t.Fatal("mux should still be waiting while a sub-source is pending and ctx is alive")
	}

	// Cancelling the ctx wakes the still-blocked waiter and the mux resolves.
	cancel()

	_, err := out.Result(t.Context())
	require.ErrorIs(t, err, context.Canceled)
}
