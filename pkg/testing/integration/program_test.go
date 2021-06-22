// Copyright 2016-2018, Pulumi Corporation.
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

package integration

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Test that RunCommand writes the command's output to a log file.
func TestRunCommandLog(t *testing.T) {
	// Try to find node on the path. We need a program to run, and node is probably
	// available on all platforms where we're testing. If it's not found, skip the test.
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Couldn't find Node on PATH")
	}

	opts := &ProgramTestOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	tempdir, err := ioutil.TempDir("", "test")
	contract.AssertNoError(err)
	defer os.RemoveAll(tempdir)

	args := []string{node, "-e", "console.log('output from node');"}
	err = RunCommand(t, "node", args, tempdir, opts)
	assert.Nil(t, err)

	matches, err := filepath.Glob(filepath.Join(tempdir, commandOutputFolderName, "node.*"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(matches))

	output, err := ioutil.ReadFile(matches[0])
	assert.Nil(t, err)
	assert.Equal(t, "output from node\n", string(output))
}

func TestSanitizedPkg(t *testing.T) {
	v2 := getSanitizedModulePath("github.com/pulumi/pulumi-docker/sdk/v2")
	assert.Equal(t, "github.com/pulumi/pulumi-docker/sdk", v2)

	v3 := getSanitizedModulePath("github.com/pulumi/pulumi-aws/sdk/v3")
	assert.Equal(t, "github.com/pulumi/pulumi-aws/sdk", v3)

	nonVersion := getSanitizedModulePath("github.com/pulumi/pulumi-auth/sdk")
	assert.Equal(t, "github.com/pulumi/pulumi-auth/sdk", nonVersion)
}

func TestDepRootCalc(t *testing.T) {
	var dep string

	dep = getRewritePath("github.com/pulumi/pulumi-docker/sdk/v2", "/gopath", "")
	assert.Equal(t, "/gopath/src/github.com/pulumi/pulumi-docker/sdk", dep)

	dep = getRewritePath("github.com/pulumi/pulumi-gcp/sdk/v3", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/pulumi-gcp/sdk", dep)

	dep = getRewritePath("github.com/example/foo/pkg/v2", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/foo/pkg", dep)

	dep = getRewritePath("github.com/example/foo/v2", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/foo", dep)

	dep = getRewritePath("github.com/example/foo", "/gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/foo", dep)

	dep = getRewritePath("github.com/pulumi/pulumi-auth0/sdk", "gopath", "/my-go-src")
	assert.Equal(t, "/my-go-src/pulumi-auth0/sdk", dep)
}
