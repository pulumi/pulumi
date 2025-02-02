// Copyright 2019-2024, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// NPM is the canonical "Node Package Manager".
type bunManager struct {
	executable string
}

// Assert that Bun is an instance of PackageManager.
var _ PackageManager = &bunManager{}

func newBun() (*bunManager, error) {
	bunPath, err := exec.LookPath("bun")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("Could not find `bun` executable.\n" +
				"Install bun and make sure it is in your PATH.")
		}
		return nil, err
	}
	return &bunManager{
		executable: bunPath,
	}, nil
}

func (bun *bunManager) Name() string {
	return "bun"
}

func (bun *bunManager) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := bun.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func (bun *bunManager) installCmd(ctx context.Context, production bool) *exec.Cmd {
	args := []string{"install"}

	if production {
		args = append(args, "--production")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, bun.executable, args...)
}

type packageDotJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (bun *bunManager) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	command := exec.CommandContext(ctx, bun.executable, "pm", "pack")
	command.Dir = dir

	err := command.Run()
	if err != nil {
		return nil, err
	}

	// The other package managers have the ability to read the output from the pack command
	// but bun can't do that, so we read in the package.json and get the pack filename from that
	packageJSONFilePath := filepath.Join(dir, "package.json")
	defer os.Remove(packageJSONFilePath)
	packageJSONFile, err := os.ReadFile(packageJSONFilePath)
	if err != nil {
		newErr := fmt.Errorf("'bun pm pack' was successful but "+
			"could not read package.json to find pack filename: %w", err)
		return nil, newErr
	}

	var packageJSON packageDotJSON

	err = json.Unmarshal(packageJSONFile, &packageJSON)
	if err != nil {
		newErr := fmt.Errorf("'bun pm pack' was successful but "+
			"could not get package name and version: %w", err)
		return nil, newErr
	}

	packFilename := fmt.Sprintf("%s-%s.tgz", packageJSON.Name, packageJSON.Version)
	packfile := filepath.Join(dir, packFilename)
	defer os.Remove(packfile)

	packTarball, err := os.ReadFile(packfile)
	if err != nil {
		newErr := fmt.Errorf("'bun pm pack' completed successfully but the package .tgz file was not generated: %w", err)
		return nil, newErr
	}

	return packTarball, nil
}

// checkBunLock checks if there's a file named bun.lock or bun.lockb in pwd.
// This function is used to indicate whether to prefer bun over
// other package managers.
func checkBunLock(pwd string) bool {
	bunLockFile := filepath.Join(pwd, "bun.lock")
	bunLockBinaryFile := filepath.Join(pwd, "bun.lockb")
	_, err := os.Stat(bunLockFile) // check this first as since 1.2 this is the default lockfile
	if err == nil {
		return true
	}

	_, err = os.Stat(bunLockBinaryFile)

	return err == nil
}
