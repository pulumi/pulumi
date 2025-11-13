// Copyright 2022-2025, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRequiredPolicy struct {
	name    string
	version string
	config  map[string]*json.RawMessage
}

func (p *testRequiredPolicy) Name() string {
	return p.name
}

func (p *testRequiredPolicy) Version() string {
	return p.version
}

func (p *testRequiredPolicy) Install(_ *plugin.Context) (string, error) {
	return "", nil
}

func (p *testRequiredPolicy) Config() map[string]*json.RawMessage {
	return p.config
}

func NewRequiredPolicy(name, version string, config map[string]*json.RawMessage) RequiredPolicy {
	return &testRequiredPolicy{
		name:    name,
		version: version,
		config:  config,
	}
}

func TestSimpleAnalyzer(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			assert.Equal(t, "", opts.Organization)
			assert.Equal(t, "test-proj", opts.Project)
			assert.Equal(t, "test", opts.Stack)

			assert.Equal(t, map[config.Key]string{
				config.MustMakeKey(opts.Project, "bool"):   "true",
				config.MustMakeKey(opts.Project, "float"):  "1.5",
				config.MustMakeKey(opts.Project, "string"): "hello",
				config.MustMakeKey(opts.Project, "obj"):    "{\"key\":\"value\"}",
			}, opts.Config)

			return &deploytest.Analyzer{}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	proj := "test-proj"
	p := &lt.TestPlan{
		Project: proj,
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: hostF,
		},
		Config: config.Map{
			config.MustMakeKey(proj, "bool"):   config.NewTypedValue("true", config.TypeBool),
			config.MustMakeKey(proj, "float"):  config.NewTypedValue("1.5", config.TypeFloat),
			config.MustMakeKey(proj, "string"): config.NewTypedValue("hello", config.TypeString),
			config.MustMakeKey(proj, "obj"):    config.NewObjectValue("{\"key\": \"value\"}"),
		},
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NoError(t, err)
}

func TestSimpleAnalyzeResourceFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name: "analyzerA",
					Policies: []plugin.AnalyzerPolicyInfo{
						{
							Name:             "always-fails",
							Description:      "a policy that always fails",
							EnforcementLevel: apitype.Mandatory,
							Severity:         apitype.PolicySeverityHigh,
						},
					},
				},
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					if r.Type != "pkgA:m:typA" {
						return plugin.AnalyzeResponse{
							NotApplicable: []plugin.PolicyNotApplicable{
								{PolicyName: "always-fails", Reason: "not the right resource type"},
							},
						}, nil
					}

					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Description:      "a policy that always fails",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Mandatory,
						URN:              r.URN,
					}}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: hostF,
		},
	}

	expectedResourceURN := p.NewURN("pkgA:m:typA", "resA", "")
	expectedProviderURN := p.NewURN("pulumi:providers:pkgA", "default", "")

	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		events []Event, err error,
	) error {
		var violationEvents []Event
		var summaryEvents []Event
		for _, e := range events {
			switch e.Type { //nolint:exhaustive
			case PolicyViolationEvent:
				violationEvents = append(violationEvents, e)
			case PolicyAnalyzeSummaryEvent:
				summaryEvents = append(summaryEvents, e)
			}
		}

		require.Len(t, violationEvents, 1)
		require.IsType(t, PolicyViolationEventPayload{}, violationEvents[0].Payload())
		violationPayload := violationEvents[0].Payload().(PolicyViolationEventPayload)
		assert.Equal(t, expectedResourceURN, violationPayload.ResourceURN)
		assert.Equal(t, "always-fails", violationPayload.PolicyName)
		assert.Equal(t, "analyzerA", violationPayload.PolicyPackName)
		assert.Contains(t, violationPayload.Message, "a policy failed")
		assert.Equal(t, apitype.Mandatory, violationPayload.EnforcementLevel)
		assert.Equal(t, apitype.PolicySeverityHigh, violationPayload.Severity)

		require.Len(t, summaryEvents, 2)

		require.IsType(t, PolicyAnalyzeSummaryEventPayload{}, summaryEvents[0].Payload())
		summaryPayload0 := summaryEvents[0].Payload().(PolicyAnalyzeSummaryEventPayload)
		assert.Equal(t, expectedProviderURN, summaryPayload0.ResourceURN)
		assert.Equal(t, "analyzerA", summaryPayload0.PolicyPackName)
		assert.Empty(t, summaryPayload0.Passed)
		assert.Empty(t, summaryPayload0.Failed)

		require.IsType(t, PolicyAnalyzeSummaryEventPayload{}, summaryEvents[1].Payload())
		summaryPayload1 := summaryEvents[1].Payload().(PolicyAnalyzeSummaryEventPayload)
		assert.Equal(t, expectedResourceURN, summaryPayload1.ResourceURN)
		assert.Equal(t, "analyzerA", summaryPayload1.PolicyPackName)
		assert.Empty(t, summaryPayload1.Passed)
		assert.Equal(t, []string{"always-fails"}, summaryPayload1.Failed)

		return err
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.Error(t, err)
}

func TestSimpleAnalyzeStackFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name: "analyzerA",
					Policies: []plugin.AnalyzerPolicyInfo{
						{
							Name:             "always-fails",
							Description:      "a policy that always fails",
							EnforcementLevel: apitype.Mandatory,
						},
					},
				},
				AnalyzeStackF: func(rs []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Description:      "a policy that always fails",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Mandatory,
						URN:              rs[0].URN,
					}}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			SkipDisplayTests: true, // TODO: this seems flaky, could use some more investigation.
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: hostF,
		},
	}

	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		events []Event, err error,
	) error {
		var violationEvents []Event
		var summaryEvents []Event
		for _, e := range events {
			switch e.Type { //nolint:exhaustive
			case PolicyViolationEvent:
				violationEvents = append(violationEvents, e)
			case PolicyAnalyzeStackSummaryEvent:
				summaryEvents = append(summaryEvents, e)
			}
		}

		require.Len(t, violationEvents, 1)
		require.IsType(t, PolicyViolationEventPayload{}, violationEvents[0].Payload())
		violationPayload := violationEvents[0].Payload().(PolicyViolationEventPayload)
		assert.Equal(t, "always-fails", violationPayload.PolicyName)
		assert.Equal(t, "analyzerA", violationPayload.PolicyPackName)
		assert.Contains(t, violationPayload.Message, "a policy failed")
		assert.Equal(t, apitype.Mandatory, violationPayload.EnforcementLevel)

		require.Len(t, summaryEvents, 1)
		require.IsType(t, PolicyAnalyzeStackSummaryEventPayload{}, summaryEvents[0].Payload())
		summaryPayload0 := summaryEvents[0].Payload().(PolicyAnalyzeStackSummaryEventPayload)
		assert.Equal(t, "analyzerA", summaryPayload0.PolicyPackName)
		assert.Empty(t, summaryPayload0.Passed)
		assert.Equal(t, []string{"always-fails"}, summaryPayload0.Failed)

		return err
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.Error(t, err)
}

