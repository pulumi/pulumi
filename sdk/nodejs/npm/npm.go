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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/errutil"
)

// NPM is the canonical "Node Package Manager".
type npmManager struct {
	executable string
}

// Assert that NPM is an instance of PackageManager.
var _ PackageManager = &npmManager{}

func newNPM() (*npmManager, error) {
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errors.New("Could not find `npm` executable.\n" +
				"Install npm and make sure it is in your PATH.")
		}
		return nil, err
	}
	return &npmManager{
		executable: npmPath,
	}, nil
}

func (node *npmManager) Name() string {
	return "npm"
}

func (node *npmManager) Version() (semver.Version, error) {
	cmd := exec.Command(node.executable, "--version") //nolint:gosec
	output, err := cmd.Output()
	if err != nil {
		return semver.Version{}, errutil.ErrorWithStderr(err, cmd.String())
	}
	versionStr := strings.TrimSpace(string(output))
	version, err := semver.Parse(versionStr)
	if err != nil {
		return semver.Version{}, err
	}
	return version, nil
}

func (node *npmManager) Install(ctx context.Context, dir string, production bool, stdout, stderr io.Writer) error {
	command := node.installCmd(ctx, production)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func (node *npmManager) installCmd(ctx context.Context, production bool) *exec.Cmd {
	// We pass `--loglevel=error` to prevent `npm` from printing warnings about missing
	// `description`, `repository`, and `license` fields in the package.json file.
	args := []string{"install", "--loglevel=error"}

	if production {
		args = append(args, "--production")
	}

	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	return exec.CommandContext(ctx, node.executable, args...)
}

func (node *npmManager) Link(ctx context.Context, dir, packageName, path string) error {
	packageSpecifier := getLinkPackageProperty(packageName, path)
	cmd := exec.CommandContext(ctx, "npm", "pkg", "set", packageSpecifier)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error executing npm command %s: %w, output: %s", cmd.String(), err, out)
	}
	return nil
}

func (node *npmManager) ListPackages(ctx context.Context, dir string) ([]PackageDependency, error) {
	if node.executable == "" {
		// If npm is not available, fall back to reading package.json.
		return listPackagesFromPackageJSON(dir)
	}
	//nolint:gosec // False positive on tainted command execution. We aren't accepting input from the user here.
	cmd := exec.CommandContext(ctx, node.executable, "ls", "--json", "--depth=0")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run \"%s ls --json --depth=0\": %w", node.executable, err)
	}
	var file struct {
		Dependencies map[string]struct {
			Version  string `json:"version"`
			Resolved string `json:"resolved"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &file); err != nil {
		return nil, fmt.Errorf("failed to parse npm ls output: %w", err)
	}
	result := make([]PackageDependency, 0, len(file.Dependencies))
	for name, pkg := range file.Dependencies {
		result = append(result, PackageDependency{
			Name:    name,
			Version: pkg.Version,
		})
	}
	return result, nil
}

func (node *npmManager) Pack(ctx context.Context, dir string, stderr io.Writer) ([]byte, error) {
	//nolint:gosec // False positive on tained command execution. We aren't accepting input from the user here.
	command := exec.CommandContext(ctx, node.executable, "pack", "--loglevel=error")
	command.Dir = dir

	// We have to read the name of the file from stdout.
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = stderr
	err := command.Run()
	if err != nil {
		return nil, err
	}
	// Next, we try to read the name of the file from stdout.
	// packfile is the name of the file containing the tarball,
	// as produced by `npm pack`.
	packFilename := strings.TrimSpace(stdout.String())
	packfile := filepath.Join(dir, packFilename)
	defer os.Remove(packfile)

	packTarball, err := os.ReadFile(packfile)
	if err != nil {
		newErr := fmt.Errorf("'npm pack' completed successfully but the package .tgz file was not generated: %w", err)
		return nil, newErr
	}

	return packTarball, nil
}
