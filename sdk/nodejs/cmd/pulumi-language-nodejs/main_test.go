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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestGetRequiredPackages(t *testing.T) {
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
		{
			filepath.Join(dir, "node_modules", "malformed", "tests", "false_main", "package.json"),
			`{ "name": "false_main", "main": false }`,
		},
		{
			filepath.Join(dir, "node_modules", "malformed", "tests", "invalid_main", "package.json"),
			`{ "name": "invalid_main", "main": ["this is an", "invalid main"]}`,
		},
	}
	for _, file := range files {
		err := os.MkdirAll(filepath.Dir(file.path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(file.path, []byte(file.content), 0o600)
		require.NoError(t, err)
	}

	host := &nodeLanguageHost{}
	resp, err := host.GetRequiredPackages(context.Background(), &pulumirpc.GetRequiredPackagesRequest{
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.Packages {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
	}, actual)
}

func TestGetRequiredPackagesSymlinkCycles(t *testing.T) {
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
	resp, err := host.GetRequiredPackages(context.Background(), &pulumirpc.GetRequiredPackagesRequest{
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.Packages {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
	}, actual)
}

func TestGetRequiredPackagesSymlinkCycles2(t *testing.T) {
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
	resp, err := host.GetRequiredPackages(context.Background(), &pulumirpc.GetRequiredPackagesRequest{
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.Packages {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
	}, actual)
}

func TestGetRequiredPackagesNestedPolicyPack(t *testing.T) {
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
	resp, err := host.GetRequiredPackages(context.Background(), &pulumirpc.GetRequiredPackagesRequest{
		Info: &pulumirpc.ProgramInfo{
			RootDirectory:    dir,
			ProgramDirectory: dir,
			EntryPoint:       ".",
		},
	})
	require.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.Packages {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
		// baz: v7.8.9 is not included because it is in a nested policy pack
	}, actual)
}

type filePathAndContents struct {
	path    string
	content string
}

func setupFiles(t *testing.T, files []filePathAndContents) string {
	dir := filepath.Join(t.TempDir(), "program-dependency-testdir")
	err := os.Mkdir(dir, 0o755)
	require.NoError(t, err)

	for _, file := range files {
		file.path = filepath.Join(dir, file.path)
		err := os.MkdirAll(filepath.Dir(file.path), 0o755)
		require.NoError(t, err)
		err = os.WriteFile(file.path, []byte(file.content), 0o600)
		require.NoError(t, err)
	}
	return dir
}

func TestGetProgramDependencies(t *testing.T) {
	t.Parallel()

	t.Run("With no package.json, no lock files", func(t *testing.T) {
		t.Parallel()

		testDir := setupFiles(t, []filePathAndContents{
			{
				path:    "Pulumi.yaml",
				content: `name: test`,
			},
		})
		host := &nodeLanguageHost{}
		_, err := host.GetProgramDependencies(context.Background(), &pulumirpc.GetProgramDependenciesRequest{
			Program: testDir,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    testDir,
				ProgramDirectory: testDir,
				EntryPoint:       ".",
			},
		})
		require.ErrorContains(t, err, "no package-lock.json or yarn.lock file found (searching upwards from")
	})

	t.Run("With package.json in project root, no lock files", func(t *testing.T) {
		t.Parallel()

		testDir := setupFiles(t, []filePathAndContents{
			{
				path:    "package.json",
				content: `{ "name": "@pulumi/baz", "dependencies": { "@pulumi/pulumi": "^3.113.0" } }`,
			},
			{
				path:    "Pulumi.yaml",
				content: `name: test`,
			},
		})
		host := &nodeLanguageHost{}
		_, err := host.GetProgramDependencies(context.Background(), &pulumirpc.GetProgramDependenciesRequest{
			Program: testDir,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    testDir,
				ProgramDirectory: testDir,
				EntryPoint:       ".",
			},
		})
		require.ErrorContains(t, err, "no package-lock.json or yarn.lock file found (searching upwards from")
	})

	t.Run("With package.json and yarn.lock in project root", func(t *testing.T) {
		t.Parallel()

		testDir := setupFiles(t, []filePathAndContents{
			{
				path:    "package.json",
				content: `{ "name": "@pulumi/baz", "dependencies": { "@pulumi/pulumi": "^3.113.0" } }`,
			},
			{
				path:    "Pulumi.yaml",
				content: `name: test`,
			},
			{
				path: "yarn.lock",
				content: `"@pulumi/pulumi@^3.0.0", "@pulumi/pulumi@^3.113.0":
  version "3.131.0"
  resolved "https://registry.yarnpkg.com/@pulumi/pulumi/-/pulumi-3.131.0.tgz#6233e5ee5e72907b99415b32be6a9ebf9041f096"
  integrity sha512-QNtQeav3dkU0mRdMe2TVvkBmIGkBevVvbD7/bt0fJlGoX/onzv5tysqi1GWCkXsq0FKtBtGYNpVD6wH0cqMN6g==
  dependencies:
    "@grpc/grpc-js" "^1.10.1"
    "@logdna/tail-file" "^2.0.6"
    "@npmcli/arborist" "^7.3.1"`,
			},
		})
		host := &nodeLanguageHost{}
		resp, err := host.GetProgramDependencies(context.Background(), &pulumirpc.GetProgramDependenciesRequest{
			Program: testDir,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    testDir,
				ProgramDirectory: testDir,
				EntryPoint:       ".",
			},
		})
		require.NoError(t, err)
		require.Equal(t, len(resp.Dependencies), 1)
		require.Equal(t, resp.Dependencies[0].Name, "@pulumi/pulumi")
		require.Equal(t, resp.Dependencies[0].Version, "3.131.0")
	})

	t.Run("With package.json and yarn.lock in parent dir", func(t *testing.T) {
		t.Parallel()

		testDir := setupFiles(t, []filePathAndContents{
			{
				path:    "package.json",
				content: `{ "name": "@pulumi/baz", "dependencies": { "@pulumi/pulumi": "^3.113.0" } }`,
			},
			{
				path:    filepath.Join("subdir", "Pulumi.yaml"),
				content: `name: test`,
			},
			{
				path: "yarn.lock",
				content: `"@pulumi/pulumi@^3.0.0", "@pulumi/pulumi@^3.113.0":
  version "3.131.0"
  resolved "https://registry.yarnpkg.com/@pulumi/pulumi/-/pulumi-3.131.0.tgz#6233e5ee5e72907b99415b32be6a9ebf9041f096"
  integrity sha512-QNtQeav3dkU0mRdMe2TVvkBmIGkBevVvbD7/bt0fJlGoX/onzv5tysqi1GWCkXsq0FKtBtGYNpVD6wH0cqMN6g==
  dependencies:
    "@grpc/grpc-js" "^1.10.1"
    "@logdna/tail-file" "^2.0.6"
    "@npmcli/arborist" "^7.3.1"`,
			},
		})
		subdir := filepath.Join(testDir, "subdir")
		host := &nodeLanguageHost{}
		resp, err := host.GetProgramDependencies(context.Background(), &pulumirpc.GetProgramDependenciesRequest{
			Program: subdir,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    subdir,
				ProgramDirectory: subdir,
				EntryPoint:       ".",
			},
		})
		require.NoError(t, err)
		require.Equal(t, len(resp.Dependencies), 1)
		require.Equal(t, resp.Dependencies[0].Name, "@pulumi/pulumi")
		require.Equal(t, resp.Dependencies[0].Version, "3.131.0")
	})

	t.Run("With package.json and yarn.lock in project root", func(t *testing.T) {
		t.Parallel()

		testDir := setupFiles(t, []filePathAndContents{
			{
				path:    "package.json",
				content: `{ "name": "@pulumi/baz", "dependencies": { "@pulumi/pulumi": "^3.113.0" } }`,
			},
			{
				path:    "Pulumi.yaml",
				content: `name: test`,
			},
			{
				path: "yarn.lock",
				content: `"@pulumi/pulumi@^3.0.0", "@pulumi/pulumi@^3.113.0":
  version "3.131.0"
  resolved "https://registry.yarnpkg.com/@pulumi/pulumi/-/pulumi-3.131.0.tgz#6233e5ee5e72907b99415b32be6a9ebf9041f096"
  integrity sha512-QNtQeav3dkU0mRdMe2TVvkBmIGkBevVvbD7/bt0fJlGoX/onzv5tysqi1GWCkXsq0FKtBtGYNpVD6wH0cqMN6g==
  dependencies:
    "@grpc/grpc-js" "^1.10.1"
    "@logdna/tail-file" "^2.0.6"
    "@npmcli/arborist" "^7.3.1"`,
			},
		})
		host := &nodeLanguageHost{}
		resp, err := host.GetProgramDependencies(context.Background(), &pulumirpc.GetProgramDependenciesRequest{
			Program: testDir,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    testDir,
				ProgramDirectory: testDir,
				EntryPoint:       ".",
			},
		})
		require.NoError(t, err)
		require.Equal(t, len(resp.Dependencies), 1)
		require.Equal(t, resp.Dependencies[0].Name, "@pulumi/pulumi")
		require.Equal(t, resp.Dependencies[0].Version, "3.131.0")
	})

	t.Run("With package.json and package-lock.json in project root", func(t *testing.T) {
		t.Parallel()

		testDir := setupFiles(t, []filePathAndContents{
			{
				path:    "package.json",
				content: `{ "dependencies": { "random": "^5.1.0" } }`,
			},
			{
				path:    "Pulumi.yaml",
				content: `name: test`,
			},
			{
				path: "package-lock.json",
				content: `{
  "name": "pulumi-repos",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "dependencies": {
        "random": "^5.1.0"
      }
    },
    "node_modules/random": {
      "version": "5.1.0",
      "resolved": "https://registry.npmjs.org/random/-/random-5.1.0.tgz",
      "integrity": "sha512-0NGG4HMW9sTstLbignEDasSQJlCGkNQZICIWStZ+h4SzSJfZXpecGKV7qL0AOKcIT8XX9pJ49uZnvI0n/Y+vWA==",
      "engines": {
        "node": ">=18"
      }
    }
  }
}`,
			},
			{
				path:    filepath.Join("node_modules", "random", "package.json"),
				content: `{ "name": "random", "version": "5.1.0", "type": "module" }`,
			},
		})
		host := &nodeLanguageHost{}
		resp, err := host.GetProgramDependencies(context.Background(), &pulumirpc.GetProgramDependenciesRequest{
			Program: testDir,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    testDir,
				ProgramDirectory: testDir,
				EntryPoint:       ".",
			},
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Dependencies))
		require.Equal(t, "random", resp.Dependencies[0].Name)
		require.Equal(t, "5.1.0", resp.Dependencies[0].Version)
	})

	t.Run("With package.json and package-lock.json in parent dir", func(t *testing.T) {
		t.Parallel()

		testDir := setupFiles(t, []filePathAndContents{
			{
				path:    "package.json",
				content: `{ "dependencies": { "random": "^5.1.0" } }`,
			},
			{
				path:    filepath.Join("subdir", "Pulumi.yaml"),
				content: `name: test`,
			},
			{
				path: "package-lock.json",
				content: `{
  "name": "pulumi-repos",
  "lockfileVersion": 3,
  "requires": true,
  "packages": {
    "": {
      "dependencies": {
        "random": "^5.1.0"
      }
    },
    "node_modules/random": {
      "version": "5.1.0",
      "resolved": "https://registry.npmjs.org/random/-/random-5.1.0.tgz",
      "integrity": "sha512-0NGG4HMW9sTstLbignEDasSQJlCGkNQZICIWStZ+h4SzSJfZXpecGKV7qL0AOKcIT8XX9pJ49uZnvI0n/Y+vWA==",
      "engines": {
        "node": ">=18"
      }
    }
  }
}`,
			},
			{
				path:    filepath.Join("node_modules", "random", "package.json"),
				content: `{ "name": "random", "version": "5.1.0", "type": "module" }`,
			},
		})

		subdir := filepath.Join(testDir, "subdir")
		host := &nodeLanguageHost{}
		resp, err := host.GetProgramDependencies(context.Background(), &pulumirpc.GetProgramDependenciesRequest{
			Program: subdir,
			Info: &pulumirpc.ProgramInfo{
				RootDirectory:    subdir,
				ProgramDirectory: subdir,
				EntryPoint:       ".",
			},
		})
		require.NoError(t, err)
		require.Equal(t, 1, len(resp.Dependencies))
		require.Equal(t, "random", resp.Dependencies[0].Name)
		require.Equal(t, "5.1.0", resp.Dependencies[0].Version)
	})
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

