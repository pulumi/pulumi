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

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// Check that an invalid schema triggers an error
//
// TODO(https://github.com/pulumi/pulumi/issues/13945): enable parallel tests
//
//nolint:paralleltest // These aren't yet safe to run in parallel
func TestInvalidSchema(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	engine := &languageTestServer{}
	// We can just reuse the L1Empty host for this, it's not actually going to be used apart from prepare.
	runtime := &L1EmptyLanguageHost{tempDir: tempDir}
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, runtime)
			return nil
		},
	})
	require.NoError(t, err)

	prepareResponse, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "mock",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   tempDir,
		SnapshotDirectory:    "./testdata/snapshots",
		CoreSdkDirectory:     "sdk/dir",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, prepareResponse.Token)

	_, err = engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepareResponse.Token,
		Test:  "internal-bad-schema",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "bind schema for provider bad:")
}
