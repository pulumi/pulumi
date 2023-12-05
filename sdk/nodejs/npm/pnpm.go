// Copyright 2016-2023, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package npm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// pnpm is an implementation of PackageManager that uses PNPM,
// to install dependencies.
type pnpm struct {
	executable string
}

// Assert that it is an instance of PackageManager.
var _ PackageManager = &pnpm{}

func newPNPM() (*pnpm, error) {
	path, err := exec.LookPath("pnpm")
	manager := &pnpm{
		executable: path,
	}
	return manager, err
}

func (manager *pnpm) Name() string {
	return "pnpm"
}

func (manager *pnpm) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := manager.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

// Generates the command to install packages with PNPM.
func (manager *pnpm) installCmd(ctx context.Context, production bool) *exec.Cmd {
	args := []string{"install"}

	if production {
		args = append(args, "--prod")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, manager.executable, args...)
}

// Pack runs `pnpm pack` in the given directory, packaging the Node.js app located
// there into a tarball an returning it as `[]byte`. `stdout` is ignored for this command,
// as it does not produce useful data.
func (manager *pnpm) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	command := exec.CommandContext(ctx, manager.executable, "pack")
	command.Dir = dir

	// Like NPM, stdout prints the name of the generated file.
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = stderr
	err := command.Run()
	if err != nil {
		return nil, err
	}
	// Next, we try to read the name of the file from stdout.
	// packfile is the name of the file containing the tarball,
	// as produced by `pnpm pack`.
	packfile := strings.TrimSpace(stdout.String())
	defer os.Remove(packfile)

	packTarball, err := os.ReadFile(packfile)
	if err != nil {
		newErr := fmt.Errorf("'pnpm pack' completed successfully but the packaged .tgz file was not generated: %v", err)
		return nil, newErr
	}

	return packTarball, nil
}

// checkPNPMLock checks if there's a file named pnpm-lock.yaml in pwd.
// This function is used to indicate whether to prefer PNPM over
// other package managers.
func checkPNPMLock(pwd string) bool {
	yarnFile := filepath.Join(pwd, "pnpm-lock.yaml")
	_, err := os.Stat(yarnFile)
	return err == nil
}
