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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// StateMigrationResourceSerializer converts a resource state to its checkpoint (apitype.ResourceV3) representation
// for a state migration callback. Secrets are expected to be serialized in plaintext, under the secret signature
// envelope.
//
// This is injected via Options by the engine: the serialization logic lives in pkg/resource/stack, which imports
// this package and so cannot be imported from here.
type StateMigrationResourceSerializer func(ctx context.Context, res *pkgresource.State) (apitype.ResourceV3, error)

// StateMigrationResourceDeserializer converts a checkpoint (apitype.ResourceV3) representation back into a resource
// state. It is the inverse of StateMigrationResourceSerializer and is injected via Options for the same reason.
type StateMigrationResourceDeserializer func(res apitype.ResourceV3) (*pkgresource.State, error)

type stateMigrationCallbackResult struct {
	resultResources []apitype.ResourceV3
	originalToFinal map[resource.URN]resource.URN
	allToFinal      map[resource.URN]resource.URN
}

func decodeStateMigrationResources(data []byte) ([]apitype.ResourceV3, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var resources []apitype.ResourceV3
	if err := decoder.Decode(&resources); err != nil {
		return nil, err
	}

	// json.Decoder accepts multiple top-level values. Preserve json.Unmarshal's previous rejection of trailing JSON.
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("unexpected trailing JSON value")
		}
		return nil, err
	}
	return resources, nil
}

// runStateMigrationCallbacks evaluates an ordered callback chain without mutating deployment state. A nil result
// means every callback was a no-op or the final checkpoint is semantically identical to the original.
func runStateMigrationCallbacks(
	ctx context.Context,
	urn resource.URN,
	migrations []StateMigrationFunction,
	original []apitype.ResourceV3,
) (*stateMigrationCallbackResult, error) {
	currentJSON, err := json.Marshal(original)
	if err != nil {
		return nil, fmt.Errorf("state migration for %s: marshaling prior state: %w", urn, err)
	}
	originalJSON := currentJSON
	current := original
	allSuccessors := make(map[resource.URN]resource.URN)
	changed := false

	for i, migrate := range migrations {
		logging.V(5).Infof("StateMigration: running state migration (%d of %d) for %s", i+1, len(migrations), urn)

		newJSON, successors, err := migrate(ctx, urn, currentJSON)
		if err != nil {
			return nil, fmt.Errorf("state migration %d of %d for %s: %w", i+1, len(migrations), urn, err)
		}
		if newJSON == nil {
			if len(successors) > 0 {
				return nil, fmt.Errorf("state migration %d of %d for %s: returned successors without "+
					"returning a new state", i+1, len(migrations), urn)
			}
			continue
		}

		newSet, err := decodeStateMigrationResources(newJSON)
		if err != nil {
			return nil, fmt.Errorf("state migration %d of %d for %s: unmarshaling returned state: %w",
				i+1, len(migrations), urn, err)
		}
		if err := validateStateMigrationAccounting(urn, current, newSet, successors); err != nil {
			return nil, err
		}
		for oldURN, successor := range successors {
			if previous, exists := allSuccessors[oldURN]; exists && previous != successor {
				return nil, fmt.Errorf(
					"state migration %d of %d for %s: resource %s has conflicting successors %s and %s",
					i+1, len(migrations), urn, oldURN, previous, successor)
			}
			allSuccessors[oldURN] = successor
		}

		current, currentJSON, changed = newSet, newJSON, true
	}

	if !changed {
		return nil, nil
	}

	// Compare against the original normalized through the same JSON round-trip so typed secrets, nil collections,
	// and other checkpoint serialization asymmetries do not turn an echo callback into a migration transaction.
	var normalizedOriginal []apitype.ResourceV3
	if err := json.Unmarshal(originalJSON, &normalizedOriginal); err != nil {
		return nil, fmt.Errorf("state migration for %s: normalizing prior state: %w", urn, err)
	}
	if reflect.DeepEqual(normalizedOriginal, current) {
		return nil, nil
	}
	if err := validateStateMigrationProviderStates(urn, normalizedOriginal, current); err != nil {
		return nil, err
	}

	// originalToFinal maps removed resources from the original subtree to their final successors. allToFinal also
	// maps intermediate callback states, so references returned by later callbacks cannot dangle.
	originalToFinal, allToFinal, err := finalStateMigrationSuccessors(original, current, allSuccessors)
	if err != nil {
		return nil, fmt.Errorf("state migration for %s: %w", urn, err)
	}
	return &stateMigrationCallbackResult{
		resultResources: current,
		originalToFinal: originalToFinal,
		allToFinal:      allToFinal,
	}, nil
}
