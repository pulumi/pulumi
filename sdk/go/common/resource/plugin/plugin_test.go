// Copyright 2016-2021, Pulumi Corporation.
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
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"testing"

	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogFlowArgumentPropagation(t *testing.T) {
	t.Parallel()

	engine := "127.0.0.1:12345"

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs: []string{engine},
	}), []string{engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs: []string{engine},
		logFlow:    true,
		verbose:    9,
	}), []string{"-v=9", engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs:  []string{engine},
		logFlow:     true,
		logToStderr: true,
		verbose:     9,
	}), []string{"--logtostderr", "-v=9", engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs:      []string{engine},
		tracingEndpoint: "127.0.0.1:6007",
	}), []string{"--tracing", "127.0.0.1:6007", engine})

	assert.Equal(t, buildPluginArguments(pluginArgumentOptions{
		pluginArgs:      []string{engine},
		logFlow:         true,
		logToStderr:     true,
		verbose:         9,
		tracingEndpoint: "127.0.0.1:6007",
	}), []string{"--logtostderr", "-v=9", "--tracing", "127.0.0.1:6007", "127.0.0.1:12345"})
}

// Verifies that when the plugin is an external binary,
// the process is sent a SIGINT signal when the plugin is closed.
//
// Because other processes and signal handling is involved,
// this test relies on a second fake test: TestExecPlugin_fakePlugin.
func TestExecPlugin_gracefulTermination(t *testing.T) {
	t.Parallel()

	sink := diagtest.LogSink(t)
	pwd, root := t.TempDir(), t.TempDir()
	ctx, err := NewContextWithRoot(
		sink,
		sink,
		nil, // host
		pwd,
		root,
		nil,   // runtimeOptions
		false, // disableProviderPreview
		mocktracer.New().StartSpan("root"),
		nil, // plugins
		nil, // config
	)
	require.NoError(t, err)

	exe, err := os.Executable()
	require.NoError(t, err)

	p, err := execPlugin(ctx, exe, "" /* prefix */, workspace.LanguagePlugin,
		[]string{"-test.run=^TestExecPlugin_fakePlugin$"},
		pwd,
		[]string{"FAKE_PLUGIN=1"},
	)
	require.NoError(t, err)

	// Wait until the plugin is ready. It'll print "okay" to stdout when it's ready.
	scanner := bufio.NewScanner(p.Stdout)

	// Scans the next token from Stdout and returns it.
	// Fails the test if the scanner encounters an error.
	requireScan := func() string {
		t.Helper()

		require.True(t, scanner.Scan())
		require.NoError(t, scanner.Err())
		return scanner.Text()
	}

	assert.Equal(t, "okay", requireScan(), "plugin should be ready")

	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, p.Close(), "plugin should close gracefully")
	}()
	assert.Equal(t, "received interrupt", requireScan(),
		"plugin should receive SIGINT")

	<-done
}

// TestExecPlugin_fakePlugin acts like a main() function for a fake plugin
// for TestExecPlugin_gracefulTermination to terminate.
//
// It installs a signal handler and prints messages that will be verified
// by TestExecPlugin_gracefulTermination.
//
//nolint:paralleltest // not a real test
func TestExecPlugin_fakePlugin(t *testing.T) {
	if os.Getenv("FAKE_PLUGIN") != "1" {
		return // this is not a real test
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("okay")
	<-ctx.Done()
	fmt.Println("received interrupt")
}
