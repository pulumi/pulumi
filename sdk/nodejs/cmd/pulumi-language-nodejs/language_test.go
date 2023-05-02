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
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	enginerpc "github.com/pulumi/pulumi/sdk/v3/proto/go/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func runEngine(t *testing.T) (string, enginerpc.EngineClient) {
	cmd := exec.Command("pulumi", "engine")
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
			t.Logf("engine: %s", text)
		}
	}()

	err = cmd.Start()
	require.NoError(t, err)

	stdoutBytes, err := io.ReadAll(stdout)
	require.NoError(t, err)

	address := string(stdoutBytes)

	conn, err := grpc.Dial(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()),
		grpc.WithStreamInterceptor(rpcutil.OpenTracingStreamClientInterceptor()),
		rpcutil.GrpcChannelOptions(),
	)
	require.NoError(t, err)

	client := enginerpc.NewEngineClient(conn)

	t.Cleanup(func() {
		assert.NoError(t, cmd.Process.Kill())
		wg.Wait()
		require.NoError(t, cmd.Wait())
	})

	return address, client
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	os.Setenv("PULUMI_ACCEPT", "true")

	engineAddress, engine := runEngine(t)

	tests, err := engine.GetLanguageTests(context.Background(), &enginerpc.GetLanguageTestsRequest{})
	require.NoError(t, err)

	// Run the language plugin
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Init: func(srv *grpc.Server) error {
			host := newLanguageHost(engineAddress, "", true, "", "")
			pulumirpc.RegisterLanguageRuntimeServer(srv, host)
			return nil
		},
	})
	require.NoError(t, err)

	// Create a temp project dir for the test to run in
	// rootDir := t.TempDir()
	rootDir, err := os.MkdirTemp("", "pulumi-language-nodejs-test")
	require.NoError(t, err)

	// Prepare to run the tests
	prepare, err := engine.PrepareLanguageTests(context.Background(), &enginerpc.PrepareLanguageTestsRequest{
		LanguagePluginName:   "nodejs",
		LanguagePluginTarget: fmt.Sprintf("127.0.0.1:%d", handle.Port),
		TemporaryDirectory:   rootDir,
		SnapshotDirectory:    "./testdata",
		CoreSdkDirectory:     "../..",
	})
	require.NoError(t, err)

	for _, tt := range tests.Tests {
		tt := tt
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			result, err := engine.RunLanguageTest(context.Background(), &enginerpc.RunLanguageTestRequest{
				Token: prepare.Token,
				Test:  tt,
			})

			require.NoError(t, err)
			t.Logf("stdout: %s", result.Stdout)
			t.Logf("stderr: %s", result.Stderr)
			assert.True(t, result.Success, result.Message)
		})
	}
}
