// Copyright 2016-2026, Pulumi Corporation.
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

package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestBuildProviderMap(t *testing.T) {
	t.Parallel()

	t.Run("empty snapshot", func(t *testing.T) {
		t.Parallel()
		snap := &deploy.Snapshot{Resources: []*resource.State{}}
		m := buildProviderMap(snap)
		assert.Empty(t, m)
	})

	t.Run("identifies provider resources", func(t *testing.T) {
		t.Parallel()
		providerState := &resource.State{
			Type: "pulumi:providers:aws",
			URN:  "urn:pulumi:stack::project::pulumi:providers:aws::default",
		}
		regularState := &resource.State{
			Type: "aws:s3:Bucket",
			URN:  "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
		}
		snap := &deploy.Snapshot{
			Resources: []*resource.State{providerState, regularState},
		}
		m := buildProviderMap(snap)
		assert.Len(t, m, 1)
		assert.Equal(t, providerState, m[providerState.URN])
	})

	t.Run("multiple providers", func(t *testing.T) {
		t.Parallel()
		p1 := &resource.State{
			Type: "pulumi:providers:aws",
			URN:  "urn:pulumi:stack::project::pulumi:providers:aws::default",
		}
		p2 := &resource.State{
			Type: "pulumi:providers:gcp",
			URN:  "urn:pulumi:stack::project::pulumi:providers:gcp::default",
		}
		snap := &deploy.Snapshot{
			Resources: []*resource.State{p1, p2},
		}
		m := buildProviderMap(snap)
		assert.Len(t, m, 2)
	})
}

func TestResolveProvider(t *testing.T) {
	t.Parallel()

	providerState := &resource.State{
		Type:   "pulumi:providers:aws",
		URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default",
		Inputs: resource.PropertyMap{"region": resource.NewStringProperty("us-east-1")},
	}
	providerMap := map[resource.URN]*resource.State{
		providerState.URN: providerState,
	}

	t.Run("empty provider string", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{Provider: ""}
		result := resolveProvider(state, providerMap)
		assert.Nil(t, result)
	})

	t.Run("valid provider reference", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{
			Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default::some-id",
		}
		result := resolveProvider(state, providerMap)
		require.NotNil(t, result)
		assert.Equal(t, providerState.URN, result.URN)
		assert.Equal(t, providerState.Type, result.Type)
		assert.Equal(t, "default", result.Name)
		assert.Equal(t, providerState.Inputs, result.Properties)
	})

	t.Run("provider not found in map", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{
			Provider: "urn:pulumi:stack::project::pulumi:providers:azure::default::some-id",
		}
		result := resolveProvider(state, providerMap)
		assert.Nil(t, result)
	})

	t.Run("malformed provider reference", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{Provider: "not-a-valid-reference"}
		result := resolveProvider(state, providerMap)
		assert.Nil(t, result)
	})
}

func TestBuildAnalyzerResource(t *testing.T) {
	t.Parallel()

	providerState := &resource.State{
		Type:   "pulumi:providers:aws",
		URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default",
		Inputs: resource.PropertyMap{"region": resource.NewStringProperty("us-east-1")},
	}
	providerMap := map[resource.URN]*resource.State{
		providerState.URN: providerState,
	}

	t.Run("uses Inputs not Outputs", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{
			Type:    "aws:s3:Bucket",
			URN:     "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
			Inputs:  resource.PropertyMap{"bucketName": resource.NewStringProperty("from-inputs")},
			Outputs: resource.PropertyMap{"bucketName": resource.NewStringProperty("from-outputs"), "arn": resource.NewStringProperty("arn:aws:...")},
		}
		r := buildAnalyzerResource(state, providerMap)
		assert.Equal(t, state.Inputs, r.Properties, "Analyze should use Inputs")
	})

	t.Run("sets resource metadata", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{
			Type:    "aws:s3:Bucket",
			URN:     "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
			Inputs:  resource.PropertyMap{},
			Protect: true,
			Parent:  "urn:pulumi:stack::project::pulumi:pulumi:Stack::stack",
			IgnoreChanges: []string{"tags"},
			CustomTimeouts: resource.CustomTimeouts{Create: 60},
		}
		r := buildAnalyzerResource(state, providerMap)
		assert.Equal(t, state.URN, r.URN)
		assert.Equal(t, state.Type, r.Type)
		assert.Equal(t, "my-bucket", r.Name)
		assert.True(t, r.Options.Protect)
		assert.Equal(t, state.Parent, r.Options.Parent)
		assert.Equal(t, []string{"tags"}, r.Options.IgnoreChanges)
	})

	t.Run("resolves provider", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{
			Type:     "aws:s3:Bucket",
			URN:      "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
			Inputs:   resource.PropertyMap{},
			Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default::some-id",
		}
		r := buildAnalyzerResource(state, providerMap)
		require.NotNil(t, r.Provider)
		assert.Equal(t, providerState.URN, r.Provider.URN)
	})
}

