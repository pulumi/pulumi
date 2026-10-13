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

package model_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model/rapidmodel"
)

// Tests that UnifyTypes is a function of its inputs: repeated calls on the same pair produce the same safe and
// unsafe types.
func TestUnifyTypesIsDeterministic(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(t *rapid.T) {
		a, b := rapidmodel.Type().Draw(t, "a"), rapidmodel.Type().Draw(t, "b")
		safe, unsafe := model.UnifyTypes(a, b)
		for range 20 {
			safeAgain, unsafeAgain := model.UnifyTypes(a, b)
			require.True(t, safe.Equals(safeAgain),
				"UnifyTypes(%v, %v) gave the safe type %v and then %v", a, b, safe, safeAgain)
			require.True(t, unsafe.Equals(unsafeAgain),
				"UnifyTypes(%v, %v) gave the unsafe type %v and then %v", a, b, unsafe, unsafeAgain)
		}
	})
}
