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

package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func TestValidateStateMigrationContext(t *testing.T) {
	t.Parallel()

	const (
		rootURN = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		oldURN  = resource.URN("urn:pulumi:test::test::pkg:m:Resource::old")
		newURN  = resource.URN("urn:pulumi:test::test::pkg:m:Resource::new")
	)
	successors := map[resource.URN]resource.URN{oldURN: newURN}

	t.Run("clean full update", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateStateMigrationContext(rootURN, &Options{}, &Snapshot{}, successors))
	})

	for name, opts := range map[string]*Options{
		"target": {
			Targets: NewUrnTargets([]string{string(rootURN)}),
		},
		"exclude": {
			Excludes: NewUrnTargets([]string{string(rootURN)}),
		},
		"replace target": {
			ReplaceTargets: NewUrnTargets([]string{string(rootURN)}),
		},
		"target snippet": {
			TargetSnippets: []string{"snippet-id"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := validateStateMigrationContext(rootURN, opts, &Snapshot{}, successors)
			require.ErrorContains(t, err, "cannot change state during a targeted or excluded update")
		})
	}

	t.Run("pending operation", func(t *testing.T) {
		t.Parallel()
		snap := &Snapshot{PendingOperations: []pkgresource.Operation{pkgresource.NewOperation(
			&pkgresource.State{URN: "urn:pulumi:test::test::pkg:m:Resource::pending"},
			pkgresource.OperationTypeCreating,
		)}}
		err := validateStateMigrationContext(rootURN, &Options{}, snap, successors)
		require.ErrorContains(t, err, "snapshot has 1 pending operation")
	})

	t.Run("snippet reference", func(t *testing.T) {
		t.Parallel()
		snap := &Snapshot{Snippets: []resource.Snippet{{
			UUID:       "snippet-id",
			References: map[string]string{"dependency": string(oldURN)},
		}}}
		err := validateStateMigrationContext(rootURN, &Options{}, snap, successors)
		require.ErrorContains(t, err, `cannot rewrite snippet "snippet-id" reference "dependency"`)
	})

	t.Run("unrelated snippet reference", func(t *testing.T) {
		t.Parallel()
		snap := &Snapshot{Snippets: []resource.Snippet{{
			UUID: "snippet-id",
			References: map[string]string{
				"dependency": "urn:pulumi:test::test::pkg:m:Resource::unrelated",
			},
		}}}
		require.NoError(t, validateStateMigrationContext(rootURN, &Options{}, snap, successors))
	})
}