func TestBuildAnalyzerStackResources(t *testing.T) {
	t.Parallel()

	providerState := &resource.State{
		Type:   "pulumi:providers:aws",
		URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default",
		Inputs: resource.PropertyMap{"region": resource.NewStringProperty("us-east-1")},
	}
	providerMap := map[resource.URN]*resource.State{
		providerState.URN: providerState,
	}

	t.Run("uses Outputs not Inputs", func(t *testing.T) {
		t.Parallel()
		state := &resource.State{
			Type:    "aws:s3:Bucket",
			URN:     "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
			Inputs:  resource.PropertyMap{"bucketName": resource.NewStringProperty("from-inputs")},
			Outputs: resource.PropertyMap{"bucketName": resource.NewStringProperty("from-outputs"), "arn": resource.NewStringProperty("arn:aws:...")},
		}
		snap := &deploy.Snapshot{Resources: []*resource.State{state}}
		resources := buildAnalyzerStackResources(snap, providerMap)
		require.Len(t, resources, 1)
		assert.Equal(t, state.Outputs, resources[0].Properties, "AnalyzeStack should use Outputs")
	})

	t.Run("skips provider resources", func(t *testing.T) {
		t.Parallel()
		snap := &deploy.Snapshot{
			Resources: []*resource.State{
				providerState,
				{
					Type:    "aws:s3:Bucket",
					URN:     "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
					Outputs: resource.PropertyMap{},
				},
			},
		}
		resources := buildAnalyzerStackResources(snap, providerMap)
		assert.Len(t, resources, 1)
		assert.Equal(t, tokens.Type("aws:s3:Bucket"), resources[0].Type)
	})

	t.Run("skips pending deletes", func(t *testing.T) {
		t.Parallel()
		snap := &deploy.Snapshot{
			Resources: []*resource.State{
				{
					Type:    "aws:s3:Bucket",
					URN:     "urn:pulumi:stack::project::aws:s3:Bucket::deleted-bucket",
					Outputs: resource.PropertyMap{},
					Delete:  true,
				},
				{
					Type:    "aws:s3:Bucket",
					URN:     "urn:pulumi:stack::project::aws:s3:Bucket::active-bucket",
					Outputs: resource.PropertyMap{},
				},
			},
		}
		resources := buildAnalyzerStackResources(snap, providerMap)
		assert.Len(t, resources, 1)
		assert.Equal(t, resource.URN("urn:pulumi:stack::project::aws:s3:Bucket::active-bucket"), resources[0].URN)
	})

	t.Run("includes dependencies", func(t *testing.T) {
		t.Parallel()
		parentURN := resource.URN("urn:pulumi:stack::project::pulumi:pulumi:Stack::stack")
		depURN := resource.URN("urn:pulumi:stack::project::aws:s3:Bucket::dep-bucket")
		state := &resource.State{
			Type:         "aws:s3:BucketObject",
			URN:          "urn:pulumi:stack::project::aws:s3:BucketObject::obj",
			Outputs:      resource.PropertyMap{},
			Parent:       parentURN,
			Dependencies: []resource.URN{depURN},
			PropertyDependencies: map[resource.PropertyKey][]resource.URN{
				"bucket": {depURN},
			},
		}
		snap := &deploy.Snapshot{Resources: []*resource.State{state}}
		resources := buildAnalyzerStackResources(snap, providerMap)
		require.Len(t, resources, 1)
		assert.Equal(t, parentURN, resources[0].Parent)
		assert.Equal(t, []resource.URN{depURN}, resources[0].Dependencies)
		assert.Equal(t, state.PropertyDependencies, resources[0].PropertyDependencies)
	})
}

