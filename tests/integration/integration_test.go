// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package ints

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

const WindowsOS = "windows"

// assertPerfBenchmark implements the integration.TestStatsReporter interface, and reports test
// failures when a scenario exceeds the provided threshold.
type assertPerfBenchmark struct {
	T                  *testing.T
	MaxPreviewDuration time.Duration
	MaxUpdateDuration  time.Duration
}

func (t assertPerfBenchmark) ReportCommand(stats integration.TestCommandStats) {
	var maxDuration *time.Duration
	if strings.HasPrefix(stats.StepName, "pulumi-preview") {
		maxDuration = &t.MaxPreviewDuration
	}
	if strings.HasPrefix(stats.StepName, "pulumi-update") {
		maxDuration = &t.MaxUpdateDuration
	}

	if maxDuration != nil && *maxDuration != 0 {
		if stats.ElapsedSeconds < maxDuration.Seconds() {
			t.T.Logf(
				"Test step %q was under threshold. %.2fs (max %.2fs)",
				stats.StepName, stats.ElapsedSeconds, maxDuration.Seconds())
		} else {
			t.T.Errorf(
				"Test step %q took longer than expected. %.2fs vs. max %.2fs",
				stats.StepName, stats.ElapsedSeconds, maxDuration.Seconds())
		}
	}
}

// TestStackTagValidation verifies various error scenarios related to stack names and tags.
func TestStackTagValidation(t *testing.T) {
	t.Run("Error_StackName", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "invalid name (spaces, parens, etc.)")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "stack names may only contain alphanumeric, hyphens, underscores, or periods")
	})

	t.Run("Error_DescriptionLength", func(t *testing.T) {
		e := ptesting.NewEnvironment(t)
		defer func() {
			if !t.Failed() {
				e.DeleteEnvironment()
			}
		}()
		e.RunCommand("git", "init")

		e.ImportDirectory("stack_project_name")
		e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

		prefix := "lorem ipsum dolor sit amet"     // 26
		prefix = prefix + prefix + prefix + prefix // 104
		prefix = prefix + prefix + prefix + prefix // 416 + the current Pulumi.yaml's description

		// Change the contents of the Description property of Pulumi.yaml.
		yamlPath := filepath.Join(e.CWD, "Pulumi.yaml")
		err := integration.ReplaceInFile("description: ", "description: "+prefix, yamlPath)
		assert.NoError(t, err)

		stdout, stderr := e.RunCommandExpectError("pulumi", "stack", "init", "valid-name")
		assert.Equal(t, "", stdout)
		assert.Contains(t, stderr, "error: could not create stack:")
		assert.Contains(t, stderr, "validating stack properties:")
		assert.Contains(t, stderr, "stack tag \"pulumi:description\" value is too long (max length 256 characters)")
	})
}

// TestConfigSave ensures that config commands in the Pulumi CLI work as expected.
func TestConfigSave(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	// Initialize an empty stack.
	path := filepath.Join(e.RootPath, "Pulumi.yaml")
	err := (&workspace.Project{
		Name:    "testing-config",
		Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
	}).Save(path)
	assert.NoError(t, err)
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "testing-2")
	e.RunCommand("pulumi", "stack", "init", "testing-1")

	// Now configure and save a few different things:
	e.RunCommand("pulumi", "config", "set", "configA", "value1")
	e.RunCommand("pulumi", "config", "set", "configB", "value2", "--stack", "testing-2")

	e.RunCommand("pulumi", "stack", "select", "testing-2")

	e.RunCommand("pulumi", "config", "set", "configD", "value4")
	e.RunCommand("pulumi", "config", "set", "configC", "value3", "--stack", "testing-1")

	// Now read back the config using the CLI:
	{
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configB")
		assert.Equal(t, "value2\n", stdout)
	}
	{
		// the config in a different stack, so this should error.
		stdout, stderr := e.RunCommandExpectError("pulumi", "config", "get", "configA")
		assert.Equal(t, "", stdout)
		assert.NotEqual(t, "", stderr)
	}
	{
		// but selecting the stack should let you see it
		stdout, _ := e.RunCommand("pulumi", "config", "get", "configA", "--stack", "testing-1")
		assert.Equal(t, "value1\n", stdout)
	}

	// Finally, check that the stack file contains what we expected.
	validate := func(k string, v string, cfg config.Map) {
		key, err := config.ParseKey("testing-config:config:" + k)
		assert.NoError(t, err)
		d, ok := cfg[key]
		assert.True(t, ok, "config key %v should be set", k)
		dv, err := d.Value(nil)
		assert.NoError(t, err)
		assert.Equal(t, v, dv)
	}

	testStack1, err := workspace.LoadProjectStack(filepath.Join(e.CWD, "Pulumi.testing-1.yaml"))
	assert.NoError(t, err)
	testStack2, err := workspace.LoadProjectStack(filepath.Join(e.CWD, "Pulumi.testing-2.yaml"))
	assert.NoError(t, err)

	assert.Equal(t, 2, len(testStack1.Config))
	assert.Equal(t, 2, len(testStack2.Config))

	validate("configA", "value1", testStack1.Config)
	validate("configC", "value3", testStack1.Config)

	validate("configB", "value2", testStack2.Config)
	validate("configD", "value4", testStack2.Config)

	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

