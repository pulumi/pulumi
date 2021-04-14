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
	"strings"
	"testing"

	"github.com/blang/semver"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
)

func TestArgumentConstruction(t *testing.T) {
	t.Parallel()

	t.Run("DryRun-NoArguments", func(tt *testing.T) {
		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{DryRun: true}
		args := host.constructArguments(rr, "", "")
		assert.Contains(tt, args, "--dry-run")
		assert.NotContains(tt, args, "true")
	})

	t.Run("OptionalArgs-PassedIfSpecified", func(tt *testing.T) {
		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Project: "foo"}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
		assert.Contains(tt, args, "--project foo")
	})

	t.Run("OptionalArgs-NotPassedIfNotSpecified", func(tt *testing.T) {
		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
		assert.NotContains(tt, args, "--stack")
	})

	t.Run("DotIfProgramNotSpecified", func(tt *testing.T) {
		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
		assert.Contains(tt, args, ".")
	})

	t.Run("ProgramIfProgramSpecified", func(tt *testing.T) {
		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Program: "foobar"}
		args := strings.Join(host.constructArguments(rr, "", ""), " ")
		assert.Contains(tt, args, "foobar")
	})
}

func TestConfig(t *testing.T) {
	t.Parallel()
	t.Run("Config-Empty", func(tt *testing.T) {
		host := &nodeLanguageHost{}
		rr := &pulumirpc.RunRequest{Project: "foo"}
		str, err := host.constructConfig(rr)
		assert.NoError(tt, err)
		assert.JSONEq(tt, "{}", str)
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
