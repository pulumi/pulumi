// Copyright 2025, Pulumi Corporation.
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

package snapshot

import (
	"fmt"
	"runtime/debug"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

func VerifyIntegrity(snap *apitype.DeploymentV3) error {
	if snap == nil {
		return nil
	}

	if snap.Manifest.Magic != snap.Manifest.NewMagic() {
		return SnapshotIntegrityErrorf("magic cookie mismatch: possible tampering/corruption detected")
	}

	urns := make(map[resource.URN][]apitype.ResourceV3)
	provs := make(map[providers.Reference]struct{})
	for i, res := range snap.Resources {
		urn := res.URN
		if urn == "" {
			return SnapshotIntegrityErrorf("resource at index %d missing required 'urn' field", i)
		}
		if res.Type == "" {
			return SnapshotIntegrityErrorf("resource '%s' missing required 'type' field", urn)
		}
		if !res.Custom && res.ID != "" {
			return SnapshotIntegrityErrorf("resource '%s' has 'custom false but non-empty ID", urn)
		}

		if providers.IsProviderType(res.Type) {
			ref, err := providers.NewReference(urn, res.ID)
			if err != nil {
				return SnapshotIntegrityErrorf("provider %s is not referenceable: %w", urn, err)
			}
			provs[ref] = struct{}{}
		}

		if res.Provider != "" {
			ref, err := providers.ParseReference(res.Provider)
			if err != nil {
				return SnapshotIntegrityErrorf("failed to parse provider reference for resource %s: %w", urn, err)
			}
			if _, has := provs[ref]; !has && !res.PendingReplacement {
				return SnapshotIntegrityErrorf("resource %s refers to unknown provider %s", urn, ref)
			}
		}

		// For each resource, we'll ensure that all its dependencies are declared
		// before it in the  In this case, "dependencies" includes the
		// Dependencies field, as well as the resource's Parent (if it has one),
		// any PropertyDependencies, and the DeletedWith field.
		//
		// If a dependency is missing, we'll return an error. In such cases, we'll
		// walk through the remaining resources in the snapshot to see if the
		// missing dependency is declared later in the snapshot or whether it is
		// missing entirely, producing a specific error message depending on the
		// outcome.
		if res.Parent != "" {
			if _, has := urns[res.Parent]; !has {
				for _, other := range snap.Resources[i+1:] {
					if other.URN == res.Parent {
						return SnapshotIntegrityErrorf("child resource %s's parent %s comes after it", urn, res.Parent)
					}
				}
				return SnapshotIntegrityErrorf("child resource %s refers to missing parent %s", urn, res.Parent)
			}
			// Ensure that our URN is a child of the parent's URN.
			expectedType := urn.Type()
			if res.Parent.QualifiedType() != resource.RootStackType {
				expectedType = res.Parent.QualifiedType() + "$" + expectedType
			}

			if urn.QualifiedType() != expectedType {
				logging.Warningf("child resource %s has parent %s but its URN doesn't match", urn, res.Parent)
				// TODO: Change this to an error once we're sure users won't hit this in the wild.
				// return fmt.Errorf("child resource %s has parent %s but its URN doesn't match", urn, dep.URN)
			}
		}
		for _, dep := range res.Dependencies {
			if _, has := urns[dep]; !has {
				for _, other := range snap.Resources[i+1:] {
					if other.URN == dep {
						return SnapshotIntegrityErrorf("resource %s's dependency %s comes after it", urn, dep)
					}
				}
				return SnapshotIntegrityErrorf("resource %s refers to missing dependency %s", urn, dep)
			}
		}

		for _, deps := range res.PropertyDependencies {
			for _, dep := range deps {
				if _, has := urns[dep]; !has {
					for _, other := range snap.Resources[i+1:] {
						if other.URN == dep {
							return SnapshotIntegrityErrorf(
								"resource %s's property dependency %s comes after it",
								urn, other.URN,
							)
						}
					}
					return SnapshotIntegrityErrorf(
						"resource %s refers to missing property dependency %s",
						urn, dep,
					)
				}
			}
		}
		if _, has := urns[res.DeletedWith]; res.DeletedWith != "" && !has {
			for _, other := range snap.Resources[i+1:] {
				if other.URN == res.DeletedWith {
					return SnapshotIntegrityErrorf(
						"resource %s is specified as being deleted with %s, which comes after it",
						urn, other.URN,
					)
				}
			}
			return SnapshotIntegrityErrorf(
				"resource %s is specified as being deleted with %s, which is missing",
				urn, res.DeletedWith,
			)
		}

		urns[urn] = append(urns[urn], res)
	}
	for urn, states := range urns {
		if len(states) == 1 {
			continue
		}

		deletes := 0
		// The only time we should have duplicate URNs is when all or all but one of them are marked for
		// deletion.
		for _, state := range states {
			if state.Delete {
				deletes++
			}
		}

		if deletes != len(states)-1 && deletes != len(states) {
			return SnapshotIntegrityErrorf("duplicate resource %s (not marked for deletion)", urn)
		}
	}

	return nil
}

// A snapshot integrity error is raised when a snapshot is found to be malformed
// or invalid in some way (e.g. missing or out-of-order dependencies, or
// unparseable data).
type SnapshotIntegrityError struct {
	// The underlying error that caused this integrity error, if applicable.
	Err error

	// The operation which caused the error. Defaults to SnapshotIntegrityWrite.
	Op SnapshotIntegrityOperation

	// The stack trace at the point the error was raised.
	Stack []byte

	// Metadata about the operation that caused the error, if available.
	Metadata *apitype.SnapshotIntegrityErrorMetadataV1
}

// The set of operations alongside which snapshot integrity checks can be
// performed.
type SnapshotIntegrityOperation int

const (
	// Snapshot integrity checks were performed at write time.
	SnapshotIntegrityWrite SnapshotIntegrityOperation = 0
	// Snapshot integrity checks were performed at read time.
	SnapshotIntegrityRead SnapshotIntegrityOperation = 1
)

// Creates a new snapshot integrity error with a message produced by the given
// format string and arguments. Supports wrapping errors with %w. Snapshot
// integrity errors are raised by Snapshot.VerifyIntegrity when a problem is
// detected with a snapshot (e.g. missing or out-of-order dependencies, or
// unparseable data).
func SnapshotIntegrityErrorf(format string, args ...any) error {
	return &SnapshotIntegrityError{
		Err:   fmt.Errorf(format, args...),
		Op:    SnapshotIntegrityWrite,
		Stack: debug.Stack(),
	}
}

func (s *SnapshotIntegrityError) Error() string {
	if s.Err == nil {
		return "snapshot integrity error"
	}

	return s.Err.Error()
}

func (s *SnapshotIntegrityError) Unwrap() error {
	return s.Err
}

// // Returns a copy of the given snapshot integrity error with the operation set to
// // SnapshotIntegrityRead and metadata set to the given snapshot's integrity error
// // metadata.
// func (s *SnapshotIntegrityError) ForRead(snap *DeploymentV3) *SnapshotIntegrityError {
// 	return &SnapshotIntegrityError{
// 		Err:      s.Err,
// 		Op:       SnapshotIntegrityRead,
// 		Stack:    s.Stack,
// 		Metadata: snap.Metadata.IntegrityErrorMetadata,
// 	}
// }

// // Returns a tuple in which the second element is true if and only if any error
// // in the given error's tree is a SnapshotIntegrityError. In that case, the
// // first element will be the first SnapshotIntegrityError in the tree. In the
// // event that there is no such SnapshotIntegrityError, the first element will be
// // nil.
// func AsSnapshotIntegrityError(err error) (*SnapshotIntegrityError, bool) {
// 	var sie *SnapshotIntegrityError
// 	ok := errors.As(err, &sie)
// 	return sie, ok
// }
