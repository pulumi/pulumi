// Copyright 2016-2018, Pulumi Corporation.
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
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Snapshot is a view of a collection of resources in an stack at a point in time.  It describes resources; their
// IDs, names, and properties; their dependencies; and more.  A snapshot is a diffable entity and can be used to create
// or apply an infrastructure deployment plan in order to make reality match the snapshot state.
type Snapshot struct {
	Manifest          Manifest             // a deployment manifest of versions, checksums, and so on.
	Resources         []*resource.State    // fetches all resources and their associated states.
	PendingOperations []resource.Operation // all currently pending resource operations.
}

// Manifest captures versions for all binaries used to construct this snapshot.
type Manifest struct {
	Time    time.Time              // the time this snapshot was taken.
	Magic   string                 // a magic cookie.
	Version string                 // the pulumi command version.
	Plugins []workspace.PluginInfo // the plugin versions also loaded.
}

// NewMagic creates a magic cookie out of a manifest; this can be used to check for tampering.  This ignores
// any existing magic value already stored on the manifest.
func (m Manifest) NewMagic() string {
	if m.Version == "" {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(m.Version)))
}

// NewSnapshot creates a snapshot from the given arguments.  The resources must be in topologically sorted order.
// This property is not checked; for verification, please refer to the VerifyIntegrity function below.
func NewSnapshot(manifest Manifest, resources []*resource.State, ops []resource.Operation) *Snapshot {
	return &Snapshot{
		Manifest:          manifest,
		Resources:         resources,
		PendingOperations: ops,
	}
}

// VerifyIntegrity checks a snapshot to ensure it is well-formed.  Because of the cost of this operation,
// integrity verification is only performed on demand, and not automatically during snapshot construction.
//
// This function verifies a number of invariants:
//  1. Provider resources must be referenceable (i.e. they must have a valid URN and ID)
//  2. A resource's provider must precede the resource in the resource list
//  3. Parents must precede children in the resource list
//  4. Dependents must precede their dependencies in the resource list
//  5. For every URN in the snapshot, there must be at most one resource with that URN that is not pending deletion
//  6. The magic manifest number should change every time the snapshot is mutated
func (snap *Snapshot) VerifyIntegrity() error {
	if snap != nil {
		// Ensure the magic cookie checks out.
		if snap.Manifest.Magic != snap.Manifest.NewMagic() {
			return errors.Errorf("magic cookie mismatch; possible tampering/corruption detected")
		}

		// Now check the resources.  For now, we just verify that parents come before children, and that there aren't
		// any duplicate URNs.
		urns := make(map[resource.URN]*resource.State)
		provs := make(map[providers.Reference]struct{})
		for i, state := range snap.Resources {
			urn := state.URN

			if providers.IsProviderType(state.Type) {
				ref, err := providers.NewReference(urn, state.ID)
				if err != nil {
					return errors.Errorf("provider %s is not referenceable: %v", urn, err)
				}
				provs[ref] = struct{}{}
			}
			if provider := state.Provider; provider != "" {
				ref, err := providers.ParseReference(provider)
				if err != nil {
					return errors.Errorf("failed to parse provider reference for resource %s: %v", urn, err)
				}
				if _, has := provs[ref]; !has {
					return errors.Errorf("resource %s refers to unknown provider %s", urn, ref)
				}
			}

			if par := state.Parent; par != "" {
				if _, has := urns[par]; !has {
					// The parent isn't there; to give a good error message, see whether it's missing entirely, or
					// whether it comes later in the snapshot (neither of which should ever happen).
					for _, other := range snap.Resources[i+1:] {
						if other.URN == par {
							return errors.Errorf("child resource %s's parent %s comes after it", urn, par)
						}
					}
					return errors.Errorf("child resource %s refers to missing parent %s", urn, par)
				}
			}

			for _, dep := range state.Dependencies {
				if _, has := urns[dep]; !has {
					// same as above - doing this for better error messages
					for _, other := range snap.Resources[i+1:] {
						if other.URN == dep {
							return errors.Errorf("resource %s's dependency %s comes after it", urn, other.URN)
						}
					}

					return errors.Errorf("resource %s dependency %s refers to missing resource", urn, dep)
				}
			}

			if _, has := urns[urn]; has && !state.Delete {
				// The only time we should have duplicate URNs is when all but one of them are marked for deletion.
				return errors.Errorf("duplicate resource %s (not marked for deletion)", urn)
			}

			urns[urn] = state
		}
	}

	return nil
}
