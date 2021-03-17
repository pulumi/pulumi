package lifecycletest

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestDestroyWithPendingDelete(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}
	program := deploytest.NewLanguageRuntime(func(_ plugin.RunInfo, _ *deploytest.ResourceMonitor) error {
		return nil
	})
	host := deploytest.NewPluginHost(nil, nil, program, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []TestStep{{
		Op: Update,
		Validate: func(_ workspace.Project, _ deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resource with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != JournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return res
		},
	}}
	p.Run(t, old)
}

func TestUpdateWithPendingDelete(t *testing.T) {
	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{}, nil
		}),
	}

	host := deploytest.NewPluginHost(nil, nil, nil, loaders...)

	p := &TestPlan{
		Options: UpdateOptions{Host: host},
	}

	resURN := p.NewURN("pkgA:m:typA", "resA", "")

	// Create an old snapshot with two copies of a resource that share a URN: one that is pending deletion and one
	// that is not.
	old := &deploy.Snapshot{
		Resources: []*resource.State{
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "1",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			},
			{
				Type:    resURN.Type(),
				URN:     resURN,
				Custom:  true,
				ID:      "0",
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
				Delete:  true,
			},
		},
	}

	p.Steps = []TestStep{{
		Op: Destroy,
		Validate: func(_ workspace.Project, _ deploy.Target, entries JournalEntries,
			_ []Event, res result.Result) result.Result {

			// Verify that we see a DeleteReplacement for the resource with ID 0 and a Delete for the resource with
			// ID 1.
			deletedID0, deletedID1 := false, false
			for _, entry := range entries {
				// Ignore non-terminal steps and steps that affect the injected default provider.
				if entry.Kind != JournalEntrySuccess || entry.Step.URN() != resURN ||
					(entry.Step.Op() != deploy.OpDelete && entry.Step.Op() != deploy.OpDeleteReplaced) {
					continue
				}

				switch id := entry.Step.Old().ID; id {
				case "0":
					assert.False(t, deletedID0)
					deletedID0 = true
				case "1":
					assert.False(t, deletedID1)
					deletedID1 = true
				default:
					assert.Fail(t, "unexpected resource ID %v", string(id))
				}
			}
			assert.True(t, deletedID0)
			assert.True(t, deletedID1)

			return res
		},
	}}
	p.Run(t, old)
}
