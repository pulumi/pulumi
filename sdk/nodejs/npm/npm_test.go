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

package npm

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	pulumi_testing "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

func chdir(t *testing.T, dir string) {
	cwd, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir(dir)) // Set directory
	t.Cleanup(func() {
		assert.NoError(t, os.Chdir(cwd)) // Restore directory
		restoredDir, err := os.Getwd()
		assert.NoError(t, err)
		assert.Equal(t, cwd, restoredDir)
	})
}

//nolint:paralleltest // mutates environment variables, changes working directory
func TestNPMInstall(t *testing.T) {
	testInstall(t, "npm", false /*production*/)
	testInstall(t, "npm", true /*production*/)
}

//nolint:paralleltest // mutates environment variables, changes working directory
func TestYarnInstall(t *testing.T) {
	t.Setenv("PULUMI_PREFER_YARN", "true")
	testInstall(t, "yarn", false /*production*/)
	testInstall(t, "yarn", true /*production*/)
}

func testInstall(t *testing.T, expectedBin string, production bool) {
	// Skip during short test runs since this test involves downloading dependencies.
	if testing.Short() {
		t.Skip("Skipped in short test run")
	}

	// Create a new empty test directory and change the current working directory to it.
	tempdir, _ := ioutil.TempDir("", "test-env")
	t.Cleanup(func() { os.RemoveAll(tempdir) })
	chdir(t, tempdir)

	// Create a package directory to install dependencies into.
	pkgdir := filepath.Join(tempdir, "package")
	assert.NoError(t, os.Mkdir(pkgdir, 0700))

	// Write out a minimal package.json file that has at least one dependency.
	packageJSONFilename := filepath.Join(pkgdir, "package.json")
	packageJSON := []byte(`{
	    "name": "test-package",
	    "dependencies": {
	        "@pulumi/pulumi": "latest"
	    }
	}`)
	assert.NoError(t, ioutil.WriteFile(packageJSONFilename, packageJSON, 0600))

	// Install dependencies, passing nil for stdout and stderr, which connects
	// them to the file descriptor for the null device (os.DevNull).
	pulumi_testing.YarnInstallMutex.Lock()
	defer pulumi_testing.YarnInstallMutex.Unlock()
	bin, err := Install(context.Background(), pkgdir, production, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedBin, bin)
}
