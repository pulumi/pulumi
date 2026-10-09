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

package lifecycletest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// policyExceptionTest runs an update (or a preview) of a program that registers resA and resB against a
// required policy pack whose mandatory "always-fails" policy fails for every resource and, when stackPolicy is
// set, once more for the stack as a whole. It returns the violation events and the operation's error.
func policyExceptionTest(
	t *testing.T, exceptions map[string]json.RawMessage, stackPolicy bool, dryRun bool,
) ([]PolicyViolationEventPayload, error) {
	t.Helper()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name: "analyzerA",
					Policies: []plugin.AnalyzerPolicyInfo{
						{Name: "always-fails", EnforcementLevel: apitype.Mandatory},
					},
				},
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					if stackPolicy || r.Type != "pkgA:m:typA" {
						return plugin.AnalyzeResponse{}, nil
					}
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Mandatory,
					}}}, nil
				},
				AnalyzeStackF: func(rs []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
					if !stackPolicy {
						return plugin.AnalyzeResponse{}, nil
					}
					// A stack-level violation that isn't tied to a resource is attributed to the root stack.
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Message:          "a stack policy failed",
						EnforcementLevel: apitype.Mandatory,
					}}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		if err != nil {
			return err
		}
		_, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		return err
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Project: "web",
		Stack:   "prod",
		Options: lt.TestUpdateOptions{
			T:                t,
			SkipDisplayTests: true,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{&testRequiredPolicy{name: "analyzerA", exceptions: exceptions}},
			},
			HostF: hostF,
		},
	}

	var violations []PolicyViolationEventPayload
	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
		for _, e := range events {
			if e.Type == PolicyViolationEvent {
				violations = append(violations, e.Payload().(PolicyViolationEventPayload))
			}
		}
		return err
	}

	_, err := lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, dryRun, p.BackendClient, validate)
	return violations, err
}

func exceptionsJSON(t *testing.T, js string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(js), &m))
	return m
}

const (
	resAURN = resource.URN("urn:pulumi:prod::web::pkgA:m:typA::resA")
	resBURN = resource.URN("urn:pulumi:prod::web::pkgA:m:typA::resB")
)

func TestPolicyExceptionMandatoryViolationWithoutException(t *testing.T) {
	t.Parallel()

	for _, dryRun := range []bool{false, true} {
		violations, err := policyExceptionTest(t, nil, false, dryRun)
		assert.Error(t, err)
		require.NotEmpty(t, violations)
		for _, v := range violations {
			assert.Nil(t, v.Exception)
		}
	}
}

// Excepting every violating resource lets the update and the preview succeed, and the violations are still
// reported with the exception that matched them.
func TestPolicyExceptionExceptsResourceViolations(t *testing.T) {
	t.Parallel()

	exceptions := exceptionsJSON(t, `{
		"EXC-1": {"policies": ["always-fails"], "resources": [{"urn": "`+string(resAURN)+`"}], "reason": "resA is special"},
		"EXC-2": {"policies": ["always-fails"], "stacks": ["web/prod"], "reason": "the whole stack"}
	}`)
	for _, dryRun := range []bool{false, true} {
		violations, err := policyExceptionTest(t, exceptions, false, dryRun)
		require.NoError(t, err)

		got := map[resource.URN]*apitype.PolicyEventException{}
		for _, v := range violations {
			assert.Equal(t, apitype.Mandatory, v.EnforcementLevel)
			got[v.ResourceURN] = v.Exception
		}
		// When both exceptions match, the lowest ID wins.
		assert.Equal(t, map[resource.URN]*apitype.PolicyEventException{
			resAURN: {ID: "EXC-1", Reason: "resA is special"},
			resBURN: {ID: "EXC-2", Reason: "the whole stack"},
		}, got)
	}
}

// One violation that isn't excepted still fails the update.
func TestPolicyExceptionPartialCoverageStillFails(t *testing.T) {
	t.Parallel()

	exceptions := exceptionsJSON(t, `{
		"EXC-1": {"policies": ["always-fails"], "resources": [{"urn": "`+string(resAURN)+`"}], "reason": "resA is special"}
	}`)
	violations, err := policyExceptionTest(t, exceptions, false, false)
	assert.Error(t, err)

	excepted := map[resource.URN]bool{}
	for _, v := range violations {
		excepted[v.ResourceURN] = v.Exception != nil
	}
	assert.True(t, excepted[resAURN])
	assert.False(t, excepted[resBURN])
}

// Exceptions only apply to the policies they name, and to stacks they name.
func TestPolicyExceptionOtherPolicyOrStackDoesNotMatch(t *testing.T) {
	t.Parallel()

	exceptions := exceptionsJSON(t, `{
		"other-policy": {"policies": ["something-else"], "stacks": ["web/prod"], "reason": "r"},
		"other-stack": {"policies": ["always-fails"], "stacks": ["web/dev"], "reason": "r"}
	}`)
	_, err := policyExceptionTest(t, exceptions, false, false)
	assert.Error(t, err)
}

