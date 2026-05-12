// Copyright 2026, Pulumi Corporation.
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

package packagecmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestValidatePackageName(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"a", "foo", "foo-bar", "foo_bar", "foo123", "a1"} {
		assert.NoErrorf(t, validatePackageName(name), "expected %q to be valid", name)
	}

	cases := map[string]string{
		"":                       "may not be empty",
		strings.Repeat("a", 101): "100 characters",
		"Foo":                    "package names must start with",
		"foo-Bar":                "package names must start with",
		"foo.bar":                "package names must start with",
		"foo bar":                "package names must start with",
		"foo@bar":                "package names must start with",
		"-foo":                   "package names must start with",
		"_foo":                   "package names must start with",
		"123foo":                 "package names must start with",
		"pulumi":                 "must not be `pulumi`",
		"pulumi-foo":             "must not be `pulumi`",
	}
	for input, wantSubstring := range cases {
		err := validatePackageName(input)
		require.Errorf(t, err, "expected %q to be invalid", input)
		assert.Containsf(t, err.Error(), wantSubstring, "input %q", input)
	}
}

func TestDefaultPackageNameFromCwd(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "foo", defaultPackageNameFromCwd("/tmp/foo"))
	assert.Equal(t, "foo-bar", defaultPackageNameFromCwd("/tmp/foo-bar"))
	assert.Equal(t, "myapp", defaultPackageNameFromCwd("/tmp/MyApp"))
	assert.Equal(t, "foobar", defaultPackageNameFromCwd("/tmp/foo.bar"))
	assert.Equal(t, "foobar", defaultPackageNameFromCwd("/tmp/foo bar"))
	assert.Equal(t, "foo", defaultPackageNameFromCwd("/tmp/123foo"))
	assert.Equal(t, "package", defaultPackageNameFromCwd("/tmp/!@#$"))
	assert.Equal(t, "package", defaultPackageNameFromCwd("/tmp/pulumi"))
}

//nolint:paralleltest // changes process working directory
func TestPackageNew_GenerateOnly(t *testing.T) {
	templatePath, err := filepath.Abs(filepath.Join("testdata", "template-component-nodejs"))
	require.NoError(t, err)
	dest := t.TempDir()
	t.Chdir(dest)

	args := newPackageArgs{
		templateNameOrURL: templatePath,
		name:              "my-component",
		description:       "An example component",
		yes:               true,
		generateOnly:      true,
	}
	var out bytes.Buffer
	require.NoError(t, runNewPackage(t.Context(), &out, args))

	output := out.String()
	assert.Contains(t, output, "Created package 'my-component'")
	assert.Contains(t, output, "pulumi install")

	pluginPath := filepath.Join(dest, "PulumiPlugin.yaml")
	assert.FileExists(t, pluginPath)
	assert.FileExists(t, filepath.Join(dest, "package.json"))
	assert.FileExists(t, filepath.Join(dest, "index.ts"))

	plugin, err := workspace.LoadPluginProject(pluginPath)
	require.NoError(t, err)
	assert.Nil(t, plugin.Template, "Template block should be stripped from the saved manifest")
	assert.Equal(t, "nodejs", plugin.Runtime.Name())

	pkgJSON, err := os.ReadFile(filepath.Join(dest, "package.json"))
	require.NoError(t, err)
	assert.Contains(t, string(pkgJSON), `"name": "my-component"`)
	assert.Contains(t, string(pkgJSON), `"description": "An example component"`)

	indexTS, err := os.ReadFile(filepath.Join(dest, "index.ts"))
	require.NoError(t, err)
	assert.Contains(t, string(indexTS), `"my-component:index:MyComponent"`)

	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "# my-component")
	assert.Contains(t, string(readme), "An example component")
}

