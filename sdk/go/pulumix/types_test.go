// Copyright 2016-2023, Pulumi Corporation.
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

package pulumix

import (
	"context"
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyIntOutput is a pulumi.Output that does not implement pulumix.Input[T].
type legacyIntOutput struct{ *internal.OutputState }

var _ internal.Output = legacyIntOutput{}

func (legacyIntOutput) ElementType() reflect.Type { return reflect.TypeOf(int(0)) }

// Varying bad implementations of Input[T].
type (
	outputNoContext struct{ *internal.OutputState } // doesn't take a context
	outputNoOutputT struct{ *internal.OutputState } // doesn't produce Output[T]
)

func (outputNoContext) ElementType() reflect.Type { return reflect.TypeOf(int(0)) }
func (outputNoOutputT) ElementType() reflect.Type { return reflect.TypeOf(int(0)) }

func (outputNoContext) ToOutput() Output[int] {
	panic("not implemented")
}

func (outputNoOutputT) ToOutput(context.Context) internal.Output {
	panic("not implemented")
}

func TestInputElementType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give reflect.Type
		want reflect.Type // nil if !ok
	}{
		{
			desc: "nil",
			give: nil,
			want: nil,
		},
		{
			desc: "Output",
			give: reflect.TypeOf(Output[int]{}),
			want: reflect.TypeOf(int(0)),
		},
		{
			desc: "Output complex",
			give: reflect.TypeOf(Output[[]string]{}),
			want: reflect.TypeOf([]string{}),
		},
		{
			desc: "ArrayOutput",
			give: reflect.TypeOf(ArrayOutput[int]{}),
			want: reflect.TypeOf([]int{}),
		},
		{
			desc: "not an input",
			give: reflect.TypeOf(42),
			want: nil,
		},
		{
			desc: "not a pux.Input",
			give: reflect.TypeOf(legacyIntOutput{}),
			want: nil,
		},
		{
			desc: "no context argument",
			give: reflect.TypeOf(outputNoContext{}),
			want: nil,
		},
		{
			desc: "no Output[T] return",
			give: reflect.TypeOf(outputNoOutputT{}),
			want: nil,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			got, ok := InputElementType(tt.give)
			if tt.want == nil {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