// TestResourceRemediation tests a very simple sequence of remediations. We register two, to ensure that
// the remediations are applied in the order specified, an important part of the design.
func TestResourceRemediation(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name:    "analyzerA",
					Version: "1.0.0",
					Policies: []plugin.AnalyzerPolicyInfo{
						{
							Name:             "ignored",
							Description:      "a remediation that gets ignored because it runs first",
							EnforcementLevel: apitype.Remediate,
						},
						{
							Name:             "real-deal",
							Description:      "a remediation that actually gets applied because it runs last",
							EnforcementLevel: apitype.Remediate,
						},
					},
				},
				RemediateF: func(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
					if r.Type != "pkgA:m:typA" {
						return plugin.RemediateResponse{
							NotApplicable: []plugin.PolicyNotApplicable{
								{PolicyName: "ignored", Reason: "not the right resource type"},
								{PolicyName: "real-deal", Reason: "not the right resource type"},
							},
						}, nil
					}

					// Run two remediations to ensure they are applied in order.
					return plugin.RemediateResponse{Remediations: []plugin.Remediation{
						{
							PolicyName:        "ignored",
							PolicyPackName:    "analyzerA",
							PolicyPackVersion: "1.0.0",
							Description:       "a remediation that gets ignored because it runs first",
							Properties: resource.PropertyMap{
								"a":   resource.NewProperty("nope"),
								"ggg": resource.NewProperty(true),
							},
						},
						{
							PolicyName:        "real-deal",
							PolicyPackName:    "analyzerA",
							PolicyPackVersion: "1.0.0",
							Description:       "a remediation that actually gets applied because it runs last",
							Properties: resource.PropertyMap{
								"a":   resource.NewProperty("foo"),
								"fff": resource.NewProperty(true),
								"z":   resource.NewProperty("bar"),
							},
						},
					}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHostF(nil, nil, program, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: host,
		},
	}

	expectedResourceURN := p.NewURN("pkgA:m:typA", "resA", "")
	expectedProviderURN := p.NewURN("pulumi:providers:pkgA", "default", "")

	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		events []Event, err error,
	) error {
		var remediationEvents []Event
		var summaryEvents []Event
		for _, e := range events {
			switch e.Type { //nolint:exhaustive
			case PolicyRemediationEvent:
				remediationEvents = append(remediationEvents, e)
			case PolicyRemediateSummaryEvent:
				summaryEvents = append(summaryEvents, e)
			}
		}

		require.Len(t, remediationEvents, 2)

		require.IsType(t, PolicyRemediationEventPayload{}, remediationEvents[0].Payload())
		remediationPayload0 := remediationEvents[0].Payload().(PolicyRemediationEventPayload)
		assert.Equal(t, expectedResourceURN, remediationPayload0.ResourceURN)
		assert.Equal(t, "ignored", remediationPayload0.PolicyName)
		assert.Equal(t, "analyzerA", remediationPayload0.PolicyPackName)
		assert.Equal(t, "1.0.0", remediationPayload0.PolicyPackVersion)
		assert.Equal(t, resource.PropertyMap{}, remediationPayload0.Before)
		assert.Equal(t, resource.PropertyMap{
			"a":   resource.NewProperty("nope"),
			"ggg": resource.NewProperty(true),
		}, remediationPayload0.After)

		require.IsType(t, PolicyRemediationEventPayload{}, remediationEvents[1].Payload())
		remediationPayload1 := remediationEvents[1].Payload().(PolicyRemediationEventPayload)
		assert.Equal(t, expectedResourceURN, remediationPayload1.ResourceURN)
		assert.Equal(t, "real-deal", remediationPayload1.PolicyName)
		assert.Equal(t, "analyzerA", remediationPayload1.PolicyPackName)
		assert.Equal(t, "1.0.0", remediationPayload1.PolicyPackVersion)
		assert.Equal(t, resource.PropertyMap{
			"a":   resource.NewProperty("nope"),
			"ggg": resource.NewProperty(true),
		}, remediationPayload1.Before)
		assert.Equal(t, resource.PropertyMap{
			"a":   resource.NewProperty("foo"),
			"fff": resource.NewProperty(true),
			"z":   resource.NewProperty("bar"),
		}, remediationPayload1.After)

		require.Len(t, summaryEvents, 2)

		require.IsType(t, PolicyRemediateSummaryEventPayload{}, summaryEvents[0].Payload())
		summaryPayload := summaryEvents[0].Payload().(PolicyRemediateSummaryEventPayload)
		assert.Equal(t, expectedProviderURN, summaryPayload.ResourceURN)
		assert.Equal(t, "analyzerA", summaryPayload.PolicyPackName)
		assert.Equal(t, "1.0.0", summaryPayload.PolicyPackVersion)
		assert.Empty(t, summaryPayload.Passed)
		assert.Empty(t, summaryPayload.Failed)

		require.IsType(t, PolicyRemediateSummaryEventPayload{}, summaryEvents[1].Payload())
		summaryPayload = summaryEvents[1].Payload().(PolicyRemediateSummaryEventPayload)
		assert.Equal(t, expectedResourceURN, summaryPayload.ResourceURN)
		assert.Equal(t, "analyzerA", summaryPayload.PolicyPackName)
		assert.Equal(t, "1.0.0", summaryPayload.PolicyPackVersion)
		assert.Empty(t, summaryPayload.Passed)
		assert.Equal(t, []string{"ignored", "real-deal"}, summaryPayload.Failed)

		return err
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)

	// Expect no error, valid snapshot, two resources:
	assert.Nil(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2) // stack plus pkA:m:typA

	// Ensure the rewritten properties have been applied to the inputs:
	r := snap.Resources[1]
	assert.Equal(t, "pkgA:m:typA", string(r.Type))
	require.Len(t, r.Inputs, 3)
	assert.Equal(t, "foo", r.Inputs["a"].StringValue())
	assert.Equal(t, true, r.Inputs["fff"].BoolValue())
	assert.Equal(t, "bar", r.Inputs["z"].StringValue())
}

