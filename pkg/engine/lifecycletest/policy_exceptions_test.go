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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// An exception in a local policy pack's config file allows a mandatory violation, on a resource or on the stack, so
// that it doesn't fail the update. Exceptions apply to packs that don't support config, too.
func TestLocalPolicyPackExceptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		supportsConfig bool
		policies       string
		err            string
	}{
		{name: "pack with config support", supportsConfig: true, policies: `{"p": "mandatory"}`},
		{name: "pack without config support", policies: `{}`},
		{
			name:     "policy config for a pack without config support",
			policies: `{"p": "mandatory"}`,
			err:      "does not support config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			analyzerPath, err := filepath.Abs("test")
			require.NoError(t, err)
			violation := plugin.AnalyzeDiagnostic{
				PolicyName:       "p",
				PolicyPackName:   "pack",
				Message:          "a policy failed",
				EnforcementLevel: apitype.Mandatory,
			}
			loaders := []*deploytest.PluginLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{}, nil
				}),
				deploytest.NewAnalyzerLoader(analyzerPath, func(*plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
					return &deploytest.Analyzer{
						Info: plugin.AnalyzerInfo{
							Name:           "pack",
							Policies:       []plugin.AnalyzerPolicyInfo{{Name: "p", EnforcementLevel: apitype.Mandatory}},
							SupportsConfig: tt.supportsConfig,
						},
						AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
							if r.Type != "pkgA:m:typA" {
								return plugin.AnalyzeResponse{}, nil
							}
							return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{violation}}, nil
						},
						// The stack violation has no URN, so the engine attributes it to the root stack.
						AnalyzeStackF: func([]plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
							return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{violation}}, nil
						},
					}, nil
				}, deploytest.WithGrpc),
			}
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
				return err
			})

			p := &lt.TestPlan{}
			resURN := p.NewURN("pkgA:m:typA", "resA", "")
			rootURN := resource.DefaultRootStackURN(p.GetTarget(t, nil).Name.Q(), p.GetProject().Name)
			configPath := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(configPath, []byte(`{"version": 2, "policies": `+tt.policies+`,
				"exceptions": {"EXC-1": {"reason": "r", "match": [
					{"policies": ["p"], "targets": ["`+string(resURN)+`", "`+string(rootURN)+`"]}
				]}}}`), 0o600))
			p.Options = lt.TestUpdateOptions{
				T:                t,
				SkipDisplayTests: true,
				HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
				UpdateOptions: UpdateOptions{
					LocalPolicyPacks: []LocalPolicyPack{{Path: "test", Config: configPath}},
				},
			}

			validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
				if tt.err != "" {
					return err
				}
				var urns []resource.URN
				for _, e := range events {
					if v, ok := e.Payload().(PolicyViolationEventPayload); ok {
						urns = append(urns, v.ResourceURN)
						assert.Equal(t, []apitype.PolicyEventException{{ID: "EXC-1", Reason: "r"}}, v.Exceptions)
					}
				}
				assert.ElementsMatch(t, []resource.URN{resURN, rootURN}, urns)
				return err
			}
			_, err = lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
			if tt.err != "" {
				assert.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// policyExceptionTest runs an update, or a preview, of a program that registers resA and resB against a required
// policy pack, analyzerA, whose mandatory "always-fails" policy fails for resA and resB or, with stackPolicy, once
// for the stack as a whole.
type policyExceptionTest struct {
	exceptions     map[string]json.RawMessage
	stackPolicy    bool
	supportsConfig bool
	dryRun         bool
}

type policyExceptionResult struct {
	violations map[resource.URN][]apitype.PolicyEventException
	warnings   []string
	err        error
}

func (tt policyExceptionTest) run(t *testing.T, p *lt.TestPlan) policyExceptionResult {
	t.Helper()

	violation := plugin.AnalyzeDiagnostic{
		PolicyName:       "always-fails",
		PolicyPackName:   "analyzerA",
		Message:          "a policy failed",
		EnforcementLevel: apitype.Mandatory,
	}
	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(*plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name: "analyzerA",
					Policies: []plugin.AnalyzerPolicyInfo{
						{Name: "always-fails", EnforcementLevel: apitype.Mandatory},
					},
					SupportsConfig: tt.supportsConfig,
				},
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					if tt.stackPolicy || r.Type != "pkgA:m:typA" {
						return plugin.AnalyzeResponse{}, nil
					}
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{violation}}, nil
				},
				// The stack violation has no URN, so the engine attributes it to the root stack.
				AnalyzeStackF: func([]plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
					if !tt.stackPolicy {
						return plugin.AnalyzeResponse{}, nil
					}
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{violation}}, nil
				},
			}, nil
		}, deploytest.WithGrpc),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true); err != nil {
			return err
		}
		_, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		return err
	})
	p.Options = lt.TestUpdateOptions{
		T:                t,
		SkipDisplayTests: true,
		HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
		UpdateOptions: UpdateOptions{
			RequiredPolicies: []RequiredPolicy{&testRequiredPolicy{name: "analyzerA", exceptions: tt.exceptions}},
		},
	}

	result := policyExceptionResult{violations: map[resource.URN][]apitype.PolicyEventException{}}
	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
		for _, e := range events {
			switch payload := e.Payload().(type) {
			case PolicyViolationEventPayload:
				assert.Equal(t, apitype.Mandatory, payload.EnforcementLevel)
				result.violations[payload.ResourceURN] = payload.Exceptions
			case DiagEventPayload:
				if payload.Severity == diag.Warning {
					result.warnings = append(result.warnings, colors.Never.Colorize(payload.Message))
				}
			}
		}
		return err
	}
	_, result.err = lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, tt.dryRun, p.BackendClient,
		validate)
	return result
}