// TestConfigPaths ensures that config commands with paths work as expected.
func TestConfigPaths(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	// Initialize an empty stack.
	path := filepath.Join(e.RootPath, "Pulumi.yaml")
	err := (&workspace.Project{
		Name:    "testing-config",
		Runtime: workspace.NewProjectRuntimeInfo("nodejs", nil),
	}).Save(path)
	assert.NoError(t, err)
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
	e.RunCommand("pulumi", "stack", "init", "testing")

	namespaces := []string{"", "my:"}

	tests := []struct {
		Key                   string
		Value                 string
		Secret                bool
		Path                  bool
		TopLevelKey           string
		TopLevelExpectedValue string
	}{
		{
			Key:                   "aConfigValue",
			Value:                 "this value is a value",
			TopLevelKey:           "aConfigValue",
			TopLevelExpectedValue: "this value is a value",
		},
		{
			Key:                   "anotherConfigValue",
			Value:                 "this value is another value",
			TopLevelKey:           "anotherConfigValue",
			TopLevelExpectedValue: "this value is another value",
		},
		{
			Key:                   "bEncryptedSecret",
			Value:                 "this super secret is encrypted",
			Secret:                true,
			TopLevelKey:           "bEncryptedSecret",
			TopLevelExpectedValue: "this super secret is encrypted",
		},
		{
			Key:                   "anotherEncryptedSecret",
			Value:                 "another encrypted secret",
			Secret:                true,
			TopLevelKey:           "anotherEncryptedSecret",
			TopLevelExpectedValue: "another encrypted secret",
		},
		{
			Key:                   "[]",
			Value:                 "square brackets value",
			TopLevelKey:           "[]",
			TopLevelExpectedValue: "square brackets value",
		},
		{
			Key:                   "x.y",
			Value:                 "x.y value",
			TopLevelKey:           "x.y",
			TopLevelExpectedValue: "x.y value",
		},
		{
			Key:                   "0",
			Value:                 "0 value",
			Path:                  true,
			TopLevelKey:           "0",
			TopLevelExpectedValue: "0 value",
		},
		{
			Key:                   "true",
			Value:                 "value",
			Path:                  true,
			TopLevelKey:           "true",
			TopLevelExpectedValue: "value",
		},
		{
			Key:                   `["test.Key"]`,
			Value:                 "test key value",
			Path:                  true,
			TopLevelKey:           "test.Key",
			TopLevelExpectedValue: "test key value",
		},
		{
			Key:                   `nested["test.Key"]`,
			Value:                 "nested test key value",
			Path:                  true,
			TopLevelKey:           "nested",
			TopLevelExpectedValue: `{"test.Key":"nested test key value"}`,
		},
		{
			Key:                   "outer.inner",
			Value:                 "value",
			Path:                  true,
			TopLevelKey:           "outer",
			TopLevelExpectedValue: `{"inner":"value"}`,
		},
		{
			Key:                   "names[0]",
			Value:                 "a",
			Path:                  true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a"]`,
		},
		{
			Key:                   "names[1]",
			Value:                 "b",
			Path:                  true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a","b"]`,
		},
		{
			Key:                   "names[2]",
			Value:                 "c",
			Path:                  true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a","b","c"]`,
		},
		{
			Key:                   "names[3]",
			Value:                 "super secret name",
			Path:                  true,
			Secret:                true,
			TopLevelKey:           "names",
			TopLevelExpectedValue: `["a","b","c","super secret name"]`,
		},
		{
			Key:                   "servers[0].port",
			Value:                 "80",
			Path:                  true,
			TopLevelKey:           "servers",
			TopLevelExpectedValue: `[{"port":80}]`,
		},
		{
			Key:                   "servers[0].host",
			Value:                 "example",
			Path:                  true,
			TopLevelKey:           "servers",
			TopLevelExpectedValue: `[{"host":"example","port":80}]`,
		},
		{
			Key:                   "a.b[0].c",
			Value:                 "true",
			Path:                  true,
			TopLevelKey:           "a",
			TopLevelExpectedValue: `{"b":[{"c":true}]}`,
		},
		{
			Key:                   "a.b[1].c",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "a",
			TopLevelExpectedValue: `{"b":[{"c":true},{"c":false}]}`,
		},
		{
			Key:                   "tokens[0]",
			Value:                 "shh",
			Path:                  true,
			Secret:                true,
			TopLevelKey:           "tokens",
			TopLevelExpectedValue: `["shh"]`,
		},
		{
			Key:                   "foo.bar",
			Value:                 "don't tell",
			Path:                  true,
			Secret:                true,
			TopLevelKey:           "foo",
			TopLevelExpectedValue: `{"bar":"don't tell"}`,
		},
		{
			Key:                   "semiInner.a.b.c.d",
			Value:                 "1",
			Path:                  true,
			TopLevelKey:           "semiInner",
			TopLevelExpectedValue: `{"a":{"b":{"c":{"d":1}}}}`,
		},
		{
			Key:                   "wayInner.a.b.c.d.e.f.g.h.i.j.k",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "wayInner",
			TopLevelExpectedValue: `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":{"k":false}}}}}}}}}}}`,
		},
		{
			Key:                   "foo1[0]",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "foo1",
			TopLevelExpectedValue: `[false]`,
		},
		{
			Key:                   "foo2[0]",
			Value:                 "true",
			Path:                  true,
			TopLevelKey:           "foo2",
			TopLevelExpectedValue: `[true]`,
		},
		{
			Key:                   "foo3[0]",
			Value:                 "10",
			Path:                  true,
			TopLevelKey:           "foo3",
			TopLevelExpectedValue: `[10]`,
		},
		{
			Key:                   "foo4[0]",
			Value:                 "0",
			Path:                  true,
			TopLevelKey:           "foo4",
			TopLevelExpectedValue: `[0]`,
		},
		{
			Key:                   "foo5[0]",
			Value:                 "00",
			Path:                  true,
			TopLevelKey:           "foo5",
			TopLevelExpectedValue: `["00"]`,
		},
		{
			Key:                   "foo6[0]",
			Value:                 "01",
			Path:                  true,
			TopLevelKey:           "foo6",
			TopLevelExpectedValue: `["01"]`,
		},
		{
			Key:                   "foo7[0]",
			Value:                 "0123456",
			Path:                  true,
			TopLevelKey:           "foo7",
			TopLevelExpectedValue: `["0123456"]`,
		},
		{
			Key:                   "bar1.inner",
			Value:                 "false",
			Path:                  true,
			TopLevelKey:           "bar1",
			TopLevelExpectedValue: `{"inner":false}`,
		},
		{
			Key:                   "bar2.inner",
			Value:                 "true",
			Path:                  true,
			TopLevelKey:           "bar2",
			TopLevelExpectedValue: `{"inner":true}`,
		},
		{
			Key:                   "bar3.inner",
			Value:                 "10",
			Path:                  true,
			TopLevelKey:           "bar3",
			TopLevelExpectedValue: `{"inner":10}`,
		},
		{
			Key:                   "bar4.inner",
			Value:                 "0",
			Path:                  true,
			TopLevelKey:           "bar4",
			TopLevelExpectedValue: `{"inner":0}`,
		},
		{
			Key:                   "bar5.inner",
			Value:                 "00",
			Path:                  true,
			TopLevelKey:           "bar5",
			TopLevelExpectedValue: `{"inner":"00"}`,
		},
		{
			Key:                   "bar6.inner",
			Value:                 "01",
			Path:                  true,
			TopLevelKey:           "bar6",
			TopLevelExpectedValue: `{"inner":"01"}`,
		},
		{
			Key:                   "bar7.inner",
			Value:                 "0123456",
			Path:                  true,
			TopLevelKey:           "bar7",
			TopLevelExpectedValue: `{"inner":"0123456"}`,
		},

		// Overwriting a top-level string value is allowed.
		{
			Key:                   "aConfigValue.inner",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "aConfigValue",
			TopLevelExpectedValue: `{"inner":"new value"}`,
		},
		{
			Key:                   "anotherConfigValue[0]",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "anotherConfigValue",
			TopLevelExpectedValue: `["new value"]`,
		},
		{
			Key:                   "bEncryptedSecret.inner",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "bEncryptedSecret",
			TopLevelExpectedValue: `{"inner":"new value"}`,
		},
		{
			Key:                   "anotherEncryptedSecret[0]",
			Value:                 "new value",
			Path:                  true,
			TopLevelKey:           "anotherEncryptedSecret",
			TopLevelExpectedValue: `["new value"]`,
		},
	}

	validateConfigGet := func(key string, value string, path bool) {
		args := []string{"config", "get", key}
		if path {
			args = append(args, "--path")
		}
		stdout, stderr := e.RunCommand("pulumi", args...)
		assert.Equal(t, fmt.Sprintf("%s\n", value), stdout)
		assert.Equal(t, "", stderr)
	}

	for _, ns := range namespaces {
		for _, test := range tests {
			key := fmt.Sprintf("%s%s", ns, test.Key)
			topLevelKey := fmt.Sprintf("%s%s", ns, test.TopLevelKey)

			// Set the value.
			args := []string{"config", "set"}
			if test.Secret {
				args = append(args, "--secret")
			}
			if test.Path {
				args = append(args, "--path")
			}
			args = append(args, key, test.Value)
			stdout, stderr := e.RunCommand("pulumi", args...)
			assert.Equal(t, "", stdout)
			assert.Equal(t, "", stderr)

			// Get the value and validate it.
			validateConfigGet(key, test.Value, test.Path)

			// Get the top-level value and validate it.
			validateConfigGet(topLevelKey, test.TopLevelExpectedValue, false /*path*/)
		}
	}

	badKeys := []string{
		// Syntax errors.
		"root[",
		`root["nested]`,
		"root.array[abc]",
		"root.[1]",

		// First path segment must be a non-empty string.
		`[""]`,
		"[0]",

		// Index out of range.
		"names[-1]",
		"names[5]",

		// A "secure" key that is a map with a single string value is reserved by the system.
		"key.secure",
		"super.nested.map.secure",

		// Type mismatch.
		"outer[0]",
		"names.nested",
		"outer.inner.nested",
		"outer.inner[0]",
	}

	for _, ns := range namespaces {
		for _, badKey := range badKeys {
			key := fmt.Sprintf("%s%s", ns, badKey)
			stdout, stderr := e.RunCommandExpectError("pulumi", "config", "set", "--path", key, "value")
			assert.Equal(t, "", stdout)
			assert.NotEqual(t, "", stderr)
		}
	}

	e.RunCommand("pulumi", "stack", "rm", "--yes")
}

