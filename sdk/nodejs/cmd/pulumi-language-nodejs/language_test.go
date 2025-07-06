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
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3"

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
	cmd := exec.Command("go", "build", "-C", "../../../../cmd/pulumi-test-language", "-o", binary)
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
	"l2-invoke-options-depends-on": "not implemented yet",
}

func TestLanguage(t *testing.T) {
	// Set PATH to include the local dist directory so policy can run.
	dist, err := filepath.Abs(filepath.Join("..", "..", "dist"))
	require.NoError(t, err)
	t.Setenv("PATH", fmt.Sprintf("%s%c%s", dist, os.PathListSeparator, os.Getenv("PATH")))

	engineAddress, engine := runTestingHost(t)

	tests, err := engine.GetLanguageTests(t.Context(), &testingrpc.GetLanguageTestsRequest{})
	require.NoError(t, err)

	// We should run the nodejs tests twice. Once with tsc and once with ts-node.
	for _, forceTsc := range []bool{false, true} {
		forceTsc := forceTsc
		t.Run(fmt.Sprintf("forceTsc=%v", forceTsc), func(t *testing.T) {
			t.Parallel()

			cancel := make(chan bool)

			// Run the language plugin
			handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
				Init: func(srv *grpc.Server) error {
					host := newLanguageHost(engineAddress, "", forceTsc)
					pulumirpc.RegisterLanguageRuntimeServer(srv, host)
					return nil
				},
				Cancel: cancel,
			})
			require.NoError(t, err)

			// Create a temp project dir for the test to run in
			rootDir, err := filepath.Abs(t.TempDir())
			require.NoError(t, err)

			snapshotDir := "./testdata/"
			if forceTsc {
				snapshotDir += "tsc"
			} else {
				snapshotDir += "tsnode"
			}

			// Prepare to run the tests
			prepare, err := engine.PrepareLanguageTests(t.Context(), &testingrpc.PrepareLanguageTestsRequest{
				LanguagePluginName:   "nodejs",
				LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
				TemporaryDirectory:   rootDir,
				SnapshotDirectory:    snapshotDir,
				CoreSdkDirectory:     "../..",
				CoreSdkVersion:       sdk.Version.String(),
				PolicyPackDirectory:  "testdata/policies",
				SnapshotEdits: []*testingrpc.PrepareLanguageTestsRequest_Replacement{
					{
						Path:        "package\\.json",
						Pattern:     fmt.Sprintf("pulumi-pulumi-%s\\.tgz", sdk.Version.String()),
						Replacement: "pulumi-pulumi-CORE.VERSION.tgz",
					},
					{
						Path:        "package\\.json",
						Pattern:     rootDir + "/artifacts",
						Replacement: "ROOT/artifacts",
					},
				},
			})
			require.NoError(t, err)

			for _, tt := range tests.Tests {
				tt := tt
				t.Run(tt, func(t *testing.T) {
					t.Parallel()

					if expected, ok := expectedFailures[tt]; ok {
						t.Skipf("Skipping known failure: %s", expected)
					}

					// Skip l2-large-string on Node.js 24 https://github.com/nodejs/node/issues/58197
					// TODO: https://github.com/pulumi/pulumi/issues/19442
					if tt == "l2-large-string" {
						cmd := exec.Command("node", "-v")
						output, err := cmd.Output()
						require.NoError(t, err)

						var major int
						_, err = fmt.Sscanf(string(output), "v%d", &major)
						require.NoError(t, err)

						if major >= 24 {
							t.Skip("Skipping test on Node.js 24+ due to known regression")
						}
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
		})
	}
}
