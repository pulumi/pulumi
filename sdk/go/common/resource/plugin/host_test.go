// Copyright 2016-2018, Pulumi Corporation.
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

package plugin

import (
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/require"
)

func TestClosePanic(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	ctx, err := NewContext(sink, sink, nil, nil, "", nil, false, nil)
	require.NoError(t, err)
	host, ok := ctx.Host.(*defaultHost)
	require.True(t, ok)

	// Spin up a load of loadPlugin calls and then Close the context. This should not panic.
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// We expect some of these to error that the host is shutting down, that's fine this test is just
			// checking nothing panics.
			_, _ = host.loadPlugin(host.loadRequests, func() (interface{}, error) {
				return nil, nil
			})
		}()
	}
	err = host.Close()
	require.NoError(t, err)

	wg.Wait()
}
