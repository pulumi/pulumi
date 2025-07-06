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

package about

import (
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLI(t *testing.T) {
	t.Parallel()

	cli := getCLIAbout()
	assert.Equal(t, cli.GoVersion, runtime.Version())
	assert.Equal(t, cli.GoCompiler, runtime.Compiler)
}

func TestBackend(t *testing.T) {
	t.Parallel()

	stats, err := host.Info()
	if err != nil {
		t.Skipf("Underlying stats call failed: %s", err)
	}
	backend, err := getHostAbout()
	require.NoError(t, err, "We should be able to get stats here")
	display := backend.String()
	assert.Contains(t, display, stats.Platform)
	assert.Contains(t, display, stats.PlatformVersion)
	assert.Contains(t, display, stats.KernelArch)
}
