package lifecycletest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
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

func (p *testRequiredPolicy) Install(_ context.Context) (string, error) {
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
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{
			RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			Host:             host,
		},
	}

	project := p.GetProject()
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.Nil(t, res)
}

func TestSimpleAnalyzeResourceFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				AnalyzeF: func(r plugin.AnalyzerResource) ([]plugin.AnalyzeDiagnostic, error) {
					return []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Description:      "a policy that always fails",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Mandatory,
						URN:              r.URN,
					}}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.Error(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{
			RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			Host:             host,
		},
	}

	project := p.GetProject()
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
}

func TestSimpleAnalyzeStackFailure(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.PluginLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
		deploytest.NewAnalyzerLoader("analyzerA", func(_ *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
			return &deploytest.Analyzer{
				AnalyzeStackF: func(rs []plugin.AnalyzerStackResource) ([]plugin.AnalyzeDiagnostic, error) {
					return []plugin.AnalyzeDiagnostic{{
						PolicyName:       "always-fails",
						PolicyPackName:   "analyzerA",
						Description:      "a policy that always fails",
						Message:          "a policy failed",
						EnforcementLevel: apitype.Mandatory,
						URN:              rs[0].URN,
					}}, nil
				},
			}, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, _, _, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
		assert.NoError(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{
			RequiredPolicies: []RequiredPolicy{NewRequiredPolicy("analyzerA", "", nil)},
			Host:             host,
		},
	}

	project := p.GetProject()
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
}
