// Copyright 2016-2021, Pulumi Corporation.
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

package pulumi

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkGroupActsAsWaitGroup(t *testing.T) {
	t.Parallel()

	check := func(j int) func(*testing.T) {
		return func(*testing.T) {
			var n int32
			wg := &workGroup{}
			wg.Add(j)

			for k := 0; k < j; k++ {
				go func() {
					time.Sleep(10 * time.Millisecond)
					atomic.AddInt32(&n, 1)
					wg.Done()
				}()
			}

			wg.Wait()
			assert.Equal(t, int32(j), atomic.AddInt32(&n, 0))
		}
	}

	t.Run("j=1", check(1)) //nolint:paralleltest // uses shared state with parent
	t.Run("j=2", check(2)) //nolint:paralleltest // uses shared state with parent
	t.Run("j=3", check(3)) //nolint:paralleltest // uses shared state with parent
	t.Run("j=4", check(4)) //nolint:paralleltest // uses shared state with parent

	// test Wait does not block on empty
	wg := &workGroup{}
	wg.Wait()
}
