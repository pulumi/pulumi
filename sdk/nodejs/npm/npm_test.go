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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNPMInstall(t *testing.T) {
	testInstall(t, "npm", false /*production*/)
	testInstall(t, "npm", true /*production*/)
}

func TestYarnInstall(t *testing.T) {
	os.Setenv("PULUMI_PREFER_YARN", "true")
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
	defer os.RemoveAll(tempdir)
	assert.NoError(t, os.Chdir(tempdir))

	// Create a package directory to install dependencies into.
	pkgdir := filepath.Join(tempdir, "package")
	assert.NoError(t, os.Mkdir(pkgdir, 0700))

	// Write out a minimal package.json file that has at least one dependency.
	packageJSONFilename := filepath.Join(pkgdir, "package.json")
	packageJSON := []byte(`{
	    "name": "test-package",
	    "dependencies": {
	        "@pulumi/pulumi": "^2.0.0"
	    }
	}`)
	assert.NoError(t, ioutil.WriteFile(packageJSONFilename, packageJSON, 0600))

	// Install dependencies, passing nil for stdout and stderr, which connects
	// them to the file descriptor for the null device (os.DevNull).
	bin, err := Install(pkgdir, production, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedBin, bin)
}
