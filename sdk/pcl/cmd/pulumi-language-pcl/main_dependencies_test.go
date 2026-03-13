// Copyright 2016-2026, Pulumi Corporation.
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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadPclDependenciesInfersFromResourcesAndInvokes(t *testing.T) {
	t.Parallel()

	programDir := t.TempDir()
	source := `package aws {}

resource bucket "aws:s3/bucket:Bucket" {}
resource provider "pulumi:providers:kubernetes" {}

currentIdentity = invoke("aws:index:getCallerIdentity", {})
randomInt = invoke("random:index/getRandomInteger:getRandomInteger", {})
project = invoke("pulumi:pulumi:getProject", {})
`
	err := os.WriteFile(filepath.Join(programDir, "main.pp"), []byte(source), 0o600)
	require.NoError(t, err)

	deps, err := readPclDependencies(programDir)
	require.NoError(t, err)

	names := make([]string, len(deps))
	for i, dep := range deps {
		names[i] = dep.Name
	}
	require.Equal(t, []string{"aws", "kubernetes", "random"}, names)
}