// A stack policy violation that isn't tied to a resource is attributed to the root stack URN, which a "stacks"
// target or the root stack URN itself covers.
func TestPolicyExceptionExceptsStackViolation(t *testing.T) {
	t.Parallel()

	rootStackURN := resource.DefaultRootStackURN("prod", "web")
	for _, target := range []string{
		`"stacks": ["web/prod"]`,
		`"resources": [{"urn": "` + string(rootStackURN) + `"}]`,
	} {
		exceptions := exceptionsJSON(t, `{"EXC-1": {"policies": ["always-fails"], `+target+`, "reason": "r"}}`)
		violations, err := policyExceptionTest(t, exceptions, true, false)
		require.NoError(t, err, target)
		require.Len(t, violations, 1)
		assert.Equal(t, rootStackURN, violations[0].ResourceURN)
		assert.Equal(t, &apitype.PolicyEventException{ID: "EXC-1", Reason: "r"}, violations[0].Exception)
	}

	_, err := policyExceptionTest(t, nil, true, false)
	assert.Error(t, err)
}

// Malformed exceptions from the service are ignored rather than failing the update, so the policy blocks.
func TestPolicyExceptionMalformedFromServiceIgnored(t *testing.T) {
	t.Parallel()

	exceptions := exceptionsJSON(t, `{
		"no-reason": {"policies": ["always-fails"], "stacks": ["web/prod"]},
		"unknown-field": {"policies": ["always-fails"], "stacks": ["web/prod"], "reason": "r", "tags": ["x"]}
	}`)
	violations, err := policyExceptionTest(t, exceptions, false, false)
	assert.Error(t, err)
	for _, v := range violations {
		assert.Nil(t, v.Exception)
	}
}

// propertyExceptionTest runs an update of a program that registers resA (env: dev) and resB (env: prod). The
// provider adds an output-only "arn". The pack's resource policy and stack policy each fail for resA only; the stack
// policy reports its violation against resA's URN. It returns the violations and the update's error.
func propertyExceptionTest(t *testing.T, exceptions map[string]json.RawMessage) ([]PolicyViolationEventPayload, error) {
	t.Helper()

	failsForDev := func(env property.Value) []plugin.AnalyzeDiagnostic {
		if !env.IsString() || env.AsString() != "dev" {
			return nil
		}
		return []plugin.AnalyzeDiagnostic{{
			PolicyName: "res-policy", PolicyPackName: "analyzerA", Message: "failed", EnforcementLevel: apitype.Mandatory,
		}}
	}
	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					outputs := req.Properties.Set("arn", property.New("arn:"+req.URN.Name()))
					return plugin.CreateResponse{ID: "id", Properties: outputs, Status: resource.StatusOK}, nil
				},
			}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name: "analyzerA",
					Policies: []plugin.AnalyzerPolicyInfo{
						{Name: "res-policy", EnforcementLevel: apitype.Mandatory},
						{Name: "stack-policy", EnforcementLevel: apitype.Mandatory},
					},
				},
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					return plugin.AnalyzeResponse{Diagnostics: failsForDev(r.Properties.Get("env"))}, nil
				},
				AnalyzeStackF: func(rs []plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
					var diags []plugin.AnalyzeDiagnostic
					for _, r := range rs {
						for _, d := range failsForDev(r.Properties.Get("env")) {
							d.PolicyName, d.URN = "stack-policy", r.URN
							diags = append(diags, d)
						}
					}
					return plugin.AnalyzeResponse{Diagnostics: diags}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		for name, env := range map[string]string{"resA": "dev", "resB": "prod"} {
			_, err := monitor.RegisterResource("pkgA:m:typA", name, true, deploytest.ResourceOptions{
				Inputs: resource.PropertyMap{"env": resource.NewProperty(env)},
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		Project: "web",
		Stack:   "prod",
		Options: lt.TestUpdateOptions{
			T:                t,
			SkipDisplayTests: true,
			UpdateOptions: UpdateOptions{
				RequiredPolicies: []RequiredPolicy{&testRequiredPolicy{name: "analyzerA", exceptions: exceptions}},
			},
			HostF: hostF,
		},
	}

	var violations []PolicyViolationEventPayload
	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
		for _, e := range events {
			if e.Type == PolicyViolationEvent {
				violations = append(violations, e.Payload().(PolicyViolationEventPayload))
			}
		}
		return err
	}
	_, err := lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
	return violations, err
}

// A resource selector's properties are matched against what the policy saw: inputs for a resource policy, and
// outputs for a stack policy.
func TestPolicyExceptionMatchesProperties(t *testing.T) {
	t.Parallel()

	t.Run("inputs for resource policies, outputs for stack policies", func(t *testing.T) {
		t.Parallel()
		violations, err := propertyExceptionTest(t, exceptionsJSON(t, `{
			"by-input": {"policies": ["res-policy"], "resources": [{"properties": {"env": "dev"}}], "reason": "inputs"},
			"by-output": {"policies": ["stack-policy"], "resources": [{"properties": {"arn": "arn:res*"}}], "reason": "outputs"}
		}`))
		require.NoError(t, err)

		got := map[string]string{}
		for _, v := range violations {
			require.NotNil(t, v.Exception, "%s on %s", v.PolicyName, v.ResourceURN)
			got[v.PolicyName+" "+v.ResourceURN.Name()] = v.Exception.ID
		}
		assert.Equal(t, map[string]string{"res-policy resA": "by-input", "stack-policy resA": "by-output"}, got)
	})

	t.Run("an output-only property never matches a resource policy", func(t *testing.T) {
		t.Parallel()
		violations, err := propertyExceptionTest(t, exceptionsJSON(t, `{
			"by-output": {"policies": ["res-policy"], "resources": [{"properties": {"arn": "arn:resA"}}], "reason": "r"}
		}`))
		assert.Error(t, err)
		require.NotEmpty(t, violations)
		assert.Nil(t, violations[0].Exception)
	})
}
