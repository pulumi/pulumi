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
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	codegen "github.com/pulumi/pulumi/pkg/v3/codegen/python"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
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
		assert.NoError(t, cmd.Process.Kill())
		wg.Wait()
		// We expect this to error because we just killed it.
		contract.IgnoreError(cmd.Wait())
	})

	return address, client
}

// Add test names here that are expected to fail and the reason why they are failing
var expectedFailures = map[string]string{
	"l1-builtin-try":                        "Temporarily disabled until pr #18915 is submitted",
	"l1-builtin-can":                        "Temporarily disabled until pr #18916 is submitted",
	"l2-invoke-scalar":                      "not implemented yet: #19388",
	"l2-component-call-simple-liftedreturn": "will be followed up after nodejs implementation submitted https://github.com/pulumi/pulumi/issues/7435", //nolint:lll
	"l3-component-simple":                   "https://github.com/pulumi/pulumi/issues/19067",
}

func TestLanguage(t *testing.T) {
	t.Parallel()
	engineAddress, engine := runTestingHost(t)

	tests, err := engine.GetLanguageTests(t.Context(), &testingrpc.GetLanguageTestsRequest{})
	require.NoError(t, err)

	// We need to run the python tests multiple times. Once with TOML projects and once with setup.py. We also want to
	// test that explicitly setting the input types works as expected, as well as the default. This shouldn't interact
	// with the project type so we vary both at once. We also want to test that the typechecker works, that doesn't vary
	// by project type but it will vary over classes-vs-dicts. We could run all combinations but we take some time/risk
	// tradeoff here only testing the old classes style with pyright.
	configs := []struct {
		name        string
		snapshotDir string
		useTOML     bool
		inputTypes  string
		typechecker string
		toolchain   string
	}{
		{
			name:        "default",
			snapshotDir: "setuppy",
			useTOML:     false,
			inputTypes:  "",
			typechecker: "mypy",
			toolchain:   "uv",
		},
		{
			name:        "toml",
			snapshotDir: "toml",
			useTOML:     true,
			inputTypes:  "classes-and-dicts",
			typechecker: "pyright",
			toolchain:   "uv",
		},
		{
			name:        "classes",
			snapshotDir: "classes",
			useTOML:     false,
			inputTypes:  "classes",
			typechecker: "pyright",
			toolchain:   "uv",
		},
	}

	for _, config := range configs {
		config := config

		t.Run(config.name, func(t *testing.T) {
			t.Parallel()

			cancel := make(chan bool)

			// Run the language plugin
			handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
				Init: func(srv *grpc.Server) error {
					pythonExec, err := filepath.Abs("../pulumi-language-python-exec")
					if err != nil {
						return err
					}
					host := newLanguageHost(pythonExec, engineAddress, "", config.typechecker, config.toolchain)
					pulumirpc.RegisterLanguageRuntimeServer(srv, host)
					return nil
				},
				Cancel: cancel,
			})
			require.NoError(t, err)

			// Create a temp project dir for the test to run in
			rootDir := t.TempDir()

			snapshotDir := "./testdata/" + config.snapshotDir

			var languageInfo string
			if config.useTOML || config.inputTypes != "" {
				var info codegen.PackageInfo
				info.PyProject.Enabled = config.useTOML
				info.InputTypes = config.inputTypes

				json, err := json.Marshal(info)
				require.NoError(t, err)
				languageInfo = string(json)
			}

			// Prepare to run the tests
			prepare, err := engine.PrepareLanguageTests(t.Context(), &testingrpc.PrepareLanguageTestsRequest{
				LanguagePluginName:   "python",
				LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
				TemporaryDirectory:   rootDir,
				SnapshotDirectory:    snapshotDir,
				CoreSdkDirectory:     "../..",
				CoreSdkVersion:       sdk.Version.String(),
				SnapshotEdits: []*testingrpc.PrepareLanguageTestsRequest_Replacement{
					{
						Path:        "requirements\\.txt",
						Pattern:     fmt.Sprintf("pulumi-%s-py3-none-any.whl", sdk.Version.String()),
						Replacement: "pulumi-CORE.VERSION-py3-none-any.whl",
					},
					{
						Path:        "requirements\\.txt",
						Pattern:     rootDir + "/artifacts",
						Replacement: "ROOT/artifacts",
					},
				},
				LanguageInfo: languageInfo,
			})
			require.NoError(t, err)

			for _, tt := range tests.Tests {
				tt := tt

				t.Run(tt, func(t *testing.T) {
					t.Parallel()

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
				assert.NoError(t, <-handle.Done)
			})
		})
	}
}
