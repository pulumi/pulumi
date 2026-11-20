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

package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/runner"
	"github.com/pulumi/pulumi/pkg/v3/testing/pulumi-test-language/tests"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const runtimeError = "error from the mock language program"

type runtimeLogHost struct {
	L1EmptyLanguageHost
	engine pulumirpc.EngineClient
}

func (h *runtimeLogHost) Run(ctx context.Context, req *pulumirpc.RunRequest) (*pulumirpc.RunResponse, error) {
	resp, err := h.L1EmptyLanguageHost.Run(ctx, req)
	if err != nil || req.DryRun {
		return resp, err
	}
	if _, err := h.engine.Log(ctx, &pulumirpc.LogRequest{
		Severity: pulumirpc.LogSeverity_DEBUG,
		Message:  "debug output from the mock language program",
	}); err != nil {
		return nil, err
	}
	_, err = h.engine.Log(ctx, &pulumirpc.LogRequest{
		Severity: pulumirpc.LogSeverity_ERROR,
		Message:  runtimeError,
	})
	return resp, err
}

func TestAssertRuntimeErrors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tempDir := t.TempDir()
	test := tests.LanguageTests["l1-empty"]
	test.AssertRuntimeErrors = func(l *tests.L, logs []string, stderr string) {
		require.Equal(l, []string{runtimeError}, logs)
		require.Empty(l, stderr)
		l.Errorf("unexpected program error: %s", logs[0])
	}
	engine, err := runner.Start(ctx, tests.LanguageTestdata, map[string]tests.LanguageTest{"l1-empty": test})
	require.NoError(t, err)
	t.Cleanup(func() {
		engine.Cancel()
		require.NoError(t, engine.Done())
	})

	conn, err := grpc.NewClient(engine.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	host := &runtimeLogHost{
		L1EmptyLanguageHost: L1EmptyLanguageHost{tempDir: tempDir},
		engine:              pulumirpc.NewEngineClient(conn),
	}
	cancel := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		close(cancel)
		require.NoError(t, <-handle.Done)
	})

	prepare, err := engine.PrepareLanguageTests(ctx, &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "mock",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   tempDir,
		SnapshotDirectory:    "./tests/testdata/snapshots",
		CoreSdkDirectory:     "sdk/dir",
		CoreSdkVersion:       "1.0.1",
	})
	require.NoError(t, err)

	resp, err := engine.RunLanguageTest(ctx, &testingrpc.RunLanguageTestRequest{
		Token: prepare.Token,
		Test:  "l1-empty",
	})
	require.NoError(t, err)
	require.False(t, resp.Success)
	require.Contains(t, strings.Join(resp.Messages, "\n"), "unexpected program error: "+runtimeError)
}