// TestRemediationDiagnostic tests the case where a remediation issues a diagnostic rather than transforming
// state. In this case, the deployment should still succeed, even though no transforms took place.
func TestRemediationDiagnostic(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				RemediateF: func(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
					return plugin.RemediateResponse{Remediations: []plugin.Remediation{{
						PolicyName:        "warning",
						PolicyPackName:    "analyzerA",
						PolicyPackVersion: "1.0.0",
						Description:       "a remediation with a diagnostic",
						Diagnostic:        "warning - could not run due to unknowns",
					}}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHostF(nil, nil, program, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: host,
		},
	}

	project := p.GetProject()
	snap, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)

	// Expect no error, valid snapshot, two resources:
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.Len(t, snap.Resources, 2) // stack plus pkA:m:typA
}

// TestRemediateFailure tests the case where a remediation fails to execute. In this case, the whole
// deployment itself should also fail.
func TestRemediateFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				RemediateF: func(r plugin.AnalyzerResource) (plugin.RemediateResponse, error) {
					return plugin.RemediateResponse{}, errors.New("this remediation failed")
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.ErrorContains(t, err, "context canceled")
		return nil
	})
	host := deploytest.NewPluginHostF(nil, nil, program, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: host,
		},
	}

	project := p.GetProject()
	snap, res := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	require.NotNil(t, res)
	require.NotNil(t, snap)
	assert.Empty(t, snap.Resources)
}

func TestSimpleAnalyzeResourceFailureRemediateDowngradedToMandatory(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Description:      "a policy that always fails",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Remediate,
						URN:              r.URN,
					}}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: hostF,
		},
		Steps: []lt.TestStep{
			{
				Op:            Update,
				SkipPreview:   true,
				ExpectFailure: true,
				Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
					events []Event, err error,
				) error {
					violationEvents := []Event{}
					for _, e := range events {
						if e.Type == PolicyViolationEvent {
							violationEvents = append(violationEvents, e)
						}
					}
					require.Len(t, violationEvents, 1)
					assert.Equal(t, apitype.Mandatory,
						violationEvents[0].Payload().(PolicyViolationEventPayload).EnforcementLevel)

					return err
				},
			},
		},
	}

	p.Run(t, nil)
}

