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
	// We can't just go run the pulumi-test-language package because of
	// https://github.com/golang/go/issues/39172, so we build it to a temp file then run that.
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
		// We expect this to error because we just killed it.
		contract.IgnoreError(cmd.Wait())
	})

	return address, client
}

// Add test names here that are expected to fail and the reason why they are failing
var expectedFailures = map[string]string{
	"l2-component-property-dep":              "TODO",
	"l2-resource-secret":                     "TODO",
	"l2-component-call-simple":               "TODO",
	"l2-resource-option-env-var-mappings":    "TODO",
	"l2-provider-grpc-config-schema-secret":  "TODO",
	"l2-resource-option-replacement-trigger": "TODO",
	"l2-component-program-resource-ref":      "TODO",
	"l2-provider-call-explicit":              "TODO",
	"l2-explicit-provider":                   "TODO",
	"l2-resource-parent-inheritance":         "TODO",
	"l2-namespaced-provider":                 "TODO",
	"l2-explicit-providers":                  "TODO",
	"l2-provider-grpc-config":                "TODO",
	"l2-provider-grpc-config-secret":         "TODO",
	"l2-resource-option-custom-timeouts":     "TODO",
	"l2-component-component-resource-ref":    "TODO",
	"l2-component-property-deps":             "TODO",
	"l2-resource-option-replace-on-changes":  "TODO",
	"l2-parameterized-resource-twice":        "TODO",
	"l2-parameterized-invoke":                "TODO",
	"l2-parameterized-resource":              "TODO",
	"l2-explicit-parameterized-provider":     "TODO",
	"l3-range-resource-output-traversal":     "TODO",
	"l3-component-simple":                    "TODO",
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	engineAddress, engine := runTestingHost(t)

	tests, err := engine.GetLanguageTests(t.Context(), &testingrpc.GetLanguageTestsRequest{})
	require.NoError(t, err)

	cancel := make(chan bool)
	// Run the language plugin
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(engineAddress, "", "")
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
		Cancel: cancel,
	})
	require.NoError(t, err)

	// Create a temp project dir for the test to run in
	rootDir := t.TempDir()

	// Prepare to run the tests
	prepare, err := engine.PrepareLanguageTests(t.Context(), &testingrpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "pcl",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   rootDir,
		SnapshotDirectory:    "./testdata",
	})
	require.NoError(t, err)

	for _, tt := range tests.Tests {
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			// Skip provider and policy tests, there are no plans to support these in PCL
			if strings.HasPrefix(tt, "provider-") {
				t.Skip("Skipping provider tests")
			}
			if strings.HasPrefix(tt, "policy-") {
				t.Skip("Skipping policy tests")
			}

			if expected, ok := expectedFailures[tt]; ok {
				t.Skipf("Skipping known failure: %s", expected)
			}

			result, err := engine.RunLanguageTest(t.Context(), &testingrpc.RunLanguageTestRequest{
				Token: prepare.Token,
				Test:  tt,
			})

			require.NoError(t, err)
			for _, msg := range result.Messages {
				t.Log(msg)
			}
			ptesting.LogTruncated(t, "stdout", result.Stdout)
			ptesting.LogTruncated(t, "stderr", result.Stderr)
			assert.True(t, result.Success)
		})
	}

	t.Cleanup(func() {
		close(cancel)
		require.NoError(t, <-handle.Done)
	})
}
