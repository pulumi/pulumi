// Copyright 2016-2022, Pulumi Corporation.
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
	"encoding/json"
	"os"
	"testing"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
)

func TestSchemaExtract(t *testing.T) {
	// Reset PULUMI_HOME to avoid contaminating global Pulumi
	// plugin store with the downloaded random resource plugin.
	pulumiHomeEnvVar := "PULUMI_HOME"
	oldHome := os.Getenv(pulumiHomeEnvVar)
	defer func() {
		if oldHome == "" {
			os.Unsetenv(pulumiHomeEnvVar)
		} else {
			os.Setenv(pulumiHomeEnvVar, oldHome)
		}
	}()
	err := os.Setenv(pulumiHomeEnvVar, t.TempDir())
	assert.NoError(t, err)

	var buf bytes.Buffer
	ver := semver.MustParse("4.3.1")
	err = schemaExtract(&buf, "random", &ver)
	assert.NoError(t, err)

	var spec map[string]interface{}
	err = json.NewDecoder(&buf).Decode(&spec)
	assert.NoError(t, err)
	assert.Equal(t, spec["name"].(string), "random")
}
