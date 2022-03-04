// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     htp://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
)

func TestArgumentConstruction(t *testing.T) {
	t.Parallel()

	t.Run("DryRun-NoArguments", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{DryRun: true}
		args := host.constructArguments(rr, "", "")
		assert.Contains(t, args, "--dry-run")
		assert.NotContains(t, args, "true")
	})

	t.Run("OptionalArgs-PassedIfSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Project: "foo"}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
		assert.Contains(t, args, "--project foo")
	})

	t.Run("OptionalArgs-NotPassedIfNotSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
		assert.NotContains(t, args, "--stack")
	})

	t.Run("DotIfProgramNotSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
		assert.Contains(t, args, ".")
	})

	t.Run("ProgramIfProgramSpecified", func(t *testing.T) {
		t.Parallel()

		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Program: "foobar"}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
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

	dir, err := ioutil.TempDir("", "test-dir")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

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
		err := os.MkdirAll(filepath.Dir(file.path), 0755)
		assert.NoError(t, err)
		err = os.WriteFile(file.path, []byte(file.content), 0600)
		assert.NoError(t, err)
	}

	host := &nodeLanguageHost{}
	resp, err := host.GetRequiredPlugins(context.TODO(), &pulumirpc.GetRequiredPluginsRequest{
		Program: dir,
	})
	assert.NoError(t, err)

	actual := make(map[string]string)
	for _, plugin := range resp.GetPlugins() {
		actual[plugin.Name] = plugin.Version
	}
	assert.Equal(t, map[string]string{
		"foo": "v1.2.3",
		"bar": "v4.5.6",
	}, actual)
}