//nolint:golint,deadcode
func pathEnv(t *testing.T, path ...string) string {
	pathEnv := []string{os.Getenv("PATH")}
	for _, p := range path {
		absPath, err := filepath.Abs(p)
		if err != nil {
			t.Fatal(err)
			return ""
		}
		pathEnv = append(pathEnv, absPath)
	}
	pathSeparator := ":"
	if runtime.GOOS == "windows" {
		pathSeparator = ";"
	}
	return "PATH=" + strings.Join(pathEnv, pathSeparator)
}

//nolint:golint,deadcode
func testComponentSlowPathEnv(t *testing.T) string {
	return pathEnv(t, filepath.Join("construct_component_slow", "testcomponent"))
}

//nolint:golint,deadcode
func testComponentPlainPathEnv(t *testing.T) string {
	return pathEnv(t, filepath.Join("construct_component_plain", "testcomponent"))
}

// nolint: unused,deadcode
func venvFromPipenv(relativeWorkdir string) (string, error) {
	workdir, err := filepath.Abs(relativeWorkdir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("pipenv", "--venv")
	cmd.Dir = workdir
	dir, err := cmd.Output()
	if err != nil {
		return "", err
	}
	venv := strings.TrimRight(string(dir), "\r\n")
	if _, err := os.Stat(venv); os.IsNotExist(err) {
		return "", fmt.Errorf("Folder '%s' returned by 'pipenv --venv' from %s does not exist: %w",
			venv, workdir, err)
	}
	return venv, nil
}

// nolint: unused,deadcode
func pulumiRuntimeVirtualEnv(t *testing.T, pulumiRepoRootDir string) string {
	venvFolder, err := venvFromPipenv(filepath.Join(pulumiRepoRootDir, "sdk", "python"))
	if err != nil {
		t.Fatal(fmt.Errorf("PULUMI_RUNTIME_VIRTUALENV guess failed: %w", err))
		return ""
	}
	r := fmt.Sprintf("PULUMI_RUNTIME_VIRTUALENV=%s", venvFolder)
	return r
}

// nolint: unused,deadcode
func testComponentProviderSchema(t *testing.T, path string, env ...string) {
	tests := []struct {
		name          string
		env           []string
		version       int32
		expected      string
		expectedError string
	}{
		{
			name:     "Default",
			expected: "{}",
		},
		{
			name:     "Schema",
			env:      []string{"INCLUDE_SCHEMA=true"},
			expected: `{"hello": "world"}`,
		},
		{
			name:          "Invalid Version",
			version:       15,
			expectedError: "unsupported schema version 15",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Start the plugin binary.
			cmd := exec.Command(path, "ignored")
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env, env...)
			cmd.Env = append(cmd.Env, test.env...)
			stdout, err := cmd.StdoutPipe()
			assert.NoError(t, err)
			err = cmd.Start()
			assert.NoError(t, err)
			defer func() {
				// Ignore the error as it may fail with access denied on Windows.
				cmd.Process.Kill() // nolint: errcheck
			}()

			// Read the port from standard output.
			reader := bufio.NewReader(stdout)
			bytes, err := reader.ReadBytes('\n')
			assert.NoError(t, err)
			port := strings.TrimSpace(string(bytes))

			// Create a connection to the server.
			conn, err := grpc.Dial("127.0.0.1:"+port, grpc.WithInsecure(), rpcutil.GrpcChannelOptions())
			assert.NoError(t, err)
			client := pulumirpc.NewResourceProviderClient(conn)

			// Call GetSchema and verify the results.
			resp, err := client.GetSchema(context.Background(), &pulumirpc.GetSchemaRequest{Version: test.version})
			if test.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				assert.Equal(t, test.expected, resp.GetSchema())
			}
		})
	}
}