// Nodejs sometimes sets stdout/stderr to non-blocking mode. When a nodejs subprocess is directly
// handed the go process's stdout/stderr file descriptors, nodejs's non-blocking configuration goes
// unnoticed by go, and a write from go can result in an error `write /dev/stdout: resource
// temporarily unavailable`. See runWithOutput for more details.
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

//nolint:paralleltest // mutates environment variables
func TestUseFnm(t *testing.T) {
	// Set $PATH to to $TMPDIR/bin so that no `fnm` executable can be found.
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "bin"), 0o755))
	t.Setenv("PATH", filepath.Join(tmpDir, "bin"))

	_, err := useFnm(tmpDir)
	require.ErrorIs(t, err, errFnmNotFound)

	// Add a fake fnm binary to $TMPDIR/bin for the rest of the tests.
	//nolint:gosec // we want this file to be executable
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bin", "fnm"), []byte("#!/bin/sh\nexit 0;\n"), 0o700))

	t.Run("no version files", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		_, err := useFnm(tmpDir)
		require.ErrorIs(t, err, errVersionFileNotFound)
	})

	t.Run(".node-version in cwd", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".node-version"), []byte("22.7.3"), 0o600))
		version, err := useFnm(tmpDir)
		require.NoError(t, err)
		require.Equal(t, "22.7.3", version)
	})

	t.Run(".nvmrc in cwd", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".nvmrc"), []byte("20.1.1"), 0o600))
		version, err := useFnm(tmpDir)
		require.NoError(t, err)
		require.Equal(t, "20.1.1", version)
	})

	t.Run(".nvmrc & .node-version in cwd", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// .nvmrc should take precedence
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".nvmrc"), []byte("20.1.1"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".node-version"), []byte("22.7.3"), 0o600))
		version, err := useFnm(tmpDir)
		require.NoError(t, err)
		require.Equal(t, "20.1.1", version)
	})

	t.Run(".node-version in parent folder", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		tmpDirNested := filepath.Join(tmpDir, "nested")
		require.NoError(t, os.MkdirAll(tmpDirNested, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".node-version"), []byte("20.1.1"), 0o600))
		version, err := useFnm(tmpDirNested)
		require.NoError(t, err)
		require.Equal(t, "20.1.1", version)
	})

	t.Run(".node-version in cwd & parent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		tmpDirNested := filepath.Join(tmpDir, "nested")
		require.NoError(t, os.MkdirAll(tmpDirNested, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".node-version"), []byte("20.1.1"), 0o600))
		// This should take precedence over the parent folder's .node-version
		require.NoError(t, os.WriteFile(filepath.Join(tmpDirNested, ".node-version"), []byte("20.7.3"), 0o600))
		version, err := useFnm(tmpDirNested)
		require.NoError(t, err)
		require.Equal(t, "20.7.3", version)
	})
}

