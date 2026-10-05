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

package credentialhelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Source identifies how a credential helper was selected.
type Source string

const (
	SourceExplicit    Source = "explicit"
	SourceEnvironment Source = "environment"
	SourceSaved       Source = "saved"
	SourceExecutable  Source = "beside-cli"
	SourcePath        Source = "path"
)

const discoveredName = "pulumi-credential-helper"

// Resolved contains a helper's absolute executable path, literal arguments, and configuration source.
type Resolved struct {
	workspace.CredentialHelper
	Source Source
}

// ResolveHelperOptions provides the configuration sources that are not read from the environment.
type ResolveHelperOptions struct {
	// Explicit takes precedence over environment configuration, including disablement.
	Explicit *workspace.CredentialHelper
	Saved    *workspace.CredentialHelper
	// Executable defaults to the current process executable.
	Executable string
}

// ResolveHelper selects and locates a helper without executing it. Nil means disabled or not found.
//
// Helpers are selected in the following order:
//  1. Explicit configuration
//  2. PULUMI_CREDENTIAL_HELPER
//  3. Saved configuration
//  4. A helper beside the symlink-resolved Pulumi executable
//  5. A helper on PATH
//
// PULUMI_CREDENTIAL_HELPER=none disables helpers unless explicit configuration is supplied.
func ResolveHelper(options ResolveHelperOptions) (*Resolved, error) {
	configured := env.CredentialHelper.Value()
	switch {
	case options.Explicit != nil:
		return resolveHelperPath(*options.Explicit, SourceExplicit)
	case configured == "none":
		return nil, nil
	case configured != "":
		args, err := parseArguments(env.CredentialHelperArgs.Value())
		if err != nil {
			return nil, err
		}
		return resolveHelperPath(workspace.CredentialHelper{Path: configured, Args: args}, SourceEnvironment)
	case options.Saved != nil:
		if !filepath.IsAbs(options.Saved.Path) {
			return nil, errors.New("saved credential helper path must be absolute")
		}
		return resolveHelperPath(*options.Saved, SourceSaved)
	}

	cliPath := options.Executable
	if cliPath == "" {
		var err error
		cliPath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("locating Pulumi executable for credential helper discovery: %w", err)
		}
	}
	cliPath, err := filepath.EvalSymlinks(cliPath)
	if err != nil {
		return nil, fmt.Errorf("resolving Pulumi executable for credential helper discovery: %w", err)
	}
	name := discoveredName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidate := filepath.Join(filepath.Dir(cliPath), name)
	if _, err := os.Lstat(candidate); err == nil {
		return resolveHelperPath(workspace.CredentialHelper{Path: candidate}, SourceExecutable)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("looking for credential helper beside Pulumi: %w", err)
	}
	found, err := exec.LookPath(name)
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("looking for credential helper on PATH: %w", err)
	}
	return resolveHelperPath(workspace.CredentialHelper{Path: found}, SourcePath)
}

// resolveHelperPath resolves and validates a helper executable named by a path or a command on PATH.
func resolveHelperPath(helper workspace.CredentialHelper, source Source) (*Resolved, error) {
	if helper.Path == "" {
		return nil, errors.New("credential helper path must not be empty")
	}
	for _, arg := range helper.Args {
		if strings.ContainsRune(arg, '\x00') {
			return nil, errors.New("credential helper arguments must not contain NUL characters")
		}
	}
	path := helper.Path
	if !strings.ContainsAny(path, `/\`) {
		found, err := exec.LookPath(path)
		if err != nil {
			return nil, fmt.Errorf("locating credential helper: %w", err)
		}
		path = found
	}
	path, err := filepath.Abs(path)
	if err == nil {
		path, err = filepath.EvalSymlinks(path)
	}
	if err != nil {
		return nil, fmt.Errorf("resolving credential helper path: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("checking credential helper: %w", err)
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0) {
		return nil, fmt.Errorf("credential helper %q is not an executable file", path)
	}
	return &Resolved{
		CredentialHelper: workspace.CredentialHelper{Path: path, Args: slices.Clone(helper.Args)},
		Source:           source,
	}, nil
}

func parseArguments(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var args []string
	decoder := json.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&args); err != nil || args == nil {
		return nil, fmt.Errorf("%s must be a JSON array of strings", env.CredentialHelperArgs.Var().Name())
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("%s must be a JSON array of strings", env.CredentialHelperArgs.Var().Name())
	}
	return args, nil
}
