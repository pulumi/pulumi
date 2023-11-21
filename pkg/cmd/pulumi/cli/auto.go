// Copyright 2016-2023, Pulumi Corporation.
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
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/cli"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type pulumiError struct {
	stdout string
	stderr string
	err    error
}

func newPulumiError(err error, stdout, stderr string) *pulumiError {
	return &pulumiError{
		stdout,
		stderr,
		err,
	}
}

func (e *pulumiError) Stdout() string {
	return e.stdout
}

func (e *pulumiError) Stderr() string {
	return e.stderr
}

func (e *pulumiError) Error() string {
	return fmt.Sprintf("%s\nstdout: %s\nstderr: %s\n", e.err.Error(), e.stdout, e.stderr)
}

type pulumiCommands struct {
	version *semver.Version

	workDir string
	home    string
	envVars map[string]string

	opts *cli.DebugOptions
}

func Auto(workDir, home string, envVars map[string]string) cli.PulumiCommands {
	return &pulumiCommands{
		workDir: workDir,
		home:    home,
		envVars: envVars,
	}
}

func (cli *pulumiCommands) Debug(opts *cli.DebugOptions) cli.PulumiCommands {
	return &pulumiCommands{
		version: cli.version,
		workDir: cli.workDir,
		home:    cli.home,
		envVars: cli.envVars,
		opts:    opts,
	}
}

func (cli *pulumiCommands) Stack(name string) cli.StackCommands {
	return &pulumiStackCommands{
		cli:   cli,
		stack: name,
	}
}

func (cli *pulumiCommands) Plugin() cli.PluginCommands {
	return &pulumiPluginCommands{cli: cli}
}

func (cli *pulumiCommands) WhoAmI(ctx context.Context) (*cli.WhoAmIResult, error) {
	panic("NYI")
}

func (cli *pulumiCommands) Version(ctx context.Context) (*semver.Version, error) {
	panic("NYI")
}

type pulumiPluginCommands struct {
	cli *pulumiCommands
}

func (cli *pulumiPluginCommands) Install(ctx context.Context, name, version string, opts *cli.PluginInstallOptions) error {
	panic("NYI")
}

func (cli *pulumiPluginCommands) Rm(ctx context.Context, name, version string) {
	panic("NYI")
}

func (cli *pulumiPluginCommands) Ls(ctx context.Context) ([]workspace.PluginInfo, error) {
	panic("NYI")
}

type pulumiStackCommands struct {
	cli *pulumiCommands

	stack string
}

func (cli *pulumiStackCommands) Config() cli.ConfigCommands {
	return &pulumiConfigCommands{stack: cli}
}

func (cli *pulumiStackCommands) Tag() cli.TagCommands {
	return &pulumiTagCommands{stack: cli}
}

func (cli *pulumiStackCommands) Preview(ctx context.Context, opts *cli.StackPreviewOptions) (*cli.StackPreviewResult, error) {
	panic("NYI")
}

func (cli *pulumiStackCommands) Update(ctx context.Context, opts *cli.StackUpdateOptions) (*cli.StackUpdateResult, error) {
	panic("NYI")
}

func (cli *pulumiStackCommands) Refresh(ctx context.Context, opts *cli.StackRefreshOptions) (*cli.StackRefreshResult, error) {
	panic("NYI")
}

func (cli *pulumiStackCommands) Destroy(ctx context.Context, opts *cli.StackDestroyOptions) (*cli.StackDestroyResult, error) {
	panic("NYI")
}

func (cli *pulumiStackCommands) Cancel(ctx context.Context) error {
	panic("NYI")
}

func (cli *pulumiStackCommands) Init(ctx context.Context, opts *cli.StackInitOptions) error {
	panic("NYI")
}

func (cli *pulumiStackCommands) Select(ctx context.Context) error {
	panic("NYI")
}

func (cli *pulumiStackCommands) Rm(ctx context.Context, opts *cli.StackRmOptions) error {
	panic("NYI")
}

func (cli *pulumiStackCommands) Ls(ctx context.Context) ([]cli.StackSummary, error) {
	panic("NYI")
}

func (cli *pulumiStackCommands) Export(ctx context.Context) (apitype.UntypedDeployment, error) {
	panic("NYI")
}

func (cli *pulumiStackCommands) Import(ctx context.Context, state apitype.UntypedDeployment) error {
	panic("NYI")
}

func (cli *pulumiStackCommands) Outputs(ctx context.Context) (map[string]cli.OutputValue, error) {
	panic("NYI")
}

func (cli *pulumiStackCommands) History(ctx context.Context, opts *cli.StackHistoryOptions) ([]cli.UpdateSummary, error) {
	panic("NYI")
}

type pulumiConfigCommands struct {
	stack *pulumiStackCommands
}

func (cli *pulumiConfigCommands) Get(ctx context.Context, key string, opts *cli.ConfigGetOptions) (cli.ConfigValue, error) {
	panic("NYI")
}

func (cli *pulumiConfigCommands) Set(ctx context.Context, key string, value cli.ConfigValue, opts *cli.ConfigSetOptions) error {
	panic("NYI")
}

func (cli *pulumiConfigCommands) GetAll(ctx context.Context) (map[string]cli.ConfigValue, error) {
	panic("NYI")
}

func (cli *pulumiConfigCommands) SetAll(ctx context.Context, values map[string]cli.ConfigValue) error {
	panic("NYI")
}

func (cli *pulumiConfigCommands) Rm(ctx context.Context, key string, opts *cli.ConfigRmOptions) error {
	panic("NYI")
}

func (cli *pulumiConfigCommands) Refresh(ctx context.Context) error {
	panic("NYI")
}

type pulumiTagCommands struct {
	stack *pulumiStackCommands
}

func (cli *pulumiTagCommands) Get(ctx context.Context, key string) (string, error) {
	panic("NYI")
}

func (cli *pulumiTagCommands) Set(ctx context.Context, key, value string) error {
	panic("NYI")
}

func (cli *pulumiTagCommands) Rm(ctx context.Context, key string) error {
	panic("NYI")
}

func (cli *pulumiTagCommands) Ls(ctx context.Context) (map[string]string, error) {
	panic("NYI")
}
