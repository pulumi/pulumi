// Copyright 2016-2020, Pulumi Corporation.
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

//go:build nodejs || all
// +build nodejs all

package workspace

import (
	"os"
	"testing"
)

var tarball = map[string][]byte{
	"PulumiPlugin.yaml": []byte("runtime: nodejs\n"),
	"package.json":      []byte(`{"name":"test","dependencies":{"@pulumi/pulumi":"^2.0.0"}}`),
}

func TestNodeNPMInstall(t *testing.T) {
	t.Parallel()
	testPluginInstall(t, "node_modules", tarball)
}

//nolint:paralleltest // mutates environment variables
func TestNodeYarnInstall(t *testing.T) {
	os.Setenv("PULUMI_PREFER_YARN", "true")
	testPluginInstall(t, "node_modules", tarball)
}