func TestValidateStateMigrationManagedIdentity(t *testing.T) {
	t.Parallel()

	const (
		rootURN  = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		oldURN   = resource.URN("urn:pulumi:test::test::pkg:m:Resource::old")
		newURN   = resource.URN("urn:pulumi:test::test::pkg:m:Resource::new")
		otherURN = resource.URN("urn:pulumi:test::test::pkg:m:Resource::other")
	)
	state := func(urn resource.URN, custom, pendingReplacement bool, id ...resource.ID) apitype.ResourceV3 {
		resourceID := resource.ID("id")
		if len(id) > 0 {
			resourceID = id[0]
		}
		return apitype.ResourceV3{
			URN:                urn,
			Type:               urn.Type(),
			Custom:             custom,
			ID:                 resourceID,
			PendingReplacement: pendingReplacement,
		}
	}

	t.Run("renamed successor preserves flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true /* custom */, true /* pendingReplacement */)},
			[]apitype.ResourceV3{state(newURN, true /* custom */, true /* pendingReplacement */)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.NoError(t, err)
	})

	t.Run("cannot clear flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true /* custom */, true /* pendingReplacement */)},
			[]apitype.ResourceV3{state(newURN, true /* custom */, false /* pendingReplacement */)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes PendingReplacement")
	})

	t.Run("cannot forge flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true /* custom */, false /* pendingReplacement */)},
			[]apitype.ResourceV3{state(newURN, true /* custom */, true /* pendingReplacement */)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes PendingReplacement")
	})

	t.Run("new resource cannot invent flag", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(rootURN, false /* custom */, false /* pendingReplacement */)},
			[]apitype.ResourceV3{
				state(rootURN, false /* custom */, false /* pendingReplacement */),
				state(newURN, true /* custom */, true /* pendingReplacement */),
			},
			nil,
		)
		require.ErrorContains(t, err, "without a pending-replacement custom predecessor")
	})

	t.Run("cannot clear taint", func(t *testing.T) {
		t.Parallel()
		old := state(oldURN, true /* custom */, false /* pendingReplacement */)
		old.Taint = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{old},
			[]apitype.ResourceV3{state(newURN, true /* custom */, false /* pendingReplacement */)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes Taint")
	})

	t.Run("cannot forge taint", func(t *testing.T) {
		t.Parallel()
		successor := state(newURN, true /* custom */, false /* pendingReplacement */)
		successor.Taint = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true /* custom */, false /* pendingReplacement */)},
			[]apitype.ResourceV3{successor},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes Taint")
	})

	t.Run("new resource cannot invent taint", func(t *testing.T) {
		t.Parallel()
		added := state(newURN, true /* custom */, false /* pendingReplacement */)
		added.Taint = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(rootURN, false /* custom */, false /* pendingReplacement */)},
			[]apitype.ResourceV3{
				state(rootURN, false /* custom */, false /* pendingReplacement */),
				added,
			},
			nil,
		)
		require.ErrorContains(t, err, "without a tainted custom predecessor")
	})

	t.Run("component does not constrain custom successor", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{
				state(rootURN, false /* custom */, false /* pendingReplacement */),
				state(oldURN, true /* custom */, true /* pendingReplacement */),
			},
			[]apitype.ResourceV3{state(newURN, true /* custom */, true /* pendingReplacement */)},
			map[resource.URN]resource.URN{
				rootURN: newURN,
				oldURN:  newURN,
			},
		)
		require.NoError(t, err)
	})

	t.Run("many-to-one successor conservatively inherits safety flags", func(t *testing.T) {
		t.Parallel()
		protected := state(oldURN, true /* custom */, false /* pendingReplacement */)
		protected.Protect = true
		retained := state(otherURN, true /* custom */, false /* pendingReplacement */)
		retained.RetainOnDelete = true
		successor := state(newURN, true /* custom */, false /* pendingReplacement */)
		successor.Protect = true
		successor.RetainOnDelete = true

		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{protected, retained},
			[]apitype.ResourceV3{successor},
			map[resource.URN]resource.URN{oldURN: newURN, otherURN: newURN},
		)
		require.NoError(t, err)
	})

	for _, flag := range []struct {
		name      string
		set       func(*apitype.ResourceV3, bool)
		wantError string
	}{
		{
			name:      "Protect",
			set:       func(state *apitype.ResourceV3, value bool) { state.Protect = value },
			wantError: "changes Protect",
		},
		{
			name:      "RetainOnDelete",
			set:       func(state *apitype.ResourceV3, value bool) { state.RetainOnDelete = value },
			wantError: "changes RetainOnDelete",
		},
	} {
		t.Run("cannot clear "+flag.name, func(t *testing.T) {
			t.Parallel()
			old := state(oldURN, true /* custom */, false /* pendingReplacement */)
			flag.set(&old, true)
			successor := state(newURN, true /* custom */, false /* pendingReplacement */)
			err := validateStateMigrationManagedIdentity(
				rootURN,
				[]apitype.ResourceV3{old},
				[]apitype.ResourceV3{successor},
				map[resource.URN]resource.URN{oldURN: newURN},
			)
			require.ErrorContains(t, err, flag.wantError)
		})

		t.Run("cannot forge "+flag.name, func(t *testing.T) {
			t.Parallel()
			old := state(oldURN, true /* custom */, false /* pendingReplacement */)
			successor := state(newURN, true /* custom */, false /* pendingReplacement */)
			flag.set(&successor, true)
			err := validateStateMigrationManagedIdentity(
				rootURN,
				[]apitype.ResourceV3{old},
				[]apitype.ResourceV3{successor},
				map[resource.URN]resource.URN{oldURN: newURN},
			)
			require.ErrorContains(t, err, flag.wantError)
		})
	}

	t.Run("introduced custom resource requires managed predecessor", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(rootURN, false /* custom */, false /* pendingReplacement */)},
			[]apitype.ResourceV3{
				state(rootURN, false /* custom */, false /* pendingReplacement */),
				state(newURN, true /* custom */, false /* pendingReplacement */),
			},
			nil,
		)
		require.ErrorContains(t, err, "without a managed custom predecessor")
	})

	t.Run("component predecessor alone cannot authorize custom successor", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(rootURN, false /* custom */, false /* pendingReplacement */)},
			[]apitype.ResourceV3{state(newURN, true /* custom */, false /* pendingReplacement */)},
			map[resource.URN]resource.URN{rootURN: newURN},
		)
		require.ErrorContains(t, err, "without a managed custom predecessor")
	})

	t.Run("cannot change physical ID", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true /* custom */, false /* pendingReplacement */, "old-id")},
			[]apitype.ResourceV3{state(newURN, true /* custom */, false /* pendingReplacement */, "new-id")},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes the physical ID")
	})

	t.Run("cannot change extension reference", func(t *testing.T) {
		t.Parallel()
		old := state(oldURN, true /* custom */, false /* pendingReplacement */)
		old.ExtensionRef = "old-extension"
		successor := state(newURN, true /* custom */, false /* pendingReplacement */)
		successor.ExtensionRef = "new-extension"
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{old},
			[]apitype.ResourceV3{successor},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes the extension reference")
	})

	t.Run("cannot change ownership", func(t *testing.T) {
		t.Parallel()
		old := state(oldURN, true /* custom */, false /* pendingReplacement */)
		successor := state(newURN, true /* custom */, false /* pendingReplacement */)
		successor.External = true
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{old},
			[]apitype.ResourceV3{successor},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "changes ownership")
	})

	t.Run("cannot map custom resource to component", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{state(oldURN, true /* custom */, false /* pendingReplacement */)},
			[]apitype.ResourceV3{state(newURN, false /* custom */, false /* pendingReplacement */)},
			map[resource.URN]resource.URN{oldURN: newURN},
		)
		require.ErrorContains(t, err, "maps managed custom resource")
	})

	t.Run("cannot merge distinct managed IDs", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationManagedIdentity(
			rootURN,
			[]apitype.ResourceV3{
				state(oldURN, true /* custom */, false /* pendingReplacement */, "old-id"),
				state(newURN, true /* custom */, false /* pendingReplacement */, "new-id"),
			},
			[]apitype.ResourceV3{state(rootURN, true /* custom */, false /* pendingReplacement */, "old-id")},
			map[resource.URN]resource.URN{
				oldURN: rootURN,
				newURN: rootURN,
			},
		)
		require.ErrorContains(t, err, "changes the physical ID")
	})
}

