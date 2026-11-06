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

package policyx

import (
	"context"
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// marshalAnalyzeProperties encodes properties the way the engine does, with unknown
// values kept as computed markers.
func marshalAnalyzeProperties(t *testing.T, props resource.PropertyMap) *structpb.Struct {
	t.Helper()
	rpcProps, err := plugin.MarshalProperties(props, plugin.MarshalOptions{KeepUnknowns: true})
	require.NoError(t, err)
	return rpcProps
}

func newTestPack(t *testing.T, policies ...Policy) (PolicyPack, map[string]PolicyConfig) {
	t.Helper()
	pack, err := NewPolicyPack("test-pack", semver.MustParse("1.0.0"), EnforcementLevelAdvisory, policies)
	require.NoError(t, err)
	return pack, map[string]PolicyConfig{}
}

func TestUnknownValueErrorFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		err  *UnknownValueError
		want string
	}{
		{
			err:  &UnknownValueError{Path: "name", Type: "string"},
			want: "string value at .name can't be known during preview",
		},
		{
			err:  &UnknownValueError{Path: "spec.tags", Type: "float64"},
			want: "float64 value at .spec.tags can't be known during preview",
		},
		{
			err:  &UnknownValueError{},
			want: "property value can't be known during preview",
		},
		{
			err:  &UnknownValueError{Type: "string"},
			want: "string value can't be known during preview",
		},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.err.Error())
	}
}

// TestAnalyzeRecoversFromComputedValuePanic reproduces the issue where a policy reads a
// computed property with an accessor such as AsString, which panics and used to take
// down the whole policy pack process.
func TestAnalyzeRecoversFromComputedValuePanic(t *testing.T) {
	t.Parallel()

	pack, config := newTestPack(t, NewResourceValidationPolicy("reads-name", ResourceValidationPolicyArgs{
		Description: "Reads a property that is unknown during preview",
		ValidateResource: func(_ context.Context, args ResourceValidationArgs) error {
			_ = args.Resource.Properties.Get("name").AsString()
			return nil
		},
	}))

	srv := &analyzerServer{policyPack: pack, config: config}

	resp, err := srv.Analyze(t.Context(), &pulumirpc.AnalyzeRequest{
		Type: "simple:index:Resource",
		Urn:  "urn:pulumi:test::test::simple:index:Resource::res",
		Name: "res",
		Properties: marshalAnalyzeProperties(t, resource.PropertyMap{
			"name": resource.MakeComputed(resource.NewStringProperty("")),
		}),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)

	diag := resp.GetDiagnostics()[0]
	assert.Equal(t, "reads-name", diag.GetPolicyName())
	assert.Equal(t, "test-pack", diag.GetPolicyPackName())
	assert.Equal(t, "1.0.0", diag.GetPolicyPackVersion())
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel())
	assert.Equal(t, "urn:pulumi:test::test::simple:index:Resource::res", diag.GetUrn())
	assert.Contains(t, diag.GetMessage(),
		"can't run policy 'reads-name' from policy pack 'test-pack@v1.0.0' during preview:")
	assert.Contains(t, diag.GetMessage(), "string value can't be known during preview")
}

