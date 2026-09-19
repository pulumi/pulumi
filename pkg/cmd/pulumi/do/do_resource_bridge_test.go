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

package do

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file stress-tests readNotFound against a resource type it was never written against, to pin
// down which "resource is gone but the provider echoed its ID back" shapes it does and does not
// classify as missing.
//
// `azure:index:linkAttachment` is invented. It stands in for the family of bridged resources whose
// import ID is composite — the TF importer splits the ID into fields and seeds them into state
// before the refresh runs, so a 404'd refresh can leave behind either nothing at all or a partial
// bag of ID-derived fields, depending on the resource. Its delete needs both halves of the ID,
// mirroring `RevokeSecurityGroupIngress` needing a groupId and `DeleteEnvironment` needing an
// application/environment pair.
func doSyntheticBridgeSpec() schema.PackageSpec {
	props := map[string]schema.PropertySpec{
		"meshId": {TypeSpec: schema.TypeSpec{Type: "string"}},
		"linkId": {TypeSpec: schema.TypeSpec{Type: "string"}},
		"region": {TypeSpec: schema.TypeSpec{Type: "string"}},
	}
	return schema.PackageSpec{
		Name: "azure",
		Resources: map[string]schema.ResourceSpec{
			"azure:index:linkAttachment": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Description: "A synthetic bridged resource with a composite import ID.",
					Properties:  props,
				},
				InputProperties: props,
				RequiredInputs:  []string{"meshId", "linkId"},
			},
		},
	}
}

// residue is what a bridged provider leaves in the state it returns from Read once the refresh
// behind it has 404'd. The ID is echoed back regardless — that is the behaviour that makes these
// resources interesting; what varies between them is how much of the ID-derived state survives.
type residue func(meshID, linkID string) (inputs, outputs resource.PropertyMap)

// bridgeOutcome is what a caller driving `pulumi do` observes from a delete.
type bridgeOutcome string

const (
	// reportedNotFound is the outcome a controller can branch on: no provider call, clean signal.
	reportedNotFound bridgeOutcome = "reported not found"
	// deleteRejected is the #23916 failure: we called Delete with unusable state and the provider
	// threw its own error, which is indistinguishable from a real failure and so retries forever.
	deleteRejected bridgeOutcome = "delete rejected by provider"
	// deleteIssued means Delete ran against the (already absent) remote object without erroring.
	deleteIssued bridgeOutcome = "delete issued"
)

// runSyntheticBridgeDelete drives `do ... delete` against the synthetic bridge and reports what the
// caller would see. `live` controls whether the remote object still exists; `leftover` controls what
// the bridge hands back when it does not.
func runSyntheticBridgeDelete(t *testing.T, live bool, leftover residue) (bridgeOutcome, error) {
	t.Helper()

	const (
		meshID   = "mesh-a1b2"
		linkID   = "link-c3d4"
		importID = meshID + "/" + linkID
	)

	provider := &testProvider{
		spec: doSyntheticBridgeSpec(),
		MockProvider: plugin.MockProvider{
			ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
				parts := strings.SplitN(string(req.ID), "/", 2)
				require.Len(t, parts, 2, "the synthetic bridge only handles composite import IDs")

				if live {
					state := resource.PropertyMap{
						"meshId": resource.NewProperty(parts[0]),
						"linkId": resource.NewProperty(parts[1]),
						"region": resource.NewProperty("westus"),
					}
					return plugin.ReadResponse{ReadResult: plugin.ReadResult{
						ID: req.ID, Inputs: state.Copy(), Outputs: state,
					}}, nil
				}

				// The refresh 404s. A bridge warns, drops the resource from state, and — the crux —
				// still hands the requested import ID back to us.
				inputs, outputs := leftover(parts[0], parts[1])
				return plugin.ReadResponse{ReadResult: plugin.ReadResult{
					ID: req.ID, Inputs: inputs, Outputs: outputs,
				}}, nil
			},
			DeleteF: func(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
				// Stand in for the cloud API: the call is malformed unless both halves of the ID
				// reached us as state. An echoed req.ID is not enough, exactly as the terraform-pf
				// bridge cannot delete from an ID alone.
				has := func(key resource.PropertyKey) bool {
					return req.Inputs.HasValue(key) || req.Outputs.HasValue(key)
				}
				if !has("meshId") || !has("linkId") {
					return plugin.DeleteResponse{}, fmt.Errorf(
						"[ERROR] operation error Mesh: DetachLink, api error MissingParameter: " +
							"The request must contain the parameter meshId or linkId")
				}
				return plugin.DeleteResponse{}, nil
			},
		},
	}

	cmd, _, _ := newDoResourceCommand(t, provider)
	cmd.SetArgs([]string{"--stateless", "azure:index:linkAttachment", "delete", importID, "--yes"})
	err := cmd.Execute()

	switch {
	case err == nil:
		return deleteIssued, nil
	case strings.Contains(err.Error(), "was not found"):
		return reportedNotFound, err
	default:
		return deleteRejected, err
	}
}