// checkMockAnalyzer implements plugin.Analyzer for testing.
type checkMockAnalyzer struct {
	name            tokens.QName
	info            plugin.AnalyzerInfo
	analyzeF        func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error)
	analyzeStackF   func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error)
}

func (m *checkMockAnalyzer) Close() error { return nil }
func (m *checkMockAnalyzer) Name() tokens.QName { return m.name }

func (m *checkMockAnalyzer) Analyze(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
	if m.analyzeF != nil {
		return m.analyzeF(r)
	}
	return plugin.AnalyzeResponse{}, nil
}

func (m *checkMockAnalyzer) AnalyzeStack(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
	if m.analyzeStackF != nil {
		return m.analyzeStackF(resources)
	}
	return plugin.AnalyzeResponse{}, nil
}

func (m *checkMockAnalyzer) Remediate(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
	return plugin.RemediateResponse{}, nil
}

func (m *checkMockAnalyzer) GetAnalyzerInfo() (plugin.AnalyzerInfo, error) {
	return m.info, nil
}

func (m *checkMockAnalyzer) GetPluginInfo() (plugin.PluginInfo, error) {
	return plugin.PluginInfo{}, nil
}

func (m *checkMockAnalyzer) Configure(policyConfig map[string]plugin.AnalyzerPolicyConfig) error {
	return nil
}

func (m *checkMockAnalyzer) Cancel(ctx context.Context) error {
	return nil
}

func newMockAnalyzer(name string) *checkMockAnalyzer {
	return &checkMockAnalyzer{
		name: tokens.QName(name),
		info: plugin.AnalyzerInfo{
			Name:    name,
			Version: "1.0.0",
			Policies: []plugin.AnalyzerPolicyInfo{
				{Name: "test-policy", EnforcementLevel: apitype.Mandatory},
			},
		},
	}
}

