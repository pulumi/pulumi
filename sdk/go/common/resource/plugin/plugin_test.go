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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogFlowArgumentPropagation(t *testing.T) {
	t.Parallel()

	var engine = "127.0.0.1:12345"

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
