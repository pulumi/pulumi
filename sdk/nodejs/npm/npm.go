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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	uuid "github.com/gofrs/uuid"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
)

// Pack runs `npm pack` in the given directory, packaging the Node.js app located there into a
// tarball and returning it as `[]byte`. `stdout` is ignored for the command, as it does not
// generate useful data. If the `PULUMI_PREFER_YARN` environment variable is set, `yarn pack` is run
// instead of `npm pack`.
func Pack(dir string, stderr io.Writer) ([]byte, error) {
	c, npm, bin, err := getCmd("pack")
	if err != nil {
		return nil, err
	}
	c.Dir = dir

	// Note that `npm pack` doesn't have the ability to specify the resulting filename, since
	// it's meant to be uploaded directly to npm, which means we have to get that information
	// by parsing the output of the command. However, if we're using `yarn`, we can specify a
	// filename.
	var packfile string

	if !npm {
		// generate a new uuid
		uuid, err := uuid.NewV4()
		if err != nil {
			return nil, err
		}
		packfile = fmt.Sprintf("%s.tgz", uuid.String())
		c.Args = append(c.Args, "--filename", packfile)
	}

	// Run the command.
	// `stdout` is ignored for the command, as it does not generate useful data.
	var stdout bytes.Buffer
	if err = runCmd(c, npm, &stdout, stderr); err != nil {
		return nil, err
	}

	// If `npm` was used, parse the filename from the output.
	if npm {
		packfile = strings.TrimSpace(stdout.String())
	}

	defer os.Remove(packfile)

	packTarball, err := ioutil.ReadFile(packfile)
	if err != nil {
		return nil, fmt.Errorf("%s pack completed successfully but the packed .tgz file was not generated", bin)
	}

	return packTarball, nil
}

// Install runs `npm install` in the given directory, installing the dependencies for the Node.js
// app located there. If the `PULUMI_PREFER_YARN` environment variable is set, `yarn install` is used
// instead of `npm install`.
func Install(dir string, stdout, stderr io.Writer) (string, error) {
	c, npm, bin, err := getCmd("install")
	if err != nil {
		return bin, err
	}
	c.Dir = dir

	// Run the command.
	if err = runCmd(c, npm, stdout, stderr); err != nil {
		return bin, err
	}

	// Ensure the "node_modules" directory exists.
	nodeModulesPath := filepath.Join(dir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		return bin, errors.Errorf("%s install reported success, but node_modules directory is missing", bin)
	}

	return bin, nil
}

// getCmd returns the exec.Cmd used to install NPM dependencies. It will either use `npm` or `yarn` depending
// on what is available on the current path, and if `PULUMI_PREFER_YARN` is truthy.
// The boolean return parameter indicates if `npm` is chosen or not (instead of `yarn`).
func getCmd(command string) (*exec.Cmd, bool, string, error) {
	if preferYarn() {
		const file = "yarn"
		yarnPath, err := exec.LookPath(file)
		if err == nil {
			return exec.Command(yarnPath, command), false, file, nil
		}
		logging.Warningf("could not find yarn on the $PATH, trying npm instead: %v", err)
	}

	const file = "npm"
	npmPath, err := exec.LookPath(file)
	if err != nil {
		return nil, false, file, errors.Wrapf(err, "could not find npm on the $PATH; npm is installed with Node.js "+
			"available at https://nodejs.org/")
	}
	// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
	// `description`, `repository`, and `license` fields in the package.json file.
	return exec.Command(npmPath, command, "--loglevel=error"), true, file, nil
}

// runCmd handles hooking up `stdout` and `stderr` and then runs the command.
func runCmd(c *exec.Cmd, npm bool, stdout, stderr io.Writer) error {
	// Setup `stdout` and `stderr`.
	// `stderr` is ignored when `yarn` is used because it outputs warnings like "package.json: No license field"
	// to `stderr` that we don't need to show.
	c.Stdout = stdout
	var stderrBuffer bytes.Buffer
	if npm {
		c.Stderr = stderr
	} else {
		c.Stderr = &stderrBuffer
	}

	// Run the command.
	if err := c.Run(); err != nil {
		// If we failed, and we're using `yarn`, write out any bytes that were written to `stderr`.
		if !npm {
			stderr.Write(stderrBuffer.Bytes())
		}
		return err
	}

	return nil
}

// preferYarn returns true if the `PULUMI_PREFER_YARN` environment variable is set.
func preferYarn() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_PREFER_YARN"))
}
