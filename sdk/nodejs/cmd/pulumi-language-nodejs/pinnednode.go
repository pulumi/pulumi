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

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/sdk/nodejs/cmd/pulumi-language-nodejs/v3/noderesolver"
	"github.com/pulumi/pulumi/sdk/v3/nodejs/npm"
)

var nodeVersionRegexp = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// parseNodeVersionOption validates the nodeVersion runtime option against the
// runtime and the already-parsed packagemanager option, returning the exact
// version or "" when the option is absent.
func parseNodeVersionOption(
	options map[string]any, runtime string, packagemanager npm.PackageManagerType,
) (string, error) {
	nodeVersion, ok := options["nodeVersion"]
	if !ok {
		return "", nil
	}
	nv, ok := nodeVersion.(string)
	if !ok {
		return "", errors.New("nodeVersion option must be a string")
	}
	if !nodeVersionRegexp.MatchString(nv) {
		return "", fmt.Errorf(
			"nodeVersion must be an exact version of the form X.Y.Z (for example 22.12.0), got %q", nv)
	}
	if runtime == "bun" {
		return "", errors.New("the nodeVersion option is not supported with the bun runtime")
	}
	if packagemanager != "" &&
		packagemanager != npm.AutoPackageManager &&
		packagemanager != npm.NpmPackageManager {
		return "", fmt.Errorf(
			"the nodeVersion option requires the npm package manager (only npm ships with a Node.js "+
				"distribution), but packagemanager is set to %q", packagemanager)
	}
	return nv, nil
}

// pinnedNode resolves the pinned Node.js toolchain when opts.nodeVersion is
// set and applies to this request. It returns nil when no version is pinned,
// and warns and returns nil when a version is pinned somewhere pinning isn't
// supported yet.
func pinnedNode(ctx context.Context, opts nodeOptions, applies bool, out io.Writer) (*noderesolver.Result, error) {
	if opts.nodeVersion == "" {
		return nil, nil
	}
	if !applies {
		fmt.Fprintf(out,
			"warning: the nodeVersion runtime option is currently only supported for policy packs "+
				"and was ignored\n")
		return nil, nil
	}
	res, err := noderesolver.Resolve(ctx, noderesolver.Spec{Version: opts.nodeVersion, Output: out})
	if err != nil {
		return nil, fmt.Errorf("resolving pinned Node.js v%s: %w", opts.nodeVersion, err)
	}
	return &res, nil
}

// prependPathInEnv returns a copy of env with dir prepended to its PATH
// entry, adding one when none exists. Only the child process that receives
// the returned slice sees the modified PATH.
func prependPathInEnv(env []string, dir string) []string {
	out := make([]string, len(env), len(env)+1)
	copy(out, env)
	for i, kv := range out {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.EqualFold(k, "PATH") {
			out[i] = k + "=" + dir + string(os.PathListSeparator) + v
			return out
		}
	}
	return append(out, "PATH="+dir)
}

// pinnedNpmInstall runs the pinned toolchain's npm directly instead of going
// through the npm package's PATH-based resolution. The managed bin dir is
// prepended to the child PATH because npm's launcher re-resolves node by
// name.
func pinnedNpmInstall(
	ctx context.Context, node noderesolver.Result, dir string, production bool, stdout, stderr io.Writer,
) error {
	args := []string{"install", "--loglevel=error"}
	if production {
		args = append(args, "--production")
	}
	cmd := exec.CommandContext(ctx, node.Npm, args...)
	cmd.Dir = dir
	cmd.Env = prependPathInEnv(os.Environ(), node.BinDir)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	return cmd.Run()
}
