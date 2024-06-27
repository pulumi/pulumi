package lifecycletest

import (
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine" //nolint:revive
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

func TestRetainOnDelete(t *testing.T) {
	t.Parallel()

	idCounter := 0

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				DiffF: func(
					urn resource.URN,
					id resource.ID,
					oldInputs, oldOutputs, newInputs resource.PropertyMap,
					ignoreChanges []string,
				) (plugin.DiffResult, error) {
					if !oldOutputs["foo"].DeepEquals(newInputs["foo"]) {
						// If foo changes do a replace, we use this to check we don't delete on replace
						return plugin.DiffResult{
							Changes:     plugin.DiffSome,
							ReplaceKeys: []resource.PropertyKey{"foo"},
						}, nil
					}
					return plugin.DiffResult{}, nil
				},
				CreateF: func(urn resource.URN, news resource.PropertyMap, timeout float64,
					preview bool,
				) (resource.ID, resource.PropertyMap, resource.Status, error) {
					resourceID := resource.ID(fmt.Sprintf("created-id-%d", idCounter))
					idCounter = idCounter + 1
					return resourceID, news, resource.StatusOK, nil
				},
				DeleteF: func(urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap,
					timeout float64,
				) (resource.Status, error) {
					assert.Fail(t, "Delete was called")
					return resource.StatusOK, nil
				},
			}, nil
		}, deploytest.WithoutGrpc),
	}

	ins := resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "bar",
	})

	createResource := true

	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		if createResource {
			_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
				Inputs:         ins,
				RetainOnDelete: true,
			})
			assert.NoError(t, err)
		}

		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, loaders...)

	p := &TestPlan{Options: TestUpdateOptions{T: t, HostF: hostF}}

	project := p.GetProject()

	// Run an update to create the resource
	snap, err := TestOp(Update).RunStep(project, p.GetTarget(t, nil), p.Options, false, p.BackendClient, nil, "0")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-0", snap.Resources[1].ID.String())

	// Run a new update which will cause a replace, we shouldn't see a provider delete but should get a new id
	ins = resource.NewPropertyMapFromMap(map[string]interface{}{
		"foo": "baz",
	})
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "1")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 2)
	assert.Equal(t, "created-id-1", snap.Resources[1].ID.String())

	// Run a new update which will cause a delete, we still shouldn't see a provider delete
	createResource = false
	snap, err = TestOp(Update).RunStep(project, p.GetTarget(t, snap), p.Options, false, p.BackendClient, nil, "2")
	assert.NoError(t, err)
	assert.NotNil(t, snap)
	assert.Len(t, snap.Resources, 0)
}
