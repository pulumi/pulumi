// Copyright 2016-2018, Pulumi Corporation.
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/nodejs/npm"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgumentConstruction(t *testing.T) {
	t.Parallel()

	info := &pulumirpc.ProgramInfo{
		RootDirectory:    "/foo/bar",
		ProgramDirectory: "/foo/bar",
		EntryPoint:       ".",
	}

	t.Run("DryRun-NoArguments", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{DryRun: true, Info: info}
		args := host.constructArguments(rr, "", "", "")
		assert.Contains(t, args, "--dry-run")
		assert.NotContains(t, args, "true")
	})

	t.Run("OptionalArgs-PassedIfSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Project: "foo", Info: info}
		args := strings.Join(host.constructArguments(rr, "", "", ""), " ")
		assert.Contains(t, args, "--project foo")
	})

	t.Run("OptionalArgs-NotPassedIfNotSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Info: info}
		args := strings.Join(host.constructArguments(rr, "", "", ""), " ")
		assert.NotContains(t, args, "--stack")
	})

	t.Run("DotIfProgramNotSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Info: info}
		args := strings.Join(host.constructArguments(rr, "", "", ""), " ")
		assert.Contains(t, args, ".")
	})

	t.Run("ProgramIfProgramSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{
			Program: "foobar",
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    "/foo/bar",
				ProgramDirectory: "/foo/bar",
				EntryPoint:       "foobar",
			},
		}
		args := strings.Join(host.constructArguments(rr, "", "", ""), " ")
		assert.Contains(t, args, "foobar")
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()
	t.Run("Config-Empty", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Project: "foo"}
		str, err := host.constructConfig(rr)
		assert.NoError(t, err)
		assert.JSONEq(t, "{}", str)
	})
}

func TestCompatibleVersions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a          string
		b          string
		compatible bool
		errmsg     string
	}{
		{"0.17.1", "0.16.2", false, "Differing major or minor versions are not supported."},
		{"0.17.1", "1.0.0", true, ""},
		{"1.0.0", "0.17.1", true, ""},
		{"1.13.0", "1.13.0", true, ""},
		{"1.1.1", "1.13.0", true, ""},
		{"1.13.0", "1.1.1", true, ""},
		{"1.1.0", "2.1.0", true, ""},
		{"2.1.0", "1.1.0", true, ""},
		{"1.1.0", "2.0.0-beta1", true, ""},
		{"2.0.0-beta1", "1.1.0", true, ""},
		{"2.1.0", "3.1.0", false, "Differing major versions are not supported."},
		{"0.16.1", "1.0.0", false, "Differing major or minor versions are not supported."},
	}

	for _, c := range cases {
		compatible, errmsg := compatibleVersions(semver.MustParse(c.a), semver.MustParse(c.b))
		assert.Equal(t, c.errmsg, errmsg)
		assert.Equal(t, c.compatible, compatible)
	}
}

