// Copyright 2016-2024, Pulumi Corporation.
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
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// pnpm is an alternative package manager for Node.js.
type pnpmManager struct {
	executable string
}

// Assert that pnpm is an instance of PackageManager.
var _ PackageManager = &pnpmManager{}

func newPnpm() (*pnpmManager, error) {
	pnpmPath, err := exec.LookPath("pnpm")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("Could not find `pnpm` executable.\n" +
				"Install pnpm and make sure it is in your PATH.")
		}
		return nil, err
	}
	return &pnpmManager{
		executable: pnpmPath,
	}, nil
}

func (pnpm *pnpmManager) Name() string {
	return "pnpm"
}

func (pnpm *pnpmManager) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := pnpm.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func (pnpm *pnpmManager) installCmd(ctx context.Context, production bool) *exec.Cmd {
	args := []string{"install", "--use-stderr"}

	if production {
		args = append(args, "--production")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, pnpm.executable, args...)
}

func (pnpm *pnpmManager) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	command := exec.CommandContext(ctx, pnpm.executable, "pack", "--use-stderr")
	command.Dir = dir

	// We have to read the name of the file from stdout.
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = stderr
	err := command.Run()
	if err != nil {
		return nil, err
	}
	// Next, we try to read the name of the file from stdout. packfile is the name
	// of the file containing the tarball, as produced by `pnpm pack`. For pnpm
	// versions <9.13, pnpm pack outputs a single line with the name of the
	// tarball. On pnpm versions >=9.13, pnpm pack lists all the packed files, and
	// the last line is the tarball.
	packFilename := ""
	scanner := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	for scanner.Scan() {
		packFilename = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the output of 'pnpm pack': %w", err)
	}

	packfile := filepath.Join(dir, packFilename)
	defer os.Remove(packfile)

	packTarball, err := os.ReadFile(packfile)
	if err != nil {
		newErr := fmt.Errorf("'pnpm pack' completed successfully but the package .tgz file was not generated: %w", err)
		return nil, newErr
	}

	return packTarball, nil
}

// checkPnpmLock checks if there's a file named pnpm-lock.yaml in pwd.
// This function is used to indicate whether to prefer pnpm over
// other package managers.
func checkPnpmLock(pwd string) bool {
	pnpmFile := filepath.Join(pwd, "pnpm-lock.yaml")
	_, err := os.Stat(pnpmFile)
	return err == nil
}