func TestRunPolicyChecks(t *testing.T) {
	t.Parallel()

	providerState := &resource.State{
		Type:   "pulumi:providers:aws",
		URN:    "urn:pulumi:stack::project::pulumi:providers:aws::default",
		Inputs: resource.PropertyMap{"region": resource.NewStringProperty("us-east-1")},
	}
	bucketState := &resource.State{
		Type:     "aws:s3:Bucket",
		URN:      "urn:pulumi:stack::project::aws:s3:Bucket::my-bucket",
		Inputs:   resource.PropertyMap{"bucketName": resource.NewStringProperty("input-name")},
		Outputs:  resource.PropertyMap{"bucketName": resource.NewStringProperty("output-name"), "arn": resource.NewStringProperty("arn:aws:s3:::my-bucket")},
		Provider: "urn:pulumi:stack::project::pulumi:providers:aws::default::some-id",
	}

	makeSnap := func(resources ...*resource.State) *deploy.Snapshot {
		return &deploy.Snapshot{Resources: resources}
	}

	makeEmitter := func(t *testing.T) (*eventEmitter, chan Event) {
		t.Helper()
		events := make(chan Event, 100)
		buffer, done := make(chan Event), make(chan bool)
		go func() {
			queueEvents(events, buffer, done)
			close(events)
		}()
		em := &eventEmitter{done: done, ch: buffer}
		return em, events
	}

	t.Run("no violations passes", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("clean-pack")
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		// Drain events.
		for range events {
		}

		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.Equal(t, 0, result.MandatoryViolations)
		assert.Equal(t, 0, result.AdvisoryViolations)
	})

	t.Run("mandatory violation fails", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("strict-pack")
		analyzer.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:        "no-public-buckets",
					PolicyPackName:    "strict-pack",
					PolicyPackVersion: "1.0.0",
					Message:           "Bucket must not be public",
					EnforcementLevel:  apitype.Mandatory,
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Equal(t, 1, result.MandatoryViolations)
	})

	t.Run("advisory violation passes", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("advisory-pack")
		analyzer.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:        "prefer-tags",
					PolicyPackName:    "advisory-pack",
					PolicyPackVersion: "1.0.0",
					Message:           "Resources should have tags",
					EnforcementLevel:  apitype.Advisory,
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.Equal(t, 0, result.MandatoryViolations)
		assert.Equal(t, 1, result.AdvisoryViolations)
	})

	t.Run("stack analysis mandatory violation fails", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("stack-pack")
		analyzer.analyzeStackF = func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:        "max-resources",
					PolicyPackName:    "stack-pack",
					PolicyPackVersion: "1.0.0",
					Message:           "Too many resources",
					EnforcementLevel:  apitype.Mandatory,
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Equal(t, 1, result.MandatoryViolations)
	})

	t.Run("remediate level treated as mandatory", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("remediate-pack")
		analyzer.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:        "enforce-encryption",
					PolicyPackName:    "remediate-pack",
					PolicyPackVersion: "1.0.0",
					Message:           "Encryption required",
					EnforcementLevel:  apitype.Remediate,
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Equal(t, 1, result.MandatoryViolations,
			"remediate violations should be counted as mandatory")
	})

	t.Run("multiple analyzers all run", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)

		analyzer1 := newMockAnalyzer("pack-a")
		analyzer1.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:       "policy-a",
					PolicyPackName:   "pack-a",
					Message:          "violation a",
					EnforcementLevel: apitype.Advisory,
				}},
			}, nil
		}
		analyzer2 := newMockAnalyzer("pack-b")
		analyzer2.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:       "policy-b",
					PolicyPackName:   "pack-b",
					Message:          "violation b",
					EnforcementLevel: apitype.Mandatory,
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer1, analyzer2}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Equal(t, 1, result.MandatoryViolations)
		assert.Equal(t, 1, result.AdvisoryViolations)
	})

	t.Run("skips pending delete resources", func(t *testing.T) {
		t.Parallel()
		deletedState := &resource.State{
			Type:    "aws:s3:Bucket",
			URN:     "urn:pulumi:stack::project::aws:s3:Bucket::deleted",
			Inputs:  resource.PropertyMap{},
			Outputs: resource.PropertyMap{},
			Delete:  true,
		}
		snap := makeSnap(providerState, deletedState)
		providerMap := buildProviderMap(snap)

		var analyzeCalled bool
		analyzer := newMockAnalyzer("test-pack")
		analyzer.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			analyzeCalled = true
			return plugin.AnalyzeResponse{}, nil
		}
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.False(t, analyzeCalled, "deleted resources should not be analyzed")
	})

	t.Run("skips provider resources for analysis", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState)
		providerMap := buildProviderMap(snap)

		var analyzeCalled bool
		analyzer := newMockAnalyzer("test-pack")
		analyzer.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			analyzeCalled = true
			return plugin.AnalyzeResponse{}, nil
		}
		var analyzeStackCalled bool
		analyzer.analyzeStackF = func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
			analyzeStackCalled = true
			assert.Empty(t, resources, "provider resources should not appear in stack analysis")
			return plugin.AnalyzeResponse{}, nil
		}
		emitter, events := makeEmitter(t)

		result, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.True(t, result.Passed)
		assert.False(t, analyzeCalled, "provider resources should not be analyzed individually")
		assert.True(t, analyzeStackCalled, "AnalyzeStack should still be called")
	})

	t.Run("Analyze receives Inputs, AnalyzeStack receives Outputs", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)

		var analyzeProperties resource.PropertyMap
		var stackProperties resource.PropertyMap

		analyzer := newMockAnalyzer("property-check")
		analyzer.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			analyzeProperties = r.Properties
			return plugin.AnalyzeResponse{}, nil
		}
		analyzer.analyzeStackF = func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
			if len(resources) > 0 {
				stackProperties = resources[0].Properties
			}
			return plugin.AnalyzeResponse{}, nil
		}
		emitter, events := makeEmitter(t)

		_, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()
		for range events {
		}

		require.NoError(t, err)
		assert.Equal(t, bucketState.Inputs, analyzeProperties,
			"Analyze() should receive Inputs")
		assert.Equal(t, bucketState.Outputs, stackProperties,
			"AnalyzeStack() should receive Outputs")
	})

	t.Run("violation events emitted", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("event-pack")
		analyzer.analyzeF = func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:        "test-policy",
					PolicyPackName:    "event-pack",
					PolicyPackVersion: "1.0.0",
					Message:           "test violation",
					EnforcementLevel:  apitype.Mandatory,
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		_, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()

		var violations []PolicyViolationEventPayload
		for e := range events {
			if payload, ok := e.Payload().(PolicyViolationEventPayload); ok {
				violations = append(violations, payload)
			}
		}

		require.NoError(t, err)
		require.Len(t, violations, 1)
		assert.Equal(t, "test-policy", violations[0].PolicyName)
		assert.Equal(t, "event-pack", violations[0].PolicyPackName)
		assert.Equal(t, apitype.Mandatory, violations[0].EnforcementLevel)
		assert.Equal(t, bucketState.URN, violations[0].ResourceURN)
	})

	t.Run("stack violation with unknown URN uses first resource", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("stack-urn-pack")
		analyzer.analyzeStackF = func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:        "stack-policy",
					PolicyPackName:    "stack-urn-pack",
					PolicyPackVersion: "1.0.0",
					Message:           "stack violation",
					EnforcementLevel:  apitype.Mandatory,
					URN:               "urn:pulumi:stack::project::aws:s3:Bucket::nonexistent",
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		_, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()

		var violations []PolicyViolationEventPayload
		for e := range events {
			if payload, ok := e.Payload().(PolicyViolationEventPayload); ok {
				violations = append(violations, payload)
			}
		}

		require.NoError(t, err)
		require.Len(t, violations, 1)
		assert.Equal(t, bucketState.URN, violations[0].ResourceURN,
			"should fall back to first resource URN when diagnostic URN is not in stack")
	})

	t.Run("stack violation with valid URN preserves it", func(t *testing.T) {
		t.Parallel()
		snap := makeSnap(providerState, bucketState)
		providerMap := buildProviderMap(snap)
		analyzer := newMockAnalyzer("stack-valid-urn-pack")
		analyzer.analyzeStackF = func(resources []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
			return plugin.AnalyzeResponse{
				Diagnostics: []plugin.AnalyzeDiagnostic{{
					PolicyName:        "stack-policy",
					PolicyPackName:    "stack-valid-urn-pack",
					PolicyPackVersion: "1.0.0",
					Message:           "stack violation",
					EnforcementLevel:  apitype.Mandatory,
					URN:               bucketState.URN,
				}},
			}, nil
		}
		emitter, events := makeEmitter(t)

		_, err := runPolicyChecks(snap, []plugin.Analyzer{analyzer}, providerMap, emitter)
		emitter.Close()

		var violations []PolicyViolationEventPayload
		for e := range events {
			if payload, ok := e.Payload().(PolicyViolationEventPayload); ok {
				violations = append(violations, payload)
			}
		}

		require.NoError(t, err)
		require.Len(t, violations, 1)
		assert.Equal(t, bucketState.URN, violations[0].ResourceURN)
	})
}