// TestAnalyzeUnknownValueErrorIsAdvisory checks that a policy can detect a computed
// value itself and return an *UnknownValueError to get an advisory diagnostic instead
// of failing the whole Analyze call.
func TestAnalyzeUnknownValueErrorIsAdvisory(t *testing.T) {
	t.Parallel()

	pack, config := newTestPack(t, NewResourceValidationPolicy("checks-name", ResourceValidationPolicyArgs{
		Description: "Checks a property that may be unknown during preview",
		ValidateResource: func(_ context.Context, args ResourceValidationArgs) error {
			v := args.Resource.Properties.Get("name")
			if v.IsComputed() {
				return &UnknownValueError{Path: "name", Type: "string", Value: v}
			}
			return nil
		},
	}))

	srv := &analyzerServer{policyPack: pack, config: config}

	resp, err := srv.Analyze(t.Context(), &pulumirpc.AnalyzeRequest{
		Type: "simple:index:Resource",
		Urn:  "urn:pulumi:test::test::simple:index:Resource::res",
		Name: "res",
		Properties: marshalAnalyzeProperties(t, resource.PropertyMap{
			"name": resource.MakeComputed(resource.NewStringProperty("")),
		}),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)
	assert.Contains(t, resp.GetDiagnostics()[0].GetMessage(),
		"string value at .name can't be known during preview")
}

// TestAnalyzeOtherErrorsStillFail verifies that errors that are not UnknownValueError
// still fail the Analyze call, matching the Node.js and Python SDKs.
func TestAnalyzeOtherErrorsStillFail(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	pack, config := newTestPack(t, NewResourceValidationPolicy("fails", ResourceValidationPolicyArgs{
		ValidateResource: func(context.Context, ResourceValidationArgs) error {
			return boom
		},
	}))

	srv := &analyzerServer{policyPack: pack, config: config}

	resp, err := srv.Analyze(t.Context(), &pulumirpc.AnalyzeRequest{
		Type: "simple:index:Resource",
		Urn:  "urn:pulumi:test::test::simple:index:Resource::res",
		Name: "res",
		Properties: marshalAnalyzeProperties(t, resource.PropertyMap{
			"name": resource.NewStringProperty("value"),
		}),
	})
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

// TestAnalyzeOtherPanicsAreRecoveredButFail verifies that a panic that is not caused by
// a computed value access no longer kills the process, but still fails the Analyze call.
func TestAnalyzeOtherPanicsAreRecoveredButFail(t *testing.T) {
	t.Parallel()

	pack, config := newTestPack(t, NewResourceValidationPolicy("panics", ResourceValidationPolicyArgs{
		ValidateResource: func(context.Context, ResourceValidationArgs) error {
			panic("unexpected condition")
		},
	}))

	srv := &analyzerServer{policyPack: pack, config: config}

	resp, err := srv.Analyze(t.Context(), &pulumirpc.AnalyzeRequest{
		Type: "simple:index:Resource",
		Urn:  "urn:pulumi:test::test::simple:index:Resource::res",
		Name: "res",
		Properties: marshalAnalyzeProperties(t, resource.PropertyMap{
			"name": resource.NewStringProperty("value"),
		}),
	})
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "policy function panicked")
}

// TestAnalyzeStackRecoversFromComputedValuePanic checks the same recovery behavior for
// stack validation policies.
func TestAnalyzeStackRecoversFromComputedValuePanic(t *testing.T) {
	t.Parallel()

	pack, config := newTestPack(t, NewStackValidationPolicy("reads-name", StackValidationPolicyArgs{
		Description: "Reads a property that is unknown during preview",
		ValidateStack: func(_ context.Context, args StackValidationArgs) error {
			_ = args.Resources[0].Properties.Get("name").AsString()
			return nil
		},
	}))

	srv := &analyzerServer{policyPack: pack, config: config}

	rpcProps := marshalAnalyzeProperties(t, resource.PropertyMap{
		"name": resource.MakeComputed(resource.NewStringProperty("")),
	})
	resp, err := srv.AnalyzeStack(t.Context(), &pulumirpc.AnalyzeStackRequest{
		Resources: []*pulumirpc.AnalyzerResource{
			{
				Type:       "simple:index:Resource",
				Urn:        "urn:pulumi:test::test::simple:index:Resource::res",
				Name:       "res",
				Properties: rpcProps,
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)
	assert.Contains(t, resp.GetDiagnostics()[0].GetMessage(),
		"can't run policy 'reads-name' from policy pack 'test-pack@v1.0.0' during preview:")
}

// TestRemediateUnknownValueErrorIsDiagnostic checks that an UnknownValueError raised by
// a remediation is reported as a diagnostic on the remediation, matching the Node.js
// SDK's behavior.
func TestRemediateUnknownValueErrorIsDiagnostic(t *testing.T) {
	t.Parallel()

	pack, _ := newTestPack(t, NewResourceRemediationPolicy("fixup", ResourceRemediationPolicyArgs{
		Description: "Fixes a property that may be unknown during preview",
		RemediateResource: func(_ context.Context, args ResourceRemediationArgs) (*property.Map, error) {
			v := args.Resource.Properties.Get("name")
			if v.IsComputed() {
				return nil, &UnknownValueError{Path: "name", Type: "string", Value: v}
			}
			result := property.NewMap(map[string]property.Value{"value": property.New(false)})
			return &result, nil
		},
	}))

	srv := &analyzerServer{policyPack: pack, config: map[string]PolicyConfig{
		"fixup": {EnforcementLevel: EnforcementLevelRemediate},
	}}

	resp, err := srv.Remediate(t.Context(), &pulumirpc.AnalyzeRequest{
		Type: "simple:index:Resource",
		Urn:  "urn:pulumi:test::test::simple:index:Resource::res",
		Name: "res",
		Properties: marshalAnalyzeProperties(t, resource.PropertyMap{
			"name": resource.MakeComputed(resource.NewStringProperty("")),
		}),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetRemediations(), 1)

	remediation := resp.GetRemediations()[0]
	assert.Equal(t, "fixup", remediation.GetPolicyName())
	assert.Nil(t, remediation.GetProperties())
	assert.Contains(t, remediation.GetDiagnostic(),
		"can't run remediation 'fixup' from policy pack 'test-pack@v1.0.0' during preview:")
	assert.Contains(t, remediation.GetDiagnostic(), "string value at .name can't be known during preview")
}

// TestRemediateRecoversFromComputedValuePanic checks that a remediation which panics on
// a computed value is recovered and reported as a diagnostic too.
func TestRemediateRecoversFromComputedValuePanic(t *testing.T) {
	t.Parallel()

	pack, _ := newTestPack(t, NewResourceRemediationPolicy("fixup", ResourceRemediationPolicyArgs{
		RemediateResource: func(_ context.Context, args ResourceRemediationArgs) (*property.Map, error) {
			_ = args.Resource.Properties.Get("name").AsString()
			return nil, nil
		},
	}))

	srv := &analyzerServer{policyPack: pack, config: map[string]PolicyConfig{
		"fixup": {EnforcementLevel: EnforcementLevelRemediate},
	}}

	resp, err := srv.Remediate(t.Context(), &pulumirpc.AnalyzeRequest{
		Type: "simple:index:Resource",
		Urn:  "urn:pulumi:test::test::simple:index:Resource::res",
		Name: "res",
		Properties: marshalAnalyzeProperties(t, resource.PropertyMap{
			"name": resource.MakeComputed(resource.NewStringProperty("")),
		}),
	})
	require.NoError(t, err)
	require.Len(t, resp.GetRemediations(), 1)
	assert.Contains(t, resp.GetRemediations()[0].GetDiagnostic(), "string value can't be known during preview")
}
