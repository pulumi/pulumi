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
	"strconv"
	"testing"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/internal"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply_simpleSuccess(t *testing.T) {
	t.Parallel()

	out := pulumix.Apply[int](
		pulumix.Val[int](1),
		func(i1 int) []string {
			return []string{
				strconv.Itoa(i1),
			}
		},
	)

	val, known, secret, deps, err := internal.AwaitOutput(context.Background(), out)
	require.NoError(t, err)
	assert.True(t, known)
	assert.False(t, secret)
	assert.Empty(t, deps)
	assert.Equal(t, []string{"1"}, val)
}

func TestApply_secretValue(t *testing.T) {
	t.Parallel()

	out := pulumix.Apply[int](
		pulumix.Output[int]{OutputState: internal.GetOutputState(pulumi.ToSecret(1))},
		func(i1 int) []string {
			return []string{
				strconv.Itoa(i1),
			}
		},
	)

	_, _, secret, _, err := internal.AwaitOutput(context.Background(), out)
	require.NoError(t, err)
	assert.True(t, secret)
}

func TestApplyErr_applyError(t *testing.T) {
	t.Parallel()

	giveErr := errors.New("great sadness")
	out := pulumix.ApplyErr[int](
		pulumix.Val[int](1),
		func(int) (string, error) {
			return "", giveErr
		},
	)

	_, _, _, _, err := internal.AwaitOutput(context.Background(), out)
	assert.ErrorIs(t, err, giveErr)
}

func TestApply_failedOutput(t *testing.T) {
	t.Parallel()

	intType := reflect.TypeOf(0)
	o1 := pulumix.Output[int]{OutputState: internal.NewOutputState(nil, intType)}

	giveErr := errors.New("great sadness")
	internal.RejectOutput(o1, giveErr)

	out := pulumix.Apply[int](
		o1,
		func(int) string {
			t.Errorf("applied function must not be called")
			return ""
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _, _, _, err := internal.AwaitOutput(ctx, out)
	assert.ErrorIs(t, err, giveErr)
}