func TestDoCmdResourceSyntheticBridgeEmptiedRead(t *testing.T) {
	t.Parallel()

	// A control: while the remote object is there, the delete must still go through. Any change to
	// readNotFound that starts reporting live resources as missing would orphan them silently.
	t.Run("live resource still deletes", func(t *testing.T) {
		t.Parallel()
		outcome, err := runSyntheticBridgeDelete(t, true, nil)
		require.NoError(t, err)
		assert.Equal(t, deleteIssued, outcome)
	})

	shapes := []struct {
		name     string
		leftover residue
		want     bridgeOutcome
	}{
		{
			// The shape the real security-group-rule and appconfig resources produce.
			name: "drops all state",
			leftover: func(_, _ string) (resource.PropertyMap, resource.PropertyMap) {
				return resource.PropertyMap{}, resource.PropertyMap{}
			},
			want: reportedNotFound,
		},
		{
			name: "drops all state and omits inputs entirely",
			leftover: func(_, _ string) (resource.PropertyMap, resource.PropertyMap) {
				return nil, resource.PropertyMap{}
			},
			want: reportedNotFound,
		},
		{
			name: "keeps the echoed id as a property",
			leftover: func(mesh, link string) (resource.PropertyMap, resource.PropertyMap) {
				id := resource.NewProperty(mesh + "/" + link)
				return resource.PropertyMap{"id": id}, resource.PropertyMap{"id": id}
			},
			want: reportedNotFound,
		},
		{
			// The importer seeded a provider-level default that survived the 404. It is real state
			// by any structural measure, but useless to the delete.
			name: "keeps a provider default the delete cannot use",
			leftover: func(_, _ string) (resource.PropertyMap, resource.PropertyMap) {
				return resource.PropertyMap{"region": resource.NewProperty("westus")}, resource.PropertyMap{}
			},
			want: deleteRejected,
		},
		{
			name: "keeps one half of the parsed composite id",
			leftover: func(mesh, _ string) (resource.PropertyMap, resource.PropertyMap) {
				return resource.PropertyMap{}, resource.PropertyMap{"meshId": resource.NewProperty(mesh)}
			},
			want: deleteRejected,
		},
		{
			// Both halves survive, so the delete is at least well-formed and the caller sees a
			// success rather than a retry loop — but we have issued a provider call for a resource
			// we were told is gone.
			name: "keeps both halves of the parsed composite id",
			leftover: func(mesh, link string) (resource.PropertyMap, resource.PropertyMap) {
				return resource.PropertyMap{
					"meshId": resource.NewProperty(mesh),
					"linkId": resource.NewProperty(link),
				}, resource.PropertyMap{}
			},
			want: deleteIssued,
		},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			outcome, err := runSyntheticBridgeDelete(t, false, shape.leftover)
			assert.Equal(t, shape.want, outcome, "unexpected outcome (err: %v)", err)
			if outcome == deleteRejected {
				// Pin the symptom this leaves a caller with: a provider error carrying no
				// not-found marker to classify by.
				assert.ErrorContains(t, err, "MissingParameter")
				assert.NotContains(t, err.Error(), "was not found")
			}
		})
	}
}