func exceptionsJSON(t *testing.T, js string) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(js), &m))
	return m
}

// matchEntry returns a match entry for the "always-fails" policy on urn.
func matchEntry(urn resource.URN) string {
	return `{"policies": ["always-fails"], "targets": ["` + string(urn) + `"]}`
}

// Without exceptions from the service, violations are reported without exceptions and fail the operation. An update
// stops at the first one; a preview reports them all.
func TestPolicyExceptionMandatoryViolationWithoutException(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")

	result := policyExceptionTest{}.run(t, p)
	require.Error(t, result.err)
	assert.Equal(t, map[resource.URN][]apitype.PolicyEventException{resA: nil}, result.violations)

	result = policyExceptionTest{dryRun: true}.run(t, p)
	require.Error(t, result.err)
	assert.Equal(t, map[resource.URN][]apitype.PolicyEventException{resA: nil, resB: nil}, result.violations)
}

// Excepting every violating resource lets the update and the preview succeed, and the violations are still
// reported with the exceptions that matched them. This holds whether or not the pack supports config.
func TestPolicyExceptionExceptsResourceViolations(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")
	exceptions := exceptionsJSON(t, `{
		"EXC-1": {"reason": "resA is special", "match": [`+matchEntry(resA)+`]},
		"EXC-2": {"reason": "both", "match": [`+matchEntry(resA)+`, `+matchEntry(resB)+`]}
	}`)
	for _, supportsConfig := range []bool{false, true} {
		for _, dryRun := range []bool{false, true} {
			tt := policyExceptionTest{exceptions: exceptions, supportsConfig: supportsConfig, dryRun: dryRun}
			result := tt.run(t, p)
			require.NoError(t, result.err)
			assert.Equal(t, map[resource.URN][]apitype.PolicyEventException{
				resA: {{ID: "EXC-1", Reason: "resA is special"}, {ID: "EXC-2", Reason: "both"}},
				resB: {{ID: "EXC-2", Reason: "both"}},
			}, result.violations)
		}
	}
}

// A stack policy violation is attributed to the root stack URN, which an exception can name.
func TestPolicyExceptionExceptsStackViolation(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	root := resource.DefaultRootStackURN(p.GetTarget(t, nil).Name.Q(), p.GetProject().Name)
	exceptions := exceptionsJSON(t, `{"EXC-1": {"reason": "r", "match": [`+matchEntry(root)+`]}}`)
	result := policyExceptionTest{exceptions: exceptions, stackPolicy: true}.run(t, p)
	require.NoError(t, result.err)
	assert.Equal(t, map[resource.URN][]apitype.PolicyEventException{
		root: {{ID: "EXC-1", Reason: "r"}},
	}, result.violations)
}

