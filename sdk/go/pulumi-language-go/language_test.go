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
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	gocodegen "github.com/pulumi/pulumi/pkg/v3/codegen/go"
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
	cmd := exec.Command("go", "build", "-C", "../../../pkg", "-o", binary, "./testing/pulumi-test-language")
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
	"l1-config-types-object": "fails to compile",
	"l1-proxy-index":         "fails to compile",
	"l2-proxy-index":         "fails to compile",
	"l1-builtin-try":         "pulumi#18506 Support try in Go program generation",
	"l1-builtin-can":         "pulumi#18570 Support can in Go program generation",

	// pulumi/pulumi#18345
	"l1-keyword-overlap":                  "outputs are not cast correctly from pcl to their pulumi types",                                                 //nolint:lll
	"l2-plain":                            "cannot use &plain.DataArgs{…} (value of type *plain.DataArgs) as plain.DataArgs value in struct literal",       //nolint:lll
	"l2-map-keys":                         "cannot use &plain.DataArgs{…} (value of type *plain.DataArgs) as plain.DataArgs value in struct literal",       //nolint:lll
	"l2-component-program-resource-ref":   "pulumi#18140: cannot use ref.Value (variable of type pulumi.StringOutput) as string value in return statement", //nolint:lll
	"l2-component-component-resource-ref": "pulumi#18140: cannot use ref.Value (variable of type pulumi.StringOutput) as string value in return statement", //nolint:lll
	"l2-component-call-simple":            "pulumi#18202: syntax error: unexpected / in parameter list; possibly missing comma or )",                       //nolint:lll
	"l2-resource-invoke-dynamic-function": "pulumi#18423: pulumi.Interface{} unexpected {, expected )",                                                     //nolint:lll
	"l3-range-resource-output-traversal":  "pulumi#21678: cannot range over an ArrayOutput",
	"l2-resource-order":                   "cannot convert localVar (variable of struct type pulumi.BoolOutput) to type pulumi.Bool", //nolint:lll
}

// Add program overrides here for programs that can't yet be generated correctly due to programgen bugs.
var programOverrides = map[string]*testingrpc.PrepareLanguageTestsRequest_ProgramOverride{
	// TODO[pulumi/pulumi#18202]: Delete this override when the programgen issue is addressed.
	"l2-component-property-deps": {
		Paths: []string{
			filepath.Join("testdata", "overrides", "l2-component-property-deps"),
		},
	},

	// TODO[pulumi/pulumi#18202]: Delete this override when the programgen issue is addressed.
	"l2-provider-call": {
		Paths: []string{
			filepath.Join("testdata", "overrides", "l2-provider-call"),
		},
	},

	// Doesn't add necessary casts for pulumi inputs
	"l3-component-simple": {
		Paths: []string{
			filepath.Join("testdata", "overrides", "l3-component-simple"),
		},
	},

	// TODO[pulumi/pulumi#18202]: Delete this override when the programgen issue is addressed.
	"l2-provider-call-explicit": {
		Paths: []string{
			filepath.Join("testdata", "overrides", "l2-provider-call-explicit"),
		},
	},

	// TODO: Import paths are incorrect due to schema object types not having canonical tokens.
	"l2-module-format": {
		Paths: []string{
			filepath.Join("testdata", "overrides", "l2-module-format"),
		},
	},
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	engineAddress, engine := runTestingHost(t)

	tests, err := engine.GetLanguageTests(t.Context(), &testingrpc.GetLanguageTestsRequest{})
	require.NoError(t, err)

	configs := []struct {
		name         string
		local        bool
		languageInfo *gocodegen.GoPackageInfo
	}{
		{
			name: "published",
		},
		{
			name:  "local",
			local: true,
		},
		{
			name: "extra-types",
			// We don't expect extra-types to interact with "local", so we
			// don't believe it is worth it to test independently.
			languageInfo: &gocodegen.GoPackageInfo{
				GenerateResourceContainerTypes: true,
				// TODO[https://github.com/pulumi/pulumi/issues/21116]:
				// l2-resource-config requires that RespectSchemaVersion
				// is set if any language option is set.
				RespectSchemaVersion: true,
			},
		},
	}

	for _, config := range configs {
		t.Run(config.name, func(t *testing.T) {
			t.Parallel()
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

			snapshotDir := filepath.Join("./testdata", config.name)

			// Prepare to run the tests
			prepare, err := engine.PrepareLanguageTests(t.Context(), &testingrpc.PrepareLanguageTestsRequest{
				LanguagePluginName:   "go",
				LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
				TemporaryDirectory:   rootDir,
				SnapshotDirectory:    snapshotDir,
				CoreSdkDirectory:     "../..",
				CoreSdkVersion:       sdk.Version.String(),
				PolicyPackDirectory:  "./testdata/policies",
				Local:                config.local,
				SnapshotEdits: []*testingrpc.PrepareLanguageTestsRequest_Replacement{
					{
						Path:        "(.+/)?go\\.mod",
						Pattern:     regexp.QuoteMeta(rootDir) + "/",
						Replacement: "/ROOT/",
					},
				},
				ProgramOverrides: programOverrides,
				LanguageInfo: func() string {
					if config.languageInfo == nil {
						return ""
					}
					b, err := json.Marshal(*config.languageInfo)
					require.NoError(t, err)
					return string(b)
				}(),
			})
			require.NoError(t, err)

			for _, tt := range tests.Tests {
				t.Run(tt, func(t *testing.T) {
					t.Parallel()

					// We can skip the l1- local tests without any SDK there's nothing new being tested here.
					if config.local && strings.HasPrefix(tt, "l1-") {
						t.Skip("Skipping l1- tests in local mode")
					}

					// TODO[https://github.com/pulumi/pulumi/issues/21292]: Skip provider tests for now, we test these
					// with NodeJS and Python only.
					if strings.HasPrefix(tt, "provider-") {
						t.Skip("Skipping provider tests")
					}

					if expected, ok := expectedFailures[tt]; ok {
						t.Skipf("Skipping known failure: %s", expected)
					}

					if _, has := programOverrides[tt]; config.local && has {
						t.Skip("Skipping override tests in local mode")
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
