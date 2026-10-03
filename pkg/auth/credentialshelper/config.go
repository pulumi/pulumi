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

package credentialshelper

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	helperEnvVar = "PULUMI_CREDENTIAL_HELPER"
	argsEnvVar   = "PULUMI_CREDENTIAL_HELPER_ARGS"
)

// Source identifies how a credentials helper was selected.
type Source string

const (
	SourceEnvironment Source = "environment"
	SourceSaved       Source = "saved"
	SourceExecutable  Source = "beside-cli"
	SourcePath        Source = "path"
)

// Resolved contains a helper's absolute executable path, literal arguments, and configuration source.
type Resolved struct {
	workspace.CredentialHelper
	Source Source
}

// ResolveOptions provides the environment, saved configuration, and locations used to find a helper.
type ResolveOptions struct {
	// Environment defaults to the process environment.
	Environment env.Env
	Saved       *workspace.CredentialHelper
	// Executable defaults to the current process executable.
	Executable string
	// WorkingDirectory defaults to the current working directory.
	WorkingDirectory string
}

// Resolve selects and locates a helper without executing it. Nil means disabled or not found.
func Resolve(options ResolveOptions) (*Resolved, error) {
	environment := options.Environment
	if environment == nil {
		environment = env.Global()
	}
	lookup := func(name string) string {
		value, _ := environment.GetStore().Raw(name)
		return value
	}
	configured := lookup(helperEnvVar)
	if configured == "none" {
		return nil, nil
	}
	workingDirectory := options.WorkingDirectory
	if workingDirectory == "" {
		var err error
		workingDirectory, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("locating credentials helper: %w", err)
		}
	}
	if configured != "" {
		args, err := parseArguments(lookup(argsEnvVar))
		if err != nil {
			return nil, err
		}
		var path string
		if !filepath.IsAbs(configured) && !strings.ContainsAny(configured, `/\`) {
			path, err = findOnPath(configured, lookup("PATH"), workingDirectory)
			if err == nil && path == "" {
				err = fmt.Errorf("credentials helper %q was not found on PATH", configured)
			}
		} else {
			path, err = resolveExecutable(configured, workingDirectory)
		}
		if err != nil {
			return nil, err
		}
		return &Resolved{CredentialHelper: workspace.CredentialHelper{Path: path, Args: args}, Source: SourceEnvironment}, nil
	}
	if options.Saved != nil {
		if !filepath.IsAbs(options.Saved.Path) {
			return nil, errors.New("saved credentials helper path must be absolute")
		}
		if err := validateArguments(options.Saved.Args); err != nil {
			return nil, err
		}
		path, err := resolveExecutable(options.Saved.Path, workingDirectory)
		if err != nil {
			return nil, err
		}
		return &Resolved{
			CredentialHelper: workspace.CredentialHelper{Path: path, Args: slices.Clone(options.Saved.Args)},
			Source:           SourceSaved,
		}, nil
	}

	cliPath := options.Executable
	if cliPath == "" {
		var err error
		cliPath, err = os.Executable()
		if err != nil {
			return nil, fmt.Errorf("locating Pulumi executable for credentials helper discovery: %w", err)
		}
	}
	if !filepath.IsAbs(cliPath) {
		cliPath = filepath.Join(workingDirectory, cliPath)
	}
	cliPath, err := filepath.EvalSymlinks(cliPath)
	if err != nil {
		return nil, fmt.Errorf("resolving Pulumi executable for credentials helper discovery: %w", err)
	}
	name := "pulumi-credentials-helper"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	candidate := filepath.Join(filepath.Dir(cliPath), name)
	if _, err := os.Lstat(candidate); err == nil {
		path, err := resolveExecutable(candidate, workingDirectory)
		if err != nil {
			return nil, err
		}
		return &Resolved{CredentialHelper: workspace.CredentialHelper{Path: path}, Source: SourceExecutable}, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("looking for credentials helper beside Pulumi: %w", err)
	}
	path, err := findOnPath(name, lookup("PATH"), workingDirectory)
	if err != nil || path == "" {
		return nil, err
	}
	return &Resolved{CredentialHelper: workspace.CredentialHelper{Path: path}, Source: SourcePath}, nil
}

func parseArguments(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of strings", argsEnvVar)
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a JSON array of strings", argsEnvVar)
	}
	args := make([]string, len(values))
	for i, value := range values {
		arg, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be a JSON array of strings", argsEnvVar)
		}
		args[i] = arg
	}
	if err := validateArguments(args); err != nil {
		return nil, err
	}
	return args, nil
}

func validateArguments(args []string) error {
	for _, arg := range args {
		if strings.ContainsRune(arg, '\x00') {
			return errors.New("credentials helper arguments must not contain NUL characters")
		}
	}
	return nil
}

func findOnPath(name, path, workingDirectory string) (string, error) {
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		name += ".exe"
	}
	for _, dir := range filepath.SplitList(path) {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(workingDirectory, dir)
		}
		candidate := filepath.Join(dir, name)
		if _, err := os.Lstat(candidate); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("looking for credentials helper on PATH: %w", err)
		}
		return resolveExecutable(candidate, workingDirectory)
	}
	return "", nil
}

func resolveExecutable(path, workingDirectory string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving credentials helper path: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolving credentials helper path: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("checking credentials helper: %w", err)
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0) {
		return "", fmt.Errorf("credentials helper %q is not an executable file", path)
	}
	return path, nil
}
