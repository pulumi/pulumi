// Copyright 2016, Pulumi Corporation.
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

package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
)

// GenPkgSignature corresponds to the shape of the codegen GeneratePackage functions.
type GenPkgSignature func(string, *schema.Package, map[string][]byte, schema.ReferenceLoader) (map[string][]byte, error)

// generatePackageFilesFromSchema loads a schema and generates files using the provided GeneratePackage function.
func generatePackageFilesFromSchema(
	schemaPath, loaderDir string, genPackageFunc GenPkgSignature,
) (map[string][]byte, error) {
	// Read in, decode, and import the schema.
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(schemaPath)

	var pkgSpec schema.PackageSpec
	if ext == ".yaml" || ext == ".yml" {
		err = yaml.Unmarshal(schemaBytes, &pkgSpec)
	} else {
		err = json.Unmarshal(schemaBytes, &pkgSpec)
	}
	if err != nil {
		return nil, err
	}

	loader := schema.NewPluginLoader(utils.NewContext(loaderDir))
	pkg, diags, err := schema.BindSpec(pkgSpec, loader, schema.ValidationOptions{
		AllowDanglingReferences: true,
	})
	if err != nil {
		return nil, err
	} else if diags.HasErrors() {
		return nil, diags
	}

	return genPackageFunc("test", pkg, nil, nil)
}

// LoadFiles loads the provided list of files from a directory.
func LoadFiles(dir, lang string, files []string) (map[string][]byte, error) {
	result := map[string][]byte{}
	for _, file := range files {
		fileBytes, err := os.ReadFile(filepath.Join(dir, lang, file))
		if err != nil {
			return nil, err
		}

		result[file] = fileBytes
	}

	return result, nil
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false, nil
	}

	if err == nil {
		return true, nil
	}

	return false, err
}

// `loadBaseline` loads the contents of the given baseline directory,
// by inspecting its `codegen-manifest.json`.
func loadBaseline(dir string) (map[string][]byte, error) {
	cm := &codegenManifest{}
	err := cm.load(dir)
	if err != nil {
		return nil, fmt.Errorf("Failed to load codegen-manifest.json: %w", err)
	}

	files := make(map[string][]byte)

	for _, f := range cm.EmittedFiles {
		bytes, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return nil, fmt.Errorf("Failed to load file %s referenced in codegen-manifest.json: %w", f, err)
		}
		files[f] = bytes
	}

	return files, nil
}

type codegenManifest struct {
	EmittedFiles []string `json:"emittedFiles"`
}

func (cm *codegenManifest) load(dir string) error {
	bytes, err := os.ReadFile(filepath.Join(dir, "codegen-manifest.json"))
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, cm)
}

func (cm *codegenManifest) save(dir string) error {
	sort.Strings(cm.EmittedFiles)
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(cm)
	if err != nil {
		return err
	}
	data := buf.Bytes()
	return os.WriteFile(filepath.Join(dir, "codegen-manifest.json"), data, 0o600)
}

// ValidateFileEquality compares maps of files for equality.
func ValidateFileEquality(t *testing.T, actual, expected map[string][]byte) bool {
	ok := true
	for name, file := range expected {
		_, inActual := actual[name]
		if inActual {
			if !assert.Equal(t, string(file), string(actual[name]), name) {
				t.Logf("%s did not agree", name)
				ok = false
			}
		} else {
			t.Logf("File %s was expected but is missing from the actual fileset", name)
			ok = false
		}
	}
	for name := range actual {
		if _, inExpected := expected[name]; !inExpected {
			t.Logf("File %s from the actual fileset was not expected", name)
			ok = false
		}
	}
	return ok
}

