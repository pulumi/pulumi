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

package pulumix_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlatten(t *testing.T) {
	t.Parallel()

	o := pulumix.Flatten[string, pulumix.Output[string]](pulumix.Val(pulumix.Val("a")))
	v, known, secret, deps, err := internal.AwaitOutput(context.Background(), o)
	require.NoError(t, err)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Empty(t, deps)
	assert.Equal(t, "a", v)
}

func TestFlatten_secret(t *testing.T) {
	t.Parallel()

	o := pulumix.Flatten[string, pulumi.StringOutput](
		pulumix.Val(
			pulumi.ToSecret(pulumi.String("a")).(pulumi.StringOutput),
		),
	)

	v, known, secret, deps, err := internal.AwaitOutput(context.Background(), o)
	require.NoError(t, err)
	assert.True(t, known)
	assert.True(t, secret)
	assert.Empty(t, deps)
	assert.Equal(t, "a", v)
}

func TestFlatten_failedOutput(t *testing.T) {
	t.Parallel()

	in := pulumix.Output[pulumix.Output[string]]{
		OutputState: internal.NewOutputState(nil, reflect.TypeOf(pulumix.Output[string]{})),
	}

	giveErr := errors.New("great sadness")
	internal.RejectOutput(in, giveErr)

	o := pulumix.Flatten[string, pulumix.Output[string]](in)
	_, _, _, _, err := internal.AwaitOutput(context.Background(), o)
	assert.ErrorIs(t, err, giveErr)
}

func TestAll(t *testing.T) {
	t.Parallel()

	o := pulumix.All(
		pulumix.Val("a").AsAny(),
		pulumix.Val(1).AsAny(),
		pulumix.Val(true).AsAny(),
		pulumix.Array[string]{pulumix.Val("b"), pulumix.Val("c")}.AsAny(),
		pulumix.Map[int]{"d": pulumix.Val(3), "e": pulumix.Val(4)}.AsAny(),
	)
	v, _, _, _, err := internal.AwaitOutput(context.Background(), o)
	require.NoError(t, err)

	assert.Equal(t, []any{
		"a",
		1,
		true,
		[]string{"b", "c"},
		map[string]int{"d": 3, "e": 4},
	}, v)
}
