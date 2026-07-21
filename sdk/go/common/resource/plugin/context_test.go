// Copyright 2016, Pulumi Corporation.
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

	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/stretchr/testify/require"
)

func TestContextRequest_race(t *testing.T) {
	t.Parallel()

	ctx, err := NewContext(
		t.Context(),
		diagtest.LogSink(t), // The diagnostics sink to use for messages.
		diagtest.LogSink(t), // The diagnostics sink to use for status messages.
		&MockHost{},         // the host that can be used to fetch providers; unused by this test.
		nil,                 // configSource
		t.TempDir(),         // the working directory to spawn all plugins in.
		nil,                 // runtimeOptions
		false,               // disableProviderPreview
		mocktracer.New().StartSpan("root"),
	)
	require.NoError(t, err)

	// Run 10 goroutines that all call context.Request() concurrently to trigger the race detector.
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			ctx.Request()
		})
	}
	wg.Wait()
}

func TestWithoutProviderDebugging(t *testing.T) {
	t.Parallel()

	pwd := t.TempDir()
	ctx, err := NewContext(
		t.Context(),
		diagtest.LogSink(t),
		diagtest.LogSink(t),
		&MockHost{},
		nil,
		pwd,
		nil,
		false,
		mocktracer.New().StartSpan("root"),
	)
	require.NoError(t, err)

	require.False(t, ctx.DisableProviderDebugging())
	require.Same(t, ctx, ctx.LifetimeContext())

	view := ctx.WithoutProviderDebugging()
	require.True(t, view.DisableProviderDebugging())
	require.False(t, ctx.DisableProviderDebugging(), "origin context must be unaffected")
	require.NotSame(t, ctx, view)

	require.Same(t, ctx, view.LifetimeContext())
	require.Equal(t, ctx.Pwd, view.Pwd)
	require.Equal(t, ctx.Host, view.Host)

	require.Same(t, view, view.WithoutProviderDebugging())
}
