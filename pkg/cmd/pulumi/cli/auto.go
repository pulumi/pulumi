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

package cli

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/version"
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

func (auto *pulumiCommands) Debug(opts *cli.DebugOptions) cli.PulumiCommands {
	return &pulumiCommands{
		workDir: auto.workDir,
		home:    auto.home,
		envVars: auto.envVars,
		opts:    opts,
	}
}

func (auto *pulumiCommands) Stack(name string) cli.StackCommands {
	return &pulumiStackCommands{
		auto:  auto,
		stack: name,
	}
}

func (auto *pulumiCommands) Plugin() cli.PluginCommands {
	return &pulumiPluginCommands{auto: auto}
}

func (auto *pulumiCommands) WhoAmI(ctx context.Context) (*cli.WhoAmIResult, error) {
	var whoCmd whoAmICmd
	who, err := whoCmd.whoAmI(ctx)
	if err != nil {
		return nil, err
	}
	return &cli.WhoAmIResult{
		User:          who.User,
		Organizations: who.Organizations,
		URL:           who.URL,
	}, nil
}

func (auto *pulumiCommands) Version(ctx context.Context) (*semver.Version, error) {
	if version.Version == "" {
		return &semver.Version{}, nil
	}
	v, err := semver.ParseTolerant(version.Version)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

type pulumiPluginCommands struct {
	auto *pulumiCommands
}

func (auto *pulumiPluginCommands) Install(ctx context.Context, name, version string, opts *cli.PluginInstallOptions) error {
	panic("NYI")
}

func (auto *pulumiPluginCommands) Rm(ctx context.Context, name, version string) {
	panic("NYI")
}

func (auto *pulumiPluginCommands) Ls(ctx context.Context) ([]workspace.PluginInfo, error) {
	panic("NYI")
}

type pulumiStackCommands struct {
	auto *pulumiCommands

	stack string
}

func (auto *pulumiStackCommands) Config() cli.ConfigCommands {
	return &pulumiConfigCommands{stack: auto}
}

func (auto *pulumiStackCommands) Tag() cli.TagCommands {
	return &pulumiTagCommands{stack: auto}
}

func (auto *pulumiStackCommands) Preview(ctx context.Context, opts *cli.StackPreviewOptions) (*cli.StackPreviewResult, error) {
	panic("NYI")
}

func (auto *pulumiStackCommands) Update(ctx context.Context, opts *cli.StackUpdateOptions) (*cli.StackUpdateResult, error) {
	panic("NYI")
}

func (auto *pulumiStackCommands) Refresh(ctx context.Context, opts *cli.StackRefreshOptions) (*cli.StackRefreshResult, error) {
	panic("NYI")
}

func (auto *pulumiStackCommands) Destroy(ctx context.Context, opts *cli.StackDestroyOptions) (*cli.StackDestroyResult, error) {
	panic("NYI")
}

func (auto *pulumiStackCommands) Cancel(ctx context.Context) error {
	panic("NYI")
}

func (auto *pulumiStackCommands) Init(ctx context.Context, opts *cli.StackInitOptions) error {
	panic("NYI")
}

func (auto *pulumiStackCommands) Select(ctx context.Context) error {
	panic("NYI")
}

func (auto *pulumiStackCommands) Rm(ctx context.Context, opts *cli.StackRmOptions) error {
	panic("NYI")
}

func (auto *pulumiStackCommands) Ls(ctx context.Context) ([]cli.StackSummary, error) {
	panic("NYI")
}

func (auto *pulumiStackCommands) Export(ctx context.Context) (apitype.UntypedDeployment, error) {
	panic("NYI")
}

func (auto *pulumiStackCommands) Import(ctx context.Context, state apitype.UntypedDeployment) error {
	panic("NYI")
}

func (auto *pulumiStackCommands) Outputs(ctx context.Context) (map[string]cli.OutputValue, error) {
	panic("NYI")
}

func (auto *pulumiStackCommands) History(ctx context.Context, opts *cli.StackHistoryOptions) ([]cli.UpdateSummary, error) {
	panic("NYI")
}

type pulumiConfigCommands struct {
	stack *pulumiStackCommands
}

func (auto *pulumiConfigCommands) Get(ctx context.Context, key string, opts *cli.ConfigGetOptions) (cli.ConfigValue, error) {
	panic("NYI")
}

func (auto *pulumiConfigCommands) Set(ctx context.Context, key string, value cli.ConfigValue, opts *cli.ConfigSetOptions) error {
	panic("NYI")
}

func (auto *pulumiConfigCommands) GetAll(ctx context.Context) (map[string]cli.ConfigValue, error) {
	panic("NYI")
}

func (auto *pulumiConfigCommands) SetAll(ctx context.Context, values map[string]cli.ConfigValue) error {
	panic("NYI")
}

func (auto *pulumiConfigCommands) Rm(ctx context.Context, key string, opts *cli.ConfigRmOptions) error {
	panic("NYI")
}

func (auto *pulumiConfigCommands) Refresh(ctx context.Context) error {
	panic("NYI")
}

type pulumiTagCommands struct {
	stack *pulumiStackCommands
}

func (auto *pulumiTagCommands) Get(ctx context.Context, key string) (string, error) {
	panic("NYI")
}

func (auto *pulumiTagCommands) Set(ctx context.Context, key, value string) error {
	panic("NYI")
}

func (auto *pulumiTagCommands) Rm(ctx context.Context, key string) error {
	panic("NYI")
}

func (auto *pulumiTagCommands) Ls(ctx context.Context) (map[string]string, error) {
	panic("NYI")
}
