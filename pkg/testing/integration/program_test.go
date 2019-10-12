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
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

func TestPrefixer(t *testing.T) {
	t.Parallel()
	byts := make([]byte, 0, 1000)
	buf := bytes.NewBuffer(byts)
	prefixer := newPrefixer(buf, "OK: ")
	_, err := prefixer.Write([]byte("\nsadsada\n\nasdsadsa\nasdsadsa\n"))
	contract.AssertNoError(err)
	assert.Equal(t, []byte("OK: \nOK: sadsada\nOK: \nOK: asdsadsa\nOK: asdsadsa\n"), buf.Bytes())
}

// Test that RunCommand writes the command's output to a log file.
func TestRunCommandLog(t *testing.T) {
	t.Parallel()
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
	err = RunCommand(nil, "node", args, tempdir, opts)
	assert.Nil(t, err)

	matches, err := filepath.Glob(filepath.Join(tempdir, commandOutputFolderName, "node.*"))
	assert.Nil(t, err)
	assert.Equal(t, 1, len(matches))

	output, err := ioutil.ReadFile(matches[0])
	assert.Nil(t, err)
	assert.Equal(t, "output from node\n", string(output))
}