// Invalid exceptions from the service are skipped with a warning rather than failing the update, so the violations
// they match still fail it. The valid exceptions still apply, and an exception naming a policy the pack doesn't have
// isn't a warning.
func TestPolicyExceptionMalformedFromServiceIgnored(t *testing.T) {
	t.Parallel()

	p := &lt.TestPlan{}
	resA := p.NewURN("pkgA:m:typA", "resA", "")
	resB := p.NewURN("pkgA:m:typA", "resB", "")
	exceptions := exceptionsJSON(t, `{
		"valid": {"reason": "r", "match": [
			`+matchEntry(resA)+`, {"policies": ["gone"], "targets": ["`+string(resA)+`"]}
		]},
		"no-reason": {"match": [`+matchEntry(resB)+`]},
		"all-policies": {"reason": "r", "match": [{"policies": ["all"], "targets": ["`+string(resB)+`"]}]}
	}`)
	result := policyExceptionTest{exceptions: exceptions}.run(t, p)
	require.Error(t, result.err)
	assert.Equal(t, map[resource.URN][]apitype.PolicyEventException{
		resA: {{ID: "valid", Reason: "r"}},
		resB: nil,
	}, result.violations)
	assert.ElementsMatch(t, []string{
		`policy pack "analyzerA": policy exception "all-policies": match[0]: "policies" must name policies; ` +
			`"all" isn't allowed; ignoring it` + "\n",
		`policy pack "analyzerA": policy exception "no-reason": "reason" is required; ignoring it` + "\n",
	}, result.warnings)
}

// A local pack with the same name as a required pack has its own exceptions: the local pack's exceptions don't apply
// to the required pack's violations, and the required pack's don't apply to the local pack's.
func TestPolicyExceptionsSameNamePacks(t *testing.T) {
	t.Parallel()

	localPath, err := filepath.Abs("local")
	require.NoError(t, err)
	analyzer := func(message string) func(*plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
		return func(*plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				Info: plugin.AnalyzerInfo{
					Name:     "analyzerA",
					Policies: []plugin.AnalyzerPolicyInfo{{Name: "always-fails", EnforcementLevel: apitype.Mandatory}},
				},
				AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
					if r.Type != "pkgA:m:typA" {
						return plugin.AnalyzeResponse{}, nil
					}
					return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Message:          message,
						EnforcementLevel: apitype.Mandatory,
					}}}, nil
				},
			}, nil
		}
	}
	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", analyzer("required"), deploytest.WithGrpc),
		deploytest.NewAnalyzerLoader(localPath, analyzer("local"), deploytest.WithGrpc),
	}
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		return err
	})

	p := &lt.TestPlan{}
	resA := p.NewURN("pkgA:m:typA", "resA", "")
	configPath := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"version": 2, "exceptions": {"local": {"reason": "r", "match": [
		`+matchEntry(resA)+`
	]}}}`), 0o600))
	p.Options = lt.TestUpdateOptions{
		T:                t,
		SkipDisplayTests: true,
		HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
		UpdateOptions: UpdateOptions{
			RequiredPolicies: []RequiredPolicy{&testRequiredPolicy{name: "analyzerA", exceptions: exceptionsJSON(t, `{
				"cloud": {"reason": "r", "match": [{"policies": ["always-fails"], "targets": ["urn:pulumi:other"]}]}
			}`)}},
			LocalPolicyPacks: []LocalPolicyPack{{Path: "local", Config: configPath}},
		},
	}

	exceptions := map[string][]apitype.PolicyEventException{}
	validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
		for _, e := range events {
			if v, ok := e.Payload().(PolicyViolationEventPayload); ok {
				exceptions[strings.TrimSpace(colors.Never.Colorize(v.Message))] = v.Exceptions
			}
		}
		return err
	}
	_, err = lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, true, p.BackendClient, validate)
	require.Error(t, err)
	assert.Equal(t, map[string][]apitype.PolicyEventException{
		"required": nil,
		"local":    {{ID: "local", Reason: "r"}},
	}, exceptions)
}
