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
	"fmt"
	"log/slog"
	"reflect"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// StateMigrationResourceSerializer converts resource states to and from their checkpoint representation for state
// migration callbacks.
type StateMigrationResourceSerializer interface {
	Serialize(context.Context, *pkgresource.State) (apitype.ResourceV3, error)
	Deserialize(apitype.ResourceV3) (*pkgresource.State, error)
}

type stateMigrationCallbackResult struct {
	resultResources []apitype.ResourceV3
	originalToFinal map[resource.URN]resource.URN
	allToFinal      map[resource.URN]resource.URN
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
		slog.DebugContext(ctx, "running state migration",
			"urn", urn, "migrationNumber", i+1, "migrationCount", len(migrations))

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

		decoder := json.NewDecoder(bytes.NewReader(newJSON))
		decoder.DisallowUnknownFields()
		var newSet []apitype.ResourceV3
		if err := decoder.Decode(&newSet); err != nil {
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

	var normalizedOriginal []apitype.ResourceV3
	if err := json.Unmarshal(originalJSON, &normalizedOriginal); err != nil {
		return nil, fmt.Errorf("state migration for %s: normalizing prior state: %w", urn, err)
	}
	if reflect.DeepEqual(normalizedOriginal, current) {
		// Nothing changed
		return nil, nil
	}
	if err := validateStateMigrationProviderStates(urn, normalizedOriginal, current); err != nil {
		return nil, err
	}

	// originalToFinal maps resources removed from the original subtree directly to their final successors. allToFinal
	// also maps resources introduced and then removed by intermediate callbacks. It is used to rewrite references in
	// the final returned state; for example, both A and B resolve to C after a chain A → B → C.
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
