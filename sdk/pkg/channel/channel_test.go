// Copyright 2024, Pulumi Corporation.
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

package channel

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterRead(t *testing.T) {
	t.Parallel()

	ch := make(chan int)
	filter := func(i int) bool { return i < 10 }
	filtered := FilterRead(ch, filter)
	seenP := promise.Run(func() ([]int, error) {
		var out []int
		for i := range filtered {
			out = append(out, i)
		}
		return out, nil
	})

	for i := 0; i < 20; i++ {
		ch <- i
	}
	close(ch)

	seen, err := seenP.Result(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, seen)
}
