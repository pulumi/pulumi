package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/stretchr/testify/assert"
)

func TestSimpleAnalyzer(t *testing.T) {
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
		assert.Error(t, err)
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{
			LocalPolicyPacks: []LocalPolicyPack{{Name: "analyzerA"}},
			Host:             host,
		},
	}

	project := p.GetProject()
	_, res := TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil)
	assert.NotNil(t, res)
}