func TestSimpleAnalyzeStackFailureRemediateDowngradedToMandatory(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				AnalyzeStackF: func(rs []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Description:      "a policy that always fails",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Remediate,
						URN:              rs[0].URN,
					}}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T:                t,
			SkipDisplayTests: true, // TODO: this seems flaky, could use some more investigation.
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: hostF,
		},
		Steps: []lt.TestStep{
			{
				Op:            Update,
				SkipPreview:   true,
				ExpectFailure: true,
				Validate: func(project workspace.Project, target deploy.Target, entries JournalEntries,
					events []Event, err error,
				) error {
					violationEvents := []Event{}
					for _, e := range events {
						if e.Type == PolicyViolationEvent {
							violationEvents = append(violationEvents, e)
						}
					}
					require.Len(t, violationEvents, 1)
					assert.Equal(t, apitype.Mandatory,
						violationEvents[0].Payload().(PolicyViolationEventPayload).EnforcementLevel)

					return err
				},
			},
		},
	}

	p.Run(t, nil)
}

func TestAnalyzerCancellation(t *testing.T) {
	t.Parallel()

	gracefulShutdown := false
	loaders := []*deploytest.PluginLoader{
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				CancelF: func() error {
					gracefulShutdown = true
					return nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	ctx, cancel := context.WithCancel(context.Background())
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		time.Sleep(1 * time.Second)
		cancel()
		return nil
	})

	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: hostF,
		},
	}
	project, target := p.GetProject(), p.GetTarget(t, nil)

	op := lt.TestOp(Update)
	_, err := op.RunWithContext(ctx, project, target, p.Options, false, nil, nil)

	assert.ErrorContains(t, err, "BAIL: canceled")
	assert.True(t, gracefulShutdown)
}

func TestSimpleAnalyzeResourceMultipleViolations(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			policies := []plugin.AnalyzerPolicyInfo{
				{
					Name:             "always-fails-advisory-unspecified",
					Description:      "a policy that always fails unspecified",
					EnforcementLevel: apitype.Advisory,
				},
				{
					Name:             "always-fails-advisory-low",
					Description:      "a policy that always fails low",
					EnforcementLevel: apitype.Advisory,
					Severity:         apitype.PolicySeverityLow,
				},
				{
					Name:             "always-fails-advisory-medium",
					Description:      "a policy that always fails medium",
					EnforcementLevel: apitype.Advisory,
					Severity:         apitype.PolicySeverityMedium,
				},
				{
					Name:             "always-fails-advisory-high",
					Description:      "a policy that always fails high",
					EnforcementLevel: apitype.Advisory,
					Severity:         apitype.PolicySeverityHigh,
				},
				{
					Name:             "always-fails-advisory-critical",
					Description:      "a policy that always fails critical",
					EnforcementLevel: apitype.Advisory,
					Severity:         apitype.PolicySeverityCritical,
				},
				{
					Name:             "always-fails-unspecified",
					Description:      "a policy that always fails unspecified",
					EnforcementLevel: apitype.Mandatory,
				},
				{
					Name:             "always-fails-low",
					Description:      "a policy that always fails low",
					EnforcementLevel: apitype.Mandatory,
					Severity:         apitype.PolicySeverityLow,
				},
				{
					Name:             "always-fails-medium",
					Description:      "a policy that always fails medium",
					EnforcementLevel: apitype.Mandatory,
					Severity:         apitype.PolicySeverityMedium,
				},
				{
					Name:             "always-fails-high",
					Description:      "a policy that always fails high",
					EnforcementLevel: apitype.Mandatory,
					Severity:         apitype.PolicySeverityHigh,
				},
				{
					Name:             "always-fails-critical",
					Description:      "a policy that always fails critical",
					EnforcementLevel: apitype.Mandatory,
					Severity:         apitype.PolicySeverityCritical,
				},
			}
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name:     "analyzerA",
					Version:  "1.0.0",
					Policies: policies,
				},
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					if r.Type != "pkgA:m:typA" {
						return plugin.AnalyzeResponse{
							NotApplicable: []plugin.PolicyNotApplicable{
								{PolicyName: "always-fails", Reason: "not the right resource type"},
							},
						}, nil
					}

					var diagnostics []plugin.AnalyzeDiagnostic
					for _, p := range policies {
						diagnostics = append(diagnostics, plugin.AnalyzeDiagnostic{
							PolicyName:        p.Name,
							PolicyPackName:    "analyzerA",
							PolicyPackVersion: "1.0.0",
							Description:       p.Description,
							Message:           "a policy failed",
							EnforcementLevel:  p.EnforcementLevel,
							Severity:          p.Severity,
							URN:               r.URN,
						})
					}

					return plugin.AnalyzeResponse{Diagnostics: diagnostics}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "1.0.0", nil)},
			},
			HostF: hostF,
		},
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Error(t, err)

	// Test data contains golden files for expected sorted output.
}

