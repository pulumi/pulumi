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

package runtime

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

func TestEvalContextConcurrentParentChildAccess(t *testing.T) {
	t.Parallel()

	root := NewEvalContext("", "", "", "", "", nil, nil, nil, nil, nil)
	root.SetVariable("shared", cty.StringVal("value"))

	child := root.NewChild()
	child.SetVariable("private", cty.StringVal("ignored"))
	ref := model.VariableReference(&model.Variable{Name: "shared", VariableType: model.StringType})

	const iterations = 2000
	var wg sync.WaitGroup
	wg.Add(2)

	// Reader: the child evaluates an expression that reads the parent's map.
	// Assertions stay out of the goroutine (require's FailNow is unsafe there);
	// the race detector is what this test relies on.
	go func() {
		defer wg.Done()
		for range iterations {
			_, _, _ = child.Evaluate(ref)
		}
	}()

	// Writer: the parent keeps publishing new variables onto the same map.
	go func() {
		defer wg.Done()
		for i := range iterations {
			root.SetVariable(fmt.Sprintf("k%d", i), cty.NumberIntVal(int64(i)))
		}
	}()

	wg.Wait()

	// Sanity check that the reference actually resolves the parent variable, so
	// the test is exercising the lookup path and not silently no-op'ing.
	value, poison, diags := child.Evaluate(ref)
	require.False(t, diags.HasErrors(), "diagnostics: %v", diags)
	require.Nil(t, poison)
	require.Equal(t, "value", value.StringValue())
}
