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

package packagecmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/spf13/cobra"
)

func newPackagePublishSdkCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:    "publish-sdk <language>",
		Args:   cobra.RangeArgs(0, 1),
		Short:  "Publish a package SDK to supported package registries.",
		Hidden: !env.Dev.Value(),
		RunE: func(cmd *cobra.Command, args []string) error {
			lang := "all"
			if len(args) > 0 {
				lang = args[0]
			}

			switch lang {
			case "nodejs":
				err := publishToNPM(path)
				if err != nil {
					return err
				}
			case "all", "python", "java", "dotnet":
				return fmt.Errorf("support for %q coming soon", lang)

			default:
				return fmt.Errorf("unsupported language %q", lang)
			}

			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&path, "path", "",
		`The path to the root of your package.
	Example: ./sdk/nodejs
	`)
	return cmd
}

func determineNPMTagFromCommandResult(currentVersion, npmOutput, npmStderr string, npmError error) (string, error) {
	currentVer, err := semver.ParseTolerant(currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse current version %q: %w", currentVersion, err)
	}

	if npmError != nil {
		// If this is a new package, it won't exist on the registry yet and return a 404.
		// We want to be able to push that and label it as latest.
		if npmOutput == "" && strings.Contains(npmStderr, "404") {
			return "latest", nil
		}
		return "", fmt.Errorf("failed to get latest version from npm: %w", npmError)
	}

	latestVer, err := semver.ParseTolerant(npmOutput)
	if err != nil {
		return "", fmt.Errorf("failed to parse latest version %q from npm: %w", npmOutput, err)
	}

	if latestVer.GT(currentVer) {
		return "backport", nil
	}
	return "latest", nil
}

// determineNPMTagForStableVersion determines if we should tag this version as a backport, rather than as latest.
func determineNPMTagForStableVersion(npm, pkgName, currentVersion string) (string, error) {
	infoCmd := exec.Command(npm, "info", pkgName, "version")

	var stderr bytes.Buffer
	infoCmd.Stderr = &stderr

	output, err := infoCmd.Output()
	return determineNPMTagFromCommandResult(currentVersion, string(output), stderr.String(), err)
}

func publishToNPM(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("reading path %s: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}

	// Verify npm exists and is set up: npm, user login
	npm, err := executable.FindExecutable("npm")
	if err != nil {
		return fmt.Errorf("npm whoami: %w", err)
	}

	// verify auth for npm
	whoamiCmd := exec.Command(npm, "whoami")
	whoamiCmd.Stderr = os.Stderr
	whoami, err := whoamiCmd.Output()
	if err != nil {
		return err
	}

	logging.V(1).Infof("Logged in as %s", whoami)

	// TODO: possibly check package dependencies

	var pkgInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	file, err := os.ReadFile(filepath.Join(path, "package.json"))
	if err != nil {
		return err
	}
	err = json.Unmarshal(file, &pkgInfo)
	if err != nil {
		return fmt.Errorf("unmarshal package.json: %w", err)
	}

	// Determine which tag to set
	// npm adds `latest` as the default tag, and we want that to mean the newest released version.
	var npmTag string

	switch {
	case strings.Contains(pkgInfo.Version, "-alpha"):
		npmTag = "dev"
	case strings.Contains(pkgInfo.Version, "dev"):
		npmTag = "dev"
	case strings.Contains(pkgInfo.Version, "-beta"):
		npmTag = "beta"
	case strings.Contains(pkgInfo.Version, "-rc"):
		npmTag = "rc"
	default:
		// For stable versions, determine tag by comparing with npm
		var err error
		npmTag, err = determineNPMTagForStableVersion(npm, pkgInfo.Name, pkgInfo.Version)
		if err != nil {
			return fmt.Errorf("determining npm tag: %w", err)
		}
	}

	pkgNameWithVersion := pkgInfo.Name + "@" + pkgInfo.Version

	// Verify version doesn't already exist
	infoCmd := exec.Command(npm, "info", pkgNameWithVersion)
	infoCmd.Stderr = os.Stderr
	logging.V(1).Infof("Running %s", infoCmd)
	// we actually do not care about the error here; we care whether the output is empty.
	output, _ := infoCmd.Output()

	if len(output) > 0 {
		// the package already exists, and we no-op.
		fmt.Printf("did not publish %s because version %s already exists\n", pkgInfo.Name, pkgNameWithVersion)
		return nil
	}

	logging.V(1).Infof("The version does not exist yet, and it is safe to publish")
	fmt.Printf("Publishing %s to npm package registry...\n", pkgInfo.Name)
	npmPublishCmd := exec.Command(npm, "publish", path, "-tag", npmTag)
	npmPublishCmd.Stdout = os.Stdout
	npmPublishCmd.Stderr = os.Stderr
	err = npmPublishCmd.Run()
	if err != nil {
		logging.V(1).Infof("error publishing package, verifying...")
		// first, check if the package was published after all, by re-running npm info
		// to verify we're not encountering a time-of-check to time-of-use (TOC/TOU) issue.
		infoCheckCmd := exec.Command("npm", "info", pkgNameWithVersion)
		infoCheckCmd.Stderr = os.Stderr
		// Ignore error. stdout will be empty if the package was not published.
		checkOutput, _ := infoCheckCmd.Output()

		if len(checkOutput) > 0 {
			// this means the package was published after all
			fmt.Println("success! published to npm")
			return nil
		}
		// if we get here, this means the package was not published. We bail.
		return fmt.Errorf("publish package: %w", err)
	}
	fmt.Println("success! published to npm")
	return nil
}
