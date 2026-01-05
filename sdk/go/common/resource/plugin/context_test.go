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

package plugin

import (
	"context"
	"sync"
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/require"
)

func TestContextRequest_race(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(
		context.Background(),
		diagtest.LogSink(t), // The diagnostics sink to use for messages.
		diagtest.LogSink(t), // The diagnostics sink to use for status messages.
		nil,                 // the host that can be used to fetch providers.
		nil,                 // configSource
		t.TempDir(),         // the working directory to spawn all plugins in.
		nil,                 // runtimeOptions
		false,               // disableProviderPreview
		mocktracer.New().StartSpan("root"),
	)
	require.NoError(t, err)

	// Run 10 goroutines that all call context.Request() concurrently to trigger the race detector.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx.Request()
		}()
	}
	wg.Wait()
}