func TestValidateStateMigrationProviderStates(t *testing.T) {
	t.Parallel()

	const rootURN = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
	providerURN := resource.NewURN("test", "test", "", "pulumi:providers:pkg", "default")
	provider := apitype.ResourceV3{
		URN:    providerURN,
		Type:   providerURN.Type(),
		Custom: true,
		ID:     "provider-id",
		Inputs: map[string]any{"region": "us-west-2"},
	}

	t.Run("unchanged", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, validateStateMigrationProviderStates(
			rootURN, []apitype.ResourceV3{provider}, []apitype.ResourceV3{provider}))
	})

	t.Run("removed or renamed", func(t *testing.T) {
		t.Parallel()
		renamed := provider
		renamed.URN = resource.NewURN("test", "test", "", "pulumi:providers:pkg", "renamed")
		err := validateStateMigrationProviderStates(
			rootURN, []apitype.ResourceV3{provider}, []apitype.ResourceV3{renamed})
		require.ErrorContains(t, err, "removes or renames provider state")
	})

	t.Run("reconfigured", func(t *testing.T) {
		t.Parallel()
		reconfigured := provider
		reconfigured.Inputs = map[string]any{"region": "eu-west-1"}
		err := validateStateMigrationProviderStates(
			rootURN, []apitype.ResourceV3{provider}, []apitype.ResourceV3{reconfigured})
		require.ErrorContains(t, err, "changes provider state")
	})

	t.Run("introduced", func(t *testing.T) {
		t.Parallel()
		err := validateStateMigrationProviderStates(rootURN, nil, []apitype.ResourceV3{provider})
		require.ErrorContains(t, err, "introduces provider state")
	})
}

func TestValidateMigratedStatesRejectsAlreadyRegisteredURN(t *testing.T) {
	t.Parallel()

	const (
		rootURN      = resource.URN("urn:pulumi:test::test::pkg:m:Component::component")
		collisionURN = resource.URN("urn:pulumi:test::test::pkg:m:Component$pkg:m:Component::collision")
	)
	root := &pkgresource.State{URN: rootURN, Type: rootURN.Type()}
	collision := &pkgresource.State{
		URN:    collisionURN,
		Type:   collisionURN.Type(),
		Parent: rootURN,
	}
	sg := &stepGenerator{
		deployment: &Deployment{olds: map[resource.URN]*pkgresource.State{rootURN: root}},
		urns: map[resource.URN]bool{
			rootURN:      true,
			collisionURN: true,
		},
		aliased: map[resource.URN]resource.URN{},
	}

	err := sg.validateStateMigrationResult(rootURN, root, []*pkgresource.State{root},
		[]*pkgresource.State{root, collision})
	require.ErrorContains(t, err, "was already registered earlier in this deployment")
}