// TestSimpleAnalyzeResourceFailureSeverityOverride tests that a policy diagnostic's severity
// can be overridden as part of the diagnostic.
func TestSimpleAnalyzeResourceFailureSeverityOverride(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name: "analyzerA",
					Policies: []plugin.AnalyzerPolicyInfo{
						{
							Name:             "always-fails",
							Description:      "a policy that always fails",
							EnforcementLevel: apitype.Mandatory,
							Severity:         apitype.PolicySeverityMedium,
						},
					},
				},
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					if r.Type != "pkgA:m:typA" {
						return plugin.AnalyzeResponse{
							NotApplicable: []plugin.PolicyNotApplicable{
								{PolicyName: "always-fails", Reason: "not the right resource type"},
							},
						}, nil
					}

					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Description:      "a policy that always fails",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Mandatory,
						URN:              r.URN,
						Severity:         apitype.PolicySeverityCritical,
					}}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			},
			HostF: hostF,
		},
	}

	expectedResourceURN := p.NewURN("pkgA:m:typA", "resA", "")
	expectedProviderURN := p.NewURN("pulumi:providers:pkgA", "default", "")

	validate := func(project workspace.Project, target deploy.Target, entries JournalEntries,
		events []Event, err error,
	) error {
		var violationEvents []Event
		var summaryEvents []Event
		for _, e := range events {
			switch e.Type { //nolint:exhaustive
			case PolicyViolationEvent:
				violationEvents = append(violationEvents, e)
			case PolicyAnalyzeSummaryEvent:
				summaryEvents = append(summaryEvents, e)
			}
		}

		require.Len(t, violationEvents, 1)
		require.IsType(t, PolicyViolationEventPayload{}, violationEvents[0].Payload())
		violationPayload := violationEvents[0].Payload().(PolicyViolationEventPayload)
		assert.Equal(t, expectedResourceURN, violationPayload.ResourceURN)
		assert.Equal(t, "always-fails", violationPayload.PolicyName)
		assert.Equal(t, "analyzerA", violationPayload.PolicyPackName)
		assert.Contains(t, violationPayload.Message, "a policy failed")
		assert.Equal(t, apitype.Mandatory, violationPayload.EnforcementLevel)
		assert.Equal(t, apitype.PolicySeverityCritical, violationPayload.Severity)

		require.Len(t, summaryEvents, 2)

		require.IsType(t, PolicyAnalyzeSummaryEventPayload{}, summaryEvents[0].Payload())
		summaryPayload0 := summaryEvents[0].Payload().(PolicyAnalyzeSummaryEventPayload)
		assert.Equal(t, expectedProviderURN, summaryPayload0.ResourceURN)
		assert.Equal(t, "analyzerA", summaryPayload0.PolicyPackName)
		assert.Empty(t, summaryPayload0.Passed)
		assert.Empty(t, summaryPayload0.Failed)

		require.IsType(t, PolicyAnalyzeSummaryEventPayload{}, summaryEvents[1].Payload())
		summaryPayload1 := summaryEvents[1].Payload().(PolicyAnalyzeSummaryEventPayload)
		assert.Equal(t, expectedResourceURN, summaryPayload1.ResourceURN)
		assert.Equal(t, "analyzerA", summaryPayload1.PolicyPackName)
		assert.Empty(t, summaryPayload1.Passed)
		assert.Equal(t, []string{"always-fails"}, summaryPayload1.Failed)

		return err
	}

	project := p.GetProject()
	_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	assert.Error(t, err)
}
