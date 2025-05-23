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

//go:build python || all

package workspace

import (
	"testing"
)

func TestPythonInstall(t *testing.T) {
	t.Parallel()
	testPluginInstall(t, "venv", map[string][]byte{
		"PulumiPlugin.yaml": []byte("runtime: python\n"),
		"package.json":      []byte("pulumi==2.0.0\n"),
		"__main__.py":       []byte("print('hello')\n"),
	})
}

func TestPythonInstallPackage(t *testing.T) {
	t.Parallel()
	testPluginInstall(t, "venv", map[string][]byte{
		"PulumiPlugin.yaml": []byte("runtime: python\n"),
		"pyproject.toml": []byte(`[project]
								name = "plugin-package"
								version = "0.1.0"
								dependencies = []
								[build-system]
								requires = ["setuptools"]
								build-backend = "setuptools.build_meta"`),
		"src/plugin_package/__main__.py": []byte("print('hello')\n"),
	})
}
