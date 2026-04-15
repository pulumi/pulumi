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

// Package base is the shared foundation consumed by generated command methods
// and their per-command option packages.
//
// The integration PR (#4) will swap PulumiCommand here for the real interface
// exported from sdk/go/auto. For the standalone code generator it is a
// structural copy so the generated output compiles in isolation.
package base

import (
	"context"
	"io"
)

// BaseOptions is embedded in every generated per-command Options struct. It
// carries ambient invocation configuration that is not specific to any
// individual CLI command.
type BaseOptions struct {
	// Cwd is the working directory in which to run the pulumi CLI. When
	// empty the caller's process-level cwd is used.
	Cwd string

	// AdditionalEnv is merged over the process environment before running
	// the command.
	AdditionalEnv map[string]string

	// Stdout, when non-nil, receives a copy of the child process' stdout in
	// real time.
	Stdout io.Writer

	// Stderr, when non-nil, receives a copy of the child process' stderr in
	// real time.
	Stderr io.Writer

	// Stdin, when non-nil, is connected to the child process' stdin.
	Stdin io.Reader
}

// CommandResult is the terminal value returned by every generated method.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// PulumiCommand is the structural contract the API calls into. It mirrors
// `github.com/pulumi/pulumi/sdk/v3/go/auto.PulumiCommand` so that the
// integration PR can drop the real type in here without touching the
// generator.
type PulumiCommand interface {
	Run(
		ctx context.Context,
		workdir string,
		stdin io.Reader,
		additionalOutput []io.Writer,
		additionalErrorOutput []io.Writer,
		additionalEnv []string,
		args ...string,
	) (string, string, int, error)
}