func TestGetRequiredPlugins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	files := []struct {
		path    string
		content string
	}{
		{
			filepath.Join(dir, "node_modules", "@pulumi", "foo", "package.json"),
			`{ "name": "@pulumi/foo", "version": "1.2.3", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "node_modules", "@pulumi", "bar", "package.json"),
			`{ "name": "@pulumi/bar", "version": "4.5.6", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "node_modules", "@pulumi", "baz", "package.json"),
			`{ "name": "@pulumi/baz", "version": "4.5.6", "pulumi": { "resource": false } }`,
		},
		{
			filepath.Join(dir, "node_modules", "malformed", "tests", "malformed_test", "package.json"),
			`{`,
		},
	}
	for _, file := range files {
		err := os.MkdirAll(filepath.Dir(file.path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(file.path, []byte(file.content), 0o600)
		require.NoError(t, err)
	}

	host := &nodeLanguageHost{}
	resp, err := host.GetRequiredPlugins(context.Background(), &pulumirpc.GetRequiredPluginsRequest{
		Program: dir,
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.GetPlugins() {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
	}, actual)
}

func TestGetRequiredPluginsSymlinkCycles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	files := []struct {
		path    string
		content string
	}{
		{
			filepath.Join(dir, "node_modules", "@pulumi", "foo", "package.json"),
			`{ "name": "@pulumi/foo", "version": "1.2.3", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "node_modules", "@pulumi", "bar", "package.json"),
			`{ "name": "@pulumi/bar", "version": "4.5.6", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "node_modules", "@pulumi", "baz", "package.json"),
			`{ "name": "@pulumi/baz", "version": "4.5.6", "pulumi": { "resource": false } }`,
		},
		{
			filepath.Join(dir, "node_modules", "malformed", "tests", "malformed_test", "package.json"),
			`{`,
		},
	}
	for _, file := range files {
		err := os.MkdirAll(filepath.Dir(file.path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(file.path, []byte(file.content), 0o600)
		require.NoError(t, err)
	}

	// Add a symlink cycle in
	err := os.Symlink(filepath.Join(dir, "node_modules"), filepath.Join(dir, "node_modules", "@node_modules"))
	require.NoError(t, err)

	host := &nodeLanguageHost{}
	resp, err := host.GetRequiredPlugins(context.Background(), &pulumirpc.GetRequiredPluginsRequest{
		Program: dir,
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.GetPlugins() {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
	}, actual)
}

func TestGetRequiredPluginsSymlinkCycles2(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "testdir")
	err := os.Mkdir(dir, 0o755)
	require.NoError(t, err)

	files := []struct {
		path    string
		content string
	}{
		{
			filepath.Join(dir, "node_modules", "@pulumi", "foo", "package.json"),
			`{ "name": "@pulumi/foo", "version": "1.2.3", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "node_modules", "@pulumi", "bar", "package.json"),
			`{ "name": "@pulumi/bar", "version": "4.5.6", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "node_modules", "@pulumi", "baz", "package.json"),
			`{ "name": "@pulumi/baz", "version": "4.5.6", "pulumi": { "resource": false } }`,
		},
		{
			filepath.Join(dir, "node_modules", "malformed", "tests", "malformed_test", "package.json"),
			`{`,
		},
	}
	for _, file := range files {
		err := os.MkdirAll(filepath.Dir(file.path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(file.path, []byte(file.content), 0o600)
		require.NoError(t, err)
	}

	// Add a symlink cycle in
	err = os.Symlink(filepath.Join("..", ".."), filepath.Join(dir, "node_modules", "@node_modules"))
	require.NoError(t, err)

	host := &nodeLanguageHost{}
	resp, err := host.GetRequiredPlugins(context.Background(), &pulumirpc.GetRequiredPluginsRequest{
		Program: dir,
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.GetPlugins() {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
	}, actual)
}

func TestGetRequiredPluginsNestedPolicyPack(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "testdir")
	err := os.Mkdir(dir, 0o755)
	require.NoError(t, err)

	files := []struct {
		path    string
		content string
	}{
		{
			filepath.Join(dir, "node_modules", "@pulumi", "foo", "package.json"),
			`{ "name": "@pulumi/foo", "version": "1.2.3", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "node_modules", "@pulumi", "bar", "package.json"),
			`{ "name": "@pulumi/bar", "version": "4.5.6", "pulumi": { "resource": true } }`,
		},
		{
			filepath.Join(dir, "policy", "PulumiPolicy.yaml"),
			`name: my-policy`,
		},
		{
			filepath.Join(dir, "policy", "node_modules", "@pulumi", "baz", "package.json"),
			`{ "name": "@pulumi/baz", "version": "7.8.9", "pulumi": { "resource": true } }`,
		},
	}
	for _, file := range files {
		err := os.MkdirAll(filepath.Dir(file.path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(file.path, []byte(file.content), 0o600)
		require.NoError(t, err)
	}

	host := &nodeLanguageHost{}
	resp, err := host.GetRequiredPlugins(context.Background(), &pulumirpc.GetRequiredPluginsRequest{
		Program: dir,
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.GetPlugins() {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
		// baz: v7.8.9 is not included because it is in a nested policy pack
	}, actual)
}

func TestParseOptions(t *testing.T) {
	t.Parallel()

	opts, err := parseOptions(nil)
	require.NoError(t, err)
	require.Equal(t, npm.AutoPackageManager, opts.packagemanager)

	_, err = parseOptions(map[string]interface{}{
		"typescript": 123,
	})
	require.ErrorContains(t, err, "typescript option must be a boolean")

	_, err = parseOptions(map[string]interface{}{
		"packagemanager": "poetry",
	})
	require.ErrorContains(t, err, "packagemanager option must be one of")

	for _, tt := range []struct {
		input    string
		expected npm.PackageManagerType
	}{
		{"auto", npm.AutoPackageManager},
		{"npm", npm.NpmPackageManager},
		{"yarn", npm.YarnPackageManager},
		{"pnpm", npm.PnpmPackageManager},
	} {
		opts, err = parseOptions(map[string]interface{}{
			"packagemanager": tt.input,
		})
		require.NoError(t, err)
		require.Equal(t, tt.expected, opts.packagemanager)
	}
}

func TestNonblockingStdout(t *testing.T) {
	// Regression test for https://github.com/pulumi/pulumi/issues/16503
	t.Parallel()

	script := `import os, time
os.set_blocking(1, False) # set stdout to non-blocking
time.sleep(3)
`

	// Create a named pipe to use as stdout
	tmp := os.TempDir()
	p := filepath.Join(tmp, "fake-stdout")
	err := syscall.Mkfifo(p, 0o644)
	defer os.Remove(p)
	require.NoError(t, err)
	// Open fd without O_NONBLOCK, ensuring that os.NewFile does not return a pollable file.
	// When our python script changes the file to non-blocking, Go does not notice and continues to
	// expect the file to be blocking, and we can trigger the bug.
	fd, err := syscall.Open(p, syscall.O_CREAT|syscall.O_RDWR, 0o644)
	require.NoError(t, err)
	fakeStdout := os.NewFile(uintptr(fd), p)
	defer fakeStdout.Close()
	require.NotNil(t, fakeStdout)

	cmd := exec.Command("python3", "-c", script)

	var done bool
	go func() {
		time.Sleep(2 * time.Second)
		for !done {
			s := "....................\n"
			n, err := fakeStdout.Write([]byte(s))
			require.NoError(t, err)
			require.Equal(t, n, len(s))
		}
	}()

	require.NoError(t, runWithOutput(cmd, fakeStdout, os.Stderr))
	done = true
}

type slowWriter struct {
	nWrites *int
}

func (s slowWriter) Write(b []byte) (int, error) {
	time.Sleep(100 * time.Millisecond)
	l := len(b)
	*s.nWrites += l
	return l, nil
}

func TestRunWithOutputDoesNotMissData(t *testing.T) {
	// This test ensures that runWithOutput writes all the data from the command and does not miss
	// any data that might be buffered when the command exits.
	t.Parallel()

	// Write `o` to stdout 100 times at 10 ms interval, followed by `x\n`
	// Write `e` to stderr 100 times at 10 ms interval, followed by `x\n`
	script := `let i = 0;
	let interval = setInterval(() => {
		process.stdout.write("o");
		process.stderr.write("e");
		i++;
		if (i == 100) {
			process.stdout.write("x\n");
			process.stderr.write("x\n");
			clearInterval(interval);
		}
	}, 10)
	`

	cmd := exec.Command("node", "-e", script)
	stdout := slowWriter{nWrites: new(int)}
	stderr := slowWriter{nWrites: new(int)}

	require.NoError(t, runWithOutput(cmd, stdout, stderr))

	require.Equal(t, 100+2 /* "x\n" */, *stdout.nWrites)
	require.Equal(t, 100+2 /* "x\n" */, *stderr.nWrites)
}
