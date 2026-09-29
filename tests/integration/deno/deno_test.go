// Copyright 2026, Pulumi Corporation.
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

package deno

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/testing/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/tests/testutil"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestDenoCallbacks(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "callbacks",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			testutil.RequirePrinted(t, stack, "info", "hook called")
			require.True(t, stack.Outputs["isDeno"].(bool))
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestDeno(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:          "simple",
		Dependencies: []string{"@pulumi/pulumi"},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			require.True(t, stack.Outputs["isDeno"].(bool))
			require.Equal(t, "hello", stack.Outputs["name"])
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestDenoNative(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir:             "native/ancestor",
		RelativeWorkDir: "project",
		PrePrepareProject: func(info *engine.Projinfo) error {
			return prepareNativeDenoProject(info)
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			require.True(t, stack.Outputs["isDeno"].(bool))
			require.Equal(t, "hello", stack.Outputs["name"])
		},
	})
}

//nolint:paralleltest // ProgramTest calls t.Parallel()
func TestDenoNativeCallbacks(t *testing.T) {
	integration.ProgramTest(t, &integration.ProgramTestOptions{
		Dir: "native/callbacks",
		PrePrepareProject: func(info *engine.Projinfo) error {
			return prepareNativeDenoProject(info)
		},
		LocalProviders: []integration.LocalDependency{
			{Package: "testprovider", Path: testutil.TestProviderDir(t)},
		},
		Quick: true,
		ExtraRuntimeValidation: func(t *testing.T, stack integration.RuntimeValidationStackInfo) {
			testutil.RequirePrinted(t, stack, "info", "hook called")
			require.True(t, stack.Outputs["isDeno"].(bool))
		},
	})
}

func prepareNativeDenoProject(info *engine.Projinfo) error {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return errors.New("could not locate the Deno integration test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	sdkPath := filepath.Join(repoRoot, "sdk", "nodejs", "bin")
	packagePath := filepath.Join(info.Root, "node_modules", "@pulumi", "pulumi")
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o755); err != nil {
		return err
	}
	if err := fsutil.CopyFile(packagePath, sdkPath, nil); err != nil {
		return fmt.Errorf("copying local Pulumi SDK: %w", err)
	}
	if err := os.Symlink(filepath.Join(repoRoot, "sdk", "nodejs", "node_modules"),
		filepath.Join(packagePath, "node_modules")); err != nil {
		return fmt.Errorf("linking local Pulumi SDK dependencies: %w", err)
	}

	deno, err := exec.LookPath("deno")
	if err != nil {
		return err
	}
	smokePath := filepath.Join(info.Root, "pulumi-import-smoke.ts")
	smoke := `import * as pulumi from "@pulumi/pulumi";
import { fileURLToPath } from "node:url";
if (typeof pulumi.CustomResource !== "function") throw new Error("Pulumi SDK import failed");
const expected = await Deno.realPath("node_modules/@pulumi/pulumi/index.js");
const actual = await Deno.realPath(fileURLToPath(import.meta.resolve("@pulumi/pulumi")));
if (actual !== expected) throw new Error("Pulumi SDK resolved from " + actual + ", expected " + expected);
`
	if err := os.WriteFile(smokePath, []byte(smoke), 0o600); err != nil {
		return err
	}
	defer os.Remove(smokePath)
	cmd := exec.Command(deno, "run", "--cached-only", "--allow-all", smokePath)
	cmd.Dir = info.Root
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checking the local Deno npm import mapping: %w: %s", err, output)
	}
	return nil
}
