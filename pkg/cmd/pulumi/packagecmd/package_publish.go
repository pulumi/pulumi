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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/spf13/cobra"
)

func newPackagePublishCmd() *cobra.Command {
	var publCmd publishCmd
	cmd := &cobra.Command{
		Use:    "publish-sdk <language>",
		Args:   cobra.RangeArgs(0, 1),
		Short:  "Publish a package SDK to supported package registries.",
		Hidden: !env.Dev.Value(),
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return publCmd.Run(ctx, args)
		}),
	}
	cmd.PersistentFlags().StringVar(&publCmd.Path, "path", "",
		`The path to the root of your package.
	Example: ./sdk/nodejs
	`)
	return cmd
}

type publishCmd struct {
	Path string
}

func (cmd *publishCmd) Run(ctx context.Context, args []string) error {
	lang := "all"
	if len(args) > 0 {
		lang = args[0]
	}

	switch lang {
	case "nodejs":
		err := publishToNPM(cmd.Path)
		if err != nil {
			return err
		}
	case "all", "python", "java", "dotnet":
		return fmt.Errorf("support for %q coming soon", lang)

	default:
		return fmt.Errorf("unsupported language %q", lang)
	}

	return nil
}

func publishToNPM(path string) error {
	// verify path
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
		npmTag = "latest"
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