//nolint:paralleltest // mutates environment variables
func TestNodeInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip()
	}
	tmpDir := t.TempDir()
	t.Setenv("PATH", filepath.Join(tmpDir, "bin"))

	fmt.Println(os.Getenv("PATH"))

	// There's no fnm executable in PATH, installNodeVersion is a no-op
	stdout := &bytes.Buffer{}
	err := installNodeVersion(tmpDir, stdout)
	require.ErrorIs(t, err, errFnmNotFound)

	// Add a mock fnm executable to $tmp/bin. For each execution, the mock fnm executable will
	// append a line with its arguments to a file. We read back the file to verify that it was
	// called with the expected arguments.
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "bin"), 0o755))
	outPath := filepath.Join(tmpDir, "out.txt")
	script := fmt.Sprintf("#!/bin/sh\necho $@ >> %s\n", outPath)
	//nolint:gosec // we want this file to be executable
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bin", "fnm"), []byte(script), 0o700))

	// There's no .node-version or .nvmrc file, so the binary should not be called.
	// We expect the file written by our mock fnm executable to not exist.
	stdout = &bytes.Buffer{}
	err = installNodeVersion(tmpDir, stdout)
	require.Error(t, err, errVersionFileNotFound)

	// Create a .node-version file
	// The mock fnm executable should be called with a command to install the requested version,
	// and a command to set the default version to this version.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".node-version"), []byte("20.1.2"), 0o600))
	stdout = &bytes.Buffer{}
	err = installNodeVersion(tmpDir, stdout)
	require.NoError(t, err)
	b, err := os.ReadFile(outPath)
	require.NoError(t, err)
	commands := strings.Split(strings.TrimSpace(string(b)), "\n")
	require.Equal(t, "install 20.1.2 --progress never", commands[0])
	require.Equal(t, "alias 20.1.2 default", commands[1])
}