//nolint:paralleltest // changes process working directory
func TestPackageNew_DefaultsNameAndDescription(t *testing.T) {
	templatePath, err := filepath.Abs(filepath.Join("testdata", "template-component-nodejs"))
	require.NoError(t, err)
	parent := t.TempDir()
	dest := filepath.Join(parent, "awesome-pkg")
	require.NoError(t, os.Mkdir(dest, 0o700))
	t.Chdir(dest)

	args := newPackageArgs{
		templateNameOrURL: templatePath,
		// Intentionally omit name and description; expect name to default to the
		// cwd basename and description to default to the template's description.
		yes:          true,
		generateOnly: true,
	}
	require.NoError(t, runNewPackage(t.Context(), io.Discard, args))

	pkgJSON, err := os.ReadFile(filepath.Join(dest, "package.json"))
	require.NoError(t, err)
	assert.Contains(t, string(pkgJSON), `"name": "awesome-pkg"`)
	assert.Contains(t, string(pkgJSON), `"description": "A Node.js component package starter"`)

	indexTS, err := os.ReadFile(filepath.Join(dest, "index.ts"))
	require.NoError(t, err)
	assert.Contains(t, string(indexTS), `"awesome-pkg:index:MyComponent"`)

	readme, err := os.ReadFile(filepath.Join(dest, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "# awesome-pkg")
	assert.Contains(t, string(readme), "A Node.js component package starter")
}

//nolint:paralleltest // changes process working directory
func TestPackageNew_NonEmptyDirRequiresForce(t *testing.T) {
	templatePath, err := filepath.Abs(filepath.Join("testdata", "template-component-nodejs"))
	require.NoError(t, err)
	dest := t.TempDir()
	t.Chdir(dest)

	require.NoError(t, os.WriteFile(filepath.Join(dest, "preexisting.txt"), []byte("hi"), 0o600))

	args := newPackageArgs{
		templateNameOrURL: templatePath,
		name:              "my-component",
		description:       "An example component",
		yes:               true,
		generateOnly:      true,
	}
	err = runNewPackage(t.Context(), io.Discard, args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not empty")

	// With --force, the same call should succeed.
	args.force = true
	require.NoError(t, runNewPackage(t.Context(), io.Discard, args))
	assert.FileExists(t, filepath.Join(dest, "PulumiPlugin.yaml"))
	assert.FileExists(t, filepath.Join(dest, "preexisting.txt"))
}

//nolint:paralleltest // changes process working directory
func TestPackageNew_BrokenTemplate(t *testing.T) {
	templatePath, err := filepath.Abs(filepath.Join("testdata", "template-broken"))
	require.NoError(t, err)
	dest := t.TempDir()
	t.Chdir(dest)

	args := newPackageArgs{
		templateNameOrURL: templatePath,
		name:              "my-component",
		description:       "An example component",
		yes:               true,
		generateOnly:      true,
	}
	err = runNewPackage(t.Context(), io.Discard, args)
	require.Error(t, err)
}

//nolint:paralleltest // changes process working directory
func TestPackageNew_DirFlagCreatesAndUsesDir(t *testing.T) {
	templatePath, err := filepath.Abs(filepath.Join("testdata", "template-component-nodejs"))
	require.NoError(t, err)
	parent := t.TempDir()
	t.Chdir(parent)

	target := filepath.Join(parent, "subdir")
	args := newPackageArgs{
		templateNameOrURL: templatePath,
		dir:               target,
		name:              "my-component",
		description:       "An example component",
		yes:               true,
		generateOnly:      true,
	}
	require.NoError(t, runNewPackage(t.Context(), io.Discard, args))
	assert.FileExists(t, filepath.Join(target, "PulumiPlugin.yaml"))
}

//nolint:paralleltest // changes process working directory
func TestPackageNew_InvalidName(t *testing.T) {
	templatePath, err := filepath.Abs(filepath.Join("testdata", "template-component-nodejs"))
	require.NoError(t, err)

	for _, name := range []string{"not a valid name", "MyComponent", "foo.bar"} {
		t.Run(name, func(t *testing.T) {
			dest := t.TempDir()
			t.Chdir(dest)

			args := newPackageArgs{
				templateNameOrURL: templatePath,
				name:              name,
				description:       "An example component",
				yes:               true,
				generateOnly:      true,
			}
			err := runNewPackage(t.Context(), io.Discard, args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not a valid package name")
		})
	}
}

func TestPackageTemplatesToOptions(t *testing.T) {
	t.Parallel()

	templates := []workspace.PackageTemplate{
		{Name: "zeta", Description: "last alphabetically"},
		{Name: "alpha", Description: "first alphabetically"},
		{Name: "broken", Error: assert.AnError},
	}

	options, lookup := packageTemplatesToOptions(templates)
	require.Len(t, options, 3)
	// Valid templates come first, sorted; broken last.
	assert.Contains(t, options[0], "alpha")
	assert.Contains(t, options[1], "zeta")
	assert.Contains(t, options[2], "broken")
	assert.Equal(t, "alpha", lookup[options[0]].Name)
	assert.Equal(t, "broken", lookup[options[2]].Name)
}

func TestLoadPackageTemplate(t *testing.T) {
	t.Parallel()
	t.Run("loads template metadata", func(t *testing.T) {
		t.Parallel()
		path, err := filepath.Abs(filepath.Join("testdata", "template-component-nodejs"))
		require.NoError(t, err)
		template, err := workspace.LoadPackageTemplate(path)
		require.NoError(t, err)
		assert.Equal(t, "template-component-nodejs", template.Name)
		assert.Equal(t, "A Node.js component package starter", template.Description)
		assert.Contains(t, template.Quickstart, "Edit index.ts")
	})
	t.Run("returns error for broken manifest", func(t *testing.T) {
		t.Parallel()
		path, err := filepath.Abs(filepath.Join("testdata", "template-broken"))
		require.NoError(t, err)
		_, err = workspace.LoadPackageTemplate(path)
		require.Error(t, err)
	})
}
