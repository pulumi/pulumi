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
	"errors"
	"fmt"

	"github.com/go-test/deep"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func AssertEqual(expected, actual *apitype.DeploymentV3) error {
	// Just want to check the same operations and resources are counted, but order might be slightly different.
	if actual == nil && expected == nil {
		return nil
	}
	if actual == nil {
		return errors.New("actual snapshot is nil")
	}
	if expected == nil {
		return errors.New("expected snapshot is nil")
	}

	if len(actual.PendingOperations) != len(expected.PendingOperations) {
		var actualPendingOps string
		for _, op := range actual.PendingOperations {
			actualPendingOps += fmt.Sprintf("%v (%v), ", op.Type, op.Resource)
		}
		var expectedPendingOps string
		for _, op := range expected.PendingOperations {
			expectedPendingOps += fmt.Sprintf("%v (%v), ", op.Type, op.Resource)
		}
		return fmt.Errorf("actual and expected pending operations differ, %d in actual (have %v), %d in expected (have %v)",
			len(actual.PendingOperations), actualPendingOps, len(expected.PendingOperations), expectedPendingOps)
	}

	pendingOpsMap := make(map[resource.URN][]apitype.OperationV2)

	for _, mop := range expected.PendingOperations {
		pendingOpsMap[mop.Resource.URN] = append(pendingOpsMap[mop.Resource.URN], mop)
	}
	for _, jop := range actual.PendingOperations {
		diffStr := ""
		found := false
		for _, mop := range pendingOpsMap[jop.Resource.URN] {
			if diff := deep.Equal(jop, mop); diff != nil {
				if jop.Resource.URN == mop.Resource.URN {
					diffStr += fmt.Sprintf("%s\n", diff)
				}
			} else {
				found = true
				break
			}
		}
		if !found {
			var pendingOps string
			for _, op := range actual.PendingOperations {
				pendingOps += fmt.Sprintf("%v (%v)\n", op.Type, op.Resource)
			}
			var expectedPendingOps string
			for _, op := range expected.PendingOperations {
				expectedPendingOps += fmt.Sprintf("%v (%v)\n", op.Type, op.Resource)
			}
			return fmt.Errorf("actual and expected pending operations differ, %v (%v) not found in expected\n"+
				"Actual: %v\nExpected: %v\nDiffs: %v",
				jop.Type, jop.Resource, pendingOps, expectedPendingOps, diffStr)
		}
	}

	if len(actual.Resources) != len(expected.Resources) {
		var actualResources string
		for _, r := range actual.Resources {
			actualResources += fmt.Sprintf("%v %v, ", r.URN, r.Delete)
		}
		var expectedResources string
		for _, r := range expected.Resources {
			expectedResources += fmt.Sprintf("%v %v, ", r.URN, r.Delete)
		}
		return fmt.Errorf("actual and expected resources differ, %d in actual (have %v), %d in expected (have %v)",
			len(actual.Resources), actualResources, len(expected.Resources), expectedResources)
	}

	resourcesMap := make(map[resource.URN][]apitype.ResourceV3)

	for _, mr := range expected.Resources {
		if len(mr.PropertyDependencies) > 0 {
			// We normalize empty slices away, so we don't get `nil != [] != key missing` diffs.
			newPropDeps := map[resource.PropertyKey][]resource.URN{}
			for k, v := range mr.PropertyDependencies {
				if len(v) > 0 {
					newPropDeps[k] = v
				}
			}
			mr.PropertyDependencies = newPropDeps
		}
		// Normalize empty Outputs and Inputs.  Since we're serializing and deserializing
		// this in the journal, we lose some information compared to the regular
		// snapshotting algorithm.
		if len(mr.Outputs) == 0 {
			mr.Outputs = make(map[string]any)
		}
		if len(mr.Inputs) == 0 {
			mr.Inputs = make(map[string]any)
		}
		resourcesMap[mr.URN] = append(resourcesMap[mr.URN], mr)
	}

	for _, jr := range actual.Resources {
		if len(jr.PropertyDependencies) > 0 {
			// We normalize empty slices away, so we don't get `nil != [] != key missing` diffs.
			newPropDeps := map[resource.PropertyKey][]resource.URN{}
			for k, v := range jr.PropertyDependencies {
				if len(v) > 0 {
					newPropDeps[k] = v
				}
			}
			jr.PropertyDependencies = newPropDeps
		}

		found := false
		var diffStr string
		// Normalize empty Outputs and Inputs.  Since we're serializing and deserializing
		// this in the journal, we lose some information compared to the regular
		// snapshotting algorithm.
		if len(jr.Outputs) == 0 {
			jr.Outputs = make(map[string]any)
		}
		if len(jr.Inputs) == 0 {
			jr.Inputs = make(map[string]any)
		}
		for _, mr := range resourcesMap[jr.URN] {
			if diff := deep.Equal(jr, mr); diff != nil {
				if jr.URN == mr.URN {
					diffStr += fmt.Sprintf("%s\n", diff)
				}
			} else {
				found = true
				break
			}
		}
		if !found {
			var actualResources string
			for _, jr := range actual.Resources {
				actualResources += fmt.Sprintf("Actual resource: %v\n", jr)
			}
			var expectedResources string
			for _, mr := range expected.Resources {
				expectedResources += fmt.Sprintf("Expected resource: %v\n", mr)
			}
			return fmt.Errorf("actual and expected resources differ, %v not found in expected.\n"+
				"Actual: %v\nExpected: %v\nDiffs: %v",
				jr, actualResources, expectedResources, diffStr)
		}
	}

	return nil
}