// Test remote component inputs properly handle unknowns.
// nolint: unused,deadcode
func testConstructUnknown(t *testing.T, lang string, dependencies ...string) {
	const testDir = "construct_component_unknown"
	tests := []struct {
		componentDir string
		env          []string
	}{
		{
			componentDir: "testcomponent",
		},
		{
			componentDir: "testcomponent-python",
			env:          []string{pulumiRuntimeVirtualEnv(t, filepath.Join("..", ".."))},
		},
		{
			componentDir: "testcomponent-go",
		},
	}
	for _, test := range tests {
		t.Run(test.componentDir, func(t *testing.T) {
			pathEnv := pathEnv(t,
				filepath.Join("..", "testprovider"),
				filepath.Join(testDir, test.componentDir))
			integration.ProgramTest(t, &integration.ProgramTestOptions{
				Env:                    append(test.env, pathEnv),
				Dir:                    filepath.Join(testDir, lang),
				Dependencies:           dependencies,
				SkipRefresh:            true,
				SkipPreview:            false,
				SkipUpdate:             true,
				SkipExportImport:       true,
				SkipEmptyPreviewUpdate: true,
				Quick:                  false,
				NoParallel:             true,
			})
		})
	}
}

func TestRotatePassphrase(t *testing.T) {
	e := ptesting.NewEnvironment(t)
	defer func() {
		if !t.Failed() {
			e.DeleteEnvironment()
		}
	}()

	e.ImportDirectory("rotate_passphrase")
	e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())

	e.RunCommand("pulumi", "stack", "init", "dev")
	e.RunCommand("pulumi", "up", "--skip-preview", "--yes")

	e.RunCommand("pulumi", "config", "set", "--secret", "foo", "bar")

	e.SetEnvVars([]string{"PULUMI_TEST_PASSPHRASE=true"})
	e.Stdin = strings.NewReader("qwerty\nqwerty\n")
	e.RunCommand("pulumi", "stack", "change-secrets-provider", "passphrase")

	e.Stdin, e.Passphrase = nil, "qwerty"
	e.RunCommand("pulumi", "config", "get", "foo")
}