// If PULUMI_ACCEPT is set, writes out actual output to the expected
// file set, so we can continue enjoying golden tests without manually
// modifying the expected output.
func rewriteFilesWhenPulumiAccept(t *testing.T, path string, actual map[string][]byte) bool {
	if os.Getenv("PULUMI_ACCEPT") == "" {
		return false
	}

	cm := &codegenManifest{}

	baseline := path

	// Remove the baseline directory's current contents.
	_, err := os.ReadDir(baseline)
	switch {
	case err == nil:
		err = os.RemoveAll(baseline)
		require.NoError(t, err)
	case os.IsNotExist(err):
		// OK
	default:
		require.NoError(t, err)
	}

	for file, bytes := range actual {
		relPath := filepath.FromSlash(file)
		path := filepath.Join(path, relPath)
		cm.EmittedFiles = append(cm.EmittedFiles, relPath)
		err := writeFileEnsuringDir(path, bytes)
		require.NoError(t, err)
	}

	err = cm.save(path)
	require.NoError(t, err)

	return true
}

// Useful for populating code-generated destination
// `codeDir=$dir/$lang` with extra manually written files such as the
// unit test files. These files are copied from `$dir/$lang-extras`
// folder if present.
func copyExtraFiles(t *testing.T, codeDir string) {
	extrasDir := codeDir + "-extras"
	extraFiles, err := PathExists(extrasDir)

	if !extraFiles {
		return
	}

	if err != nil {
		require.NoError(t, err)
		return
	}

	err = filepath.Walk(extrasDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(extrasDir, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(codeDir, relPath)

		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		err = writeFileEnsuringDir(destPath, bytes)
		if err != nil {
			return err
		}
		t.Cleanup(func() { contract.IgnoreError(os.Remove(destPath)) })
		t.Logf("Copied %s to %s", path, destPath)
		return nil
	})

	require.NoError(t, err)
}

func writeFileEnsuringDir(path string, bytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !os.IsExist(err) {
		return err
	}

	return os.WriteFile(path, bytes, 0o600)
}

func RunCommand(t *testing.T, name string, cwd string, exec string, args ...string) {
	RunCommandWithOptions(t, &integration.ProgramTestOptions{}, name, cwd, exec, args...)
}

func RunCommandWithOptions(
	t *testing.T,
	opts *integration.ProgramTestOptions,
	name string, cwd string, exec string, args ...string,
) {
	stdout, stderr, err := tryRunCommand(t, opts, name, cwd, exec, args...)
	require.NoError(t, err, "stdout: %s\nstderr: %s", stdout, stderr)
}

// RunCommandWithRetries is like RunCommand but retries the command up to maxRetries times on failure.
// This is useful for commands that may fail due to transient network errors (e.g. npm install).
func RunCommandWithRetries(t *testing.T, name string, cwd string, maxRetries int, exec string, args ...string) {
	opts := &integration.ProgramTestOptions{}
	var stdout, stderr string
	var err error
	for attempt := range maxRetries {
		stdout, stderr, err = tryRunCommand(t, opts, name, cwd, exec, args...)
		if err == nil {
			return
		}
		if attempt < maxRetries-1 {
			t.Logf("Command %q failed (attempt %d/%d), retrying: %v", name, attempt+1, maxRetries, err)
		}
	}
	require.NoError(t, err, "stdout: %s\nstderr: %s", stdout, stderr)
}

// tryRunCommand runs a command and returns stdout, stderr, and any error without failing the test.
func tryRunCommand(
	t *testing.T,
	opts *integration.ProgramTestOptions,
	name string, cwd string, exec string, args ...string,
) (string, string, error) {
	exec, err := executable.FindExecutable(exec)
	if err != nil {
		return "", "", err
	}
	wd, err := filepath.Abs(cwd)
	if err != nil {
		return "", "", err
	}
	var stdout, stderr bytes.Buffer
	opts.Stdout = &stdout
	opts.Stderr = &stderr
	opts.Verbose = true
	err = integration.RunCommand(t,
		name,
		append([]string{exec}, args...),
		wd,
		opts)
	return stdout.String(), stderr.String(), err
}

type SchemaVersion = string

// Schemas are downloaded in the makefile, and the versions specified here
// should be in sync with the makefile.
const (
	RandomSchema SchemaVersion = "4.11.2"
)
