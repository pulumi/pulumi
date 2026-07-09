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

// Package oci launches policy packs published as OCI container images. It
// shells out to a container runtime CLI (docker, podman, or nerdctl) rather
// than linking a registry or daemon client.
package oci

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// EnvContainerRuntime overrides container runtime detection with a specific
// CLI name (or path) to use.
const EnvContainerRuntime = "PULUMI_CONTAINER_RUNTIME"

var probeOrder = []string{"docker", "podman", "nerdctl"}

// ErrNoContainerRuntime is returned when no supported container runtime CLI is
// found on PATH.
var ErrNoContainerRuntime = errors.New(
	"no container runtime found: running a policy pack with runtime \"oci\" requires docker, podman, " +
		"or nerdctl on PATH (or set " + EnvContainerRuntime + " to the runtime to use)")

// Runtime is a resolved container runtime CLI.
type Runtime struct {
	Path string // absolute path to the CLI binary
	Name string // the CLI name, e.g. "docker"
}

// DetectRuntime resolves the container runtime CLI to use. lookPath may be nil
// to use exec.LookPath; tests inject a fake.
func DetectRuntime(lookPath func(string) (string, error)) (*Runtime, error) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if override := os.Getenv(EnvContainerRuntime); override != "" {
		path, err := lookPath(override)
		if err != nil {
			return nil, fmt.Errorf("container runtime %q (from %s) not found on PATH: %w",
				override, EnvContainerRuntime, err)
		}
		return &Runtime{Path: path, Name: override}, nil
	}
	for _, name := range probeOrder {
		if path, err := lookPath(name); err == nil {
			return &Runtime{Path: path, Name: name}, nil
		}
	}
	return nil, ErrNoContainerRuntime
}