func TestValidateStateMigrationResultRoot(t *testing.T) {
	t.Parallel()

	registrationURN := resource.NewURN("test", "test", "", "pkg:m:NewComponent", "component")
	priorURN := resource.NewURN("test", "test", "", "pkg:m:OldComponent", "component")
	priorRoot := &pkgresource.State{URN: priorURN, Type: priorURN.Type()}
	registrationRoot := &pkgresource.State{URN: registrationURN, Type: registrationURN.Type()}
	sg := &stepGenerator{
		deployment: &Deployment{olds: map[resource.URN]*pkgresource.State{priorURN: priorRoot}},
		aliased:    map[resource.URN]resource.URN{},
		urns:       map[resource.URN]bool{},
	}

	t.Run("new root only", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, sg.validateStateMigrationResult(
			registrationURN, priorRoot, []*pkgresource.State{priorRoot},
			[]*pkgresource.State{registrationRoot}))
	})

	t.Run("prior root only", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, sg.validateStateMigrationResult(
			registrationURN, priorRoot, []*pkgresource.State{priorRoot},
			[]*pkgresource.State{priorRoot}))
	})

	t.Run("both roots", func(t *testing.T) {
		t.Parallel()
		err := sg.validateStateMigrationResult(
			registrationURN, priorRoot, []*pkgresource.State{priorRoot},
			[]*pkgresource.State{priorRoot, registrationRoot})
		require.ErrorContains(t, err, "must include exactly one logical root")
	})
}

func TestValidateStateMigrationResultURNScope(t *testing.T) {
	t.Parallel()

	rootURN := resource.NewURN("test", "test", "", "pkg:m:Component", "component")
	root := &pkgresource.State{URN: rootURN, Type: rootURN.Type()}
	sg := &stepGenerator{
		deployment: &Deployment{olds: map[resource.URN]*pkgresource.State{rootURN: root}},
		aliased:    map[resource.URN]resource.URN{},
		urns:       map[resource.URN]bool{},
	}

	for _, tt := range []struct {
		name     string
		childURN resource.URN
		want     string
	}{
		{
			name:     "different stack",
			childURN: resource.NewURN("other", "test", rootURN.QualifiedType(), "pkg:m:Resource", "child"),
			want:     `migration results must remain in registration stack "test"`,
		},
		{
			name:     "different project",
			childURN: resource.NewURN("test", "other", rootURN.QualifiedType(), "pkg:m:Resource", "child"),
			want:     `and project "test"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			child := &pkgresource.State{URN: tt.childURN, Type: tt.childURN.Type(), Parent: rootURN}
			err := sg.validateStateMigrationResult(
				rootURN, root, []*pkgresource.State{root}, []*pkgresource.State{root, child})
			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestValidateStateMigrationResultParentQualifiedType(t *testing.T) {
	t.Parallel()

	rootURN := resource.NewURN("test", "test", "", "pkg:m:Component", "component")
	root := &pkgresource.State{URN: rootURN, Type: rootURN.Type()}
	sg := &stepGenerator{
		deployment: &Deployment{olds: map[resource.URN]*pkgresource.State{rootURN: root}},
		aliased:    map[resource.URN]resource.URN{},
		urns:       map[resource.URN]bool{},
	}

	t.Run("consistent", func(t *testing.T) {
		t.Parallel()
		childURN := resource.NewURN("test", "test", rootURN.QualifiedType(), "pkg:m:Resource", "child")
		child := &pkgresource.State{URN: childURN, Type: childURN.Type(), Parent: rootURN}
		require.NoError(t, sg.validateStateMigrationResult(
			rootURN, root, []*pkgresource.State{root}, []*pkgresource.State{root, child}))
	})

	t.Run("inconsistent", func(t *testing.T) {
		t.Parallel()
		childURN := resource.NewURN("test", "test", "", "pkg:m:Resource", "child")
		child := &pkgresource.State{URN: childURN, Type: childURN.Type(), Parent: rootURN}
		err := sg.validateStateMigrationResult(
			rootURN, root, []*pkgresource.State{root}, []*pkgresource.State{root, child})
		require.ErrorContains(t, err, "but qualified type pkg:m:Resource; expected pkg:m:Component$pkg:m:Resource")
	})
}
