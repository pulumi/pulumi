// Copyright 2016-2026, Pulumi Corporation.
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
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	testingrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func runTestingHost(t *testing.T) (string, testingrpc.LanguageTestClient) {
	t.Helper()

	// Build the test engine binary.
	binary := t.TempDir() + "/pulumi-test-language"
	cmd := exec.Command("go", "build", "-C", "../../../../pkg", "-o", binary, "./testing/pulumi-test-language")
	output, err := cmd.CombinedOutput()
	t.Logf("build output: %s", output)
	require.NoError(t, err)

	cmd = exec.Command(binary)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)
	stderrReader := bufio.NewReader(stderr)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			text, err := stderrReader.ReadString('\n')
			if err != nil {
				wg.Done()
				return
			}
			t.Log(text)
		}
	}()

	err = cmd.Start()
	require.NoError(t, err)

	stdoutBytes, err := io.ReadAll(stdout)
	require.NoError(t, err)

	address := string(stdoutBytes)

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(rpcutil.OpenTracingStreamClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	require.NoError(t, err)

	client := testingrpc.NewLanguageTestClient(conn)

	t.Cleanup(func() {
		require.NoError(t, cmd.Process.Kill())
		wg.Wait()
		contract.IgnoreError(cmd.Wait())
	})

	return address, client
}

// Tests that are expected to pass with the REST language host.
// We start with l1 and l2 tests that don't require callbacks/transforms.
var supportedTests = map[string]bool{
	"l1-empty":          true,
	"l2-resource-simple": true,
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	engineAddress, engine := runTestingHost(t)

	tests, err := engine.GetLanguageTests(t.Context(), &testingrpc.GetLanguageTestsRequest{})
	require.NoError(t, err)

	cancel := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			host := &restLanguageHost{
				engineAddress: engineAddress,
			}
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Cancel: cancel,
	})
	require.NoError(t, err)

	rootDir := t.TempDir()

	prepare, err := engine.PrepareLanguageTests(t.Context(), &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "rest",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   rootDir,
		SnapshotDirectory:    "./testdata",
	})
	require.NoError(t, err)

	for _, tt := range tests.Tests {
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			// Skip provider, policy, and l3 tests for now.
			if strings.HasPrefix(tt, "provider-") {
				t.Skip("Skipping provider tests")
			}
			if strings.HasPrefix(tt, "policy-") {
				t.Skip("Skipping policy tests")
			}
			if strings.HasPrefix(tt, "l3-") {
				t.Skip("Skipping l3 tests (require callbacks/transforms)")
			}

			result, err := engine.RunLanguageTest(t.Context(), &testingrpc.RunLanguageTestRequest{
				Token: prepare.Token,
				Test:  tt,
			}, grpc.MaxCallRecvMsgSize(1024*1024*1024))

			require.NoError(t, err)
			for _, msg := range result.Messages {
				t.Log(msg)
			}
			ptesting.LogTruncated(t, "stdout", result.Stdout)
			ptesting.LogTruncated(t, "stderr", result.Stderr)

			if !supportedTests[tt] {
				// For unsupported tests, we just want to make sure they don't crash.
				// We don't assert success.
				t.Logf("Test %s completed (success=%v, not yet supported)", tt, result.Success)
				return
			}
			assert.True(t, result.Success)
		})
	}

	t.Cleanup(func() {
		close(cancel)
		require.NoError(t, <-handle.Done)
	})
}
