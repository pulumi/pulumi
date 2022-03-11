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

package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/python"
	"github.com/shirou/gopsutil/host"
	"github.com/stretchr/testify/assert"
)

func TestCLI(t *testing.T) {
	t.Parallel()

	cli := getCLIAbout()
	assert.Equal(t, cli.GoVersion, runtime.Version())
	assert.Equal(t, cli.GoCompiler, runtime.Compiler)
}

func TestProjectRuntime(t *testing.T) {
	t.Parallel()

	cmd, err := python.Command("--version")
	var out []byte
	if err != nil {
		t.Skip("Python needs to be in path for this func Test")
	}
	out, err = cmd.Output()
	assert.NoError(t, err, "This should not fail")
	version := strings.TrimSpace("v" + strings.TrimPrefix(string(out), "Python "))

	var runtime projectRuntimeAbout
	runtime, err = getProjectRuntimeAbout(&workspace.Project{
		Name:    "TestProject",
		Runtime: workspace.NewProjectRuntimeInfo("python", make(map[string]interface{})),
	})
	assert.NoError(t, err)
	assert.Equal(t, runtime.Language, "python")
	assert.Equal(t, runtime.Version, version)
}

func TestBackend(t *testing.T) {
	t.Parallel()

	stats, err := host.Info()
	if err != nil {
		t.Skipf("Underlying stats call failed: %s", err)
	}
	backend, err := getHostAbout()
	assert.NoError(t, err, "We should be able to get stats here")
	display := backend.String()
	assert.Contains(t, display, stats.Platform)
	assert.Contains(t, display, stats.PlatformVersion)
	assert.Contains(t, display, stats.KernelArch)
}
