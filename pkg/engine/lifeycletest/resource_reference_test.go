//nolint:goconst
package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// TestResourceReferences tests that resource references can be marshaled between the engine, language host,
// resource providers, and statefile if each entity supports resource references.
func TestResourceReferences(t *testing.T) {
	var urnA resource.URN
	var urnB resource.URN
	var idB resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap,
					timeout float64, preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					id := "created-id"
					if preview {
						id = ""
					}

					if urn.Name() == "resC" {
						assert.True(t, news.DeepEquals(resource.PropertyMap{
							"resA": resource.MakeComponentResourceReference(urnA, ""),
							"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
						}))
					}

					return resource.ID(id), news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			return v, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		urnA, _, _, err = monitor.RegisterResource("component", "resA", false)
		assert.NoError(t, err)

		err = monitor.RegisterResourceOutputs(urnA, resource.PropertyMap{})
		assert.NoError(t, err)

		urnB, idB, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		_, _, props, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"resA": resource.MakeComponentResourceReference(urnA, ""),
				"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
			},
		})
		assert.NoError(t, err)

		assert.True(t, props.DeepEquals(resource.PropertyMap{
			"resA": resource.MakeComponentResourceReference(urnA, ""),
			"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
		}))
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// TestResourceReferences_DownlevelSDK tests that resource references are properly marshaled as URNs (for references to
// component resources) or IDs (for references to custom resources) if the SDK does not support resource references.
func TestResourceReferences_DownlevelSDK(t *testing.T) {
	var urnA resource.URN
	var urnB resource.URN
	var idB resource.ID

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap,
					timeout float64, preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					id := "created-id"
					if preview {
						id = ""
					}

					state := resource.PropertyMap{}
					if urn.Name() == "resC" {
						state = resource.PropertyMap{
							"resA": resource.MakeComponentResourceReference(urnA, ""),
							"resB": resource.MakeCustomResourceReference(urnB, idB, ""),
						}
					}

					return resource.ID(id), state, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			return v, nil
		}),
	}

	opts := deploytest.ResourceOptions{DisableResourceReferences: true}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		urnA, _, _, err = monitor.RegisterResource("component", "resA", false, opts)
		assert.NoError(t, err)

		err = monitor.RegisterResourceOutputs(urnA, resource.PropertyMap{})
		assert.NoError(t, err)

		urnB, idB, _, err = monitor.RegisterResource("pkgA:m:typA", "resB", true, opts)
		assert.NoError(t, err)

		_, _, props, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, opts)
		assert.NoError(t, err)

		assert.Equal(t, resource.NewStringProperty(string(urnA)), props["resA"])
		if idB != "" {
			assert.Equal(t, resource.NewStringProperty(string(idB)), props["resB"])
		} else {
			assert.True(t, props["resB"].IsComputed())
		}
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}

// TestResourceReferences_DownlevelEngine tests an SDK that supports resource references communicating with an engine
// that does not.
func TestResourceReferences_DownlevelEngine(t *testing.T) {
	var urnA resource.URN
	var refB resource.PropertyValue

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			v := &deploytest.Provider{
				CreateF: func(urn resource.URN, news resource.PropertyMap,
					timeout float64, preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

					id := "created-id"
					if preview {
						id = ""
					}

					// If we have resource references here, the engine has not properly disabled them.
					if urn.Name() == "resC" {
						assert.Equal(t, resource.NewStringProperty(string(urnA)), news["resA"])
						assert.Equal(t, refB.ResourceReferenceValue().ID, news["resB"])
					}

					return resource.ID(id), news, resource.StatusOK, nil
				},
				ReadF: func(urn resource.URN, id resource.ID,
					inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {
					return plugin.ReadResult{Inputs: inputs, Outputs: state}, resource.StatusOK, nil
				},
			}
			return v, nil
		}),
	}

	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		var err error
		urnA, _, _, err = monitor.RegisterResource("component", "resA", false)
		assert.NoError(t, err)

		err = monitor.RegisterResourceOutputs(urnA, resource.PropertyMap{})
		assert.NoError(t, err)

		urnB, idB, _, err := monitor.RegisterResource("pkgA:m:typA", "resB", true)
		assert.NoError(t, err)

		refB = resource.MakeCustomResourceReference(urnB, idB, "")
		_, _, props, err := monitor.RegisterResource("pkgA:m:typA", "resC", true, deploytest.ResourceOptions{
			Inputs: resource.PropertyMap{
				"resA": resource.MakeComponentResourceReference(urnA, ""),
				"resB": refB,
			},
		})
		assert.NoError(t, err)

		assert.Equal(t, resource.NewStringProperty(string(urnA)), props["resA"])
		if refB.ResourceReferenceValue().ID.IsComputed() {
			assert.True(t, props["resB"].IsComputed())
		} else {
			assert.True(t, refB.ResourceReferenceValue().ID.DeepEquals(props["resB"]))
		}
		return nil
	})

	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host, DisableResourceReferences: true},
		Steps:   MakeBasicLifecycleSteps(t, 4),
	}
	p.Run(t, nil)
}
