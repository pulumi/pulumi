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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type pulumiCLIError struct {
	stdout string
	stderr string
	code   int
	err    error
}

func newPulumiCLIError(err error, stdout, stderr string, code int) *pulumiCLIError {
	return &pulumiCLIError{
		stdout,
		stderr,
		code,
		err,
	}
}

func (e *pulumiCLIError) Stdout() string {
	return e.stdout
}

func (e *pulumiCLIError) Stderr() string {
	return e.stderr
}

func (e *pulumiCLIError) Error() string {
	return fmt.Sprintf("%s\ncode: %d\nstdout: %s\nstderr: %s\n", e.err.Error(), e.code, e.stdout, e.stderr)
}

const unknownErrorCode = -2

func runPulumiCLI(
	ctx context.Context,
	workdir string,
	additionalOutput []io.Writer,
	additionalErrorOutput []io.Writer,
	additionalEnv []string,
	args ...string,
) (stdout, stderr string, status int, err error) {
	// all commands should be run in non-interactive mode.
	// this causes commands to fail rather than prompting for input (and thus hanging indefinitely)
	if !slices.Contains(args, "--non-interactive") {
		args = append(args, "--non-interactive")
	}

	cmd := exec.CommandContext(ctx, "pulumi", args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), additionalEnv...)

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	additionalOutput = append(additionalOutput, &stdoutBuf)
	additionalErrorOutput = append(additionalErrorOutput, &stderrBuf)
	cmd.Stdout = io.MultiWriter(additionalOutput...)
	cmd.Stderr = io.MultiWriter(additionalErrorOutput...)

	code := unknownErrorCode
	err = cmd.Run()
	if exitError, ok := err.(*exec.ExitError); ok {
		code = exitError.ExitCode()
	} else if err == nil {
		// If there was no error then the exit code was 0
		code = 0
	}
	return stdoutBuf.String(), stderrBuf.String(), code, err
}

type pulumiCLICommands struct {
	version *semver.Version

	workDir string
	home    string
	envVars map[string]string

	opts *DebugOptions
}

func OutOfProcess(workDir, home string, envVars map[string]string) *pulumiCLICommands {
	return &pulumiCLICommands{
		workDir: workDir,
		home:    home,
		envVars: envVars,
	}
}

func (cli *pulumiCLICommands) runPulumiCLI(
	ctx context.Context,
	args ...string,
) (stdout, stderr string, status int, err error) {
	var env []string
	if cli.home != "" {
		homeEnv := fmt.Sprintf("%s=%s", cli.home, cli.home)
		env = append(env, homeEnv)
	}
	for k, v := range cli.envVars {
		env = append(env, fmt.Sprintf("%v=%v", k, v))
	}
	return runPulumiCLI(
		ctx,
		cli.workDir,
		nil, /* additionalOutputs */
		nil, /* additionalErrorOutputs */
		env,
		args...,
	)
}

var minCLIVersion = semver.MustParse("3.2.0-alpha")

func (cli *pulumiCLICommands) getVersion(ctx context.Context) (*semver.Version, error) {
	if cli.version != nil {
		return cli.version, nil
	}

	stdout, stderr, errCode, err := cli.runPulumiCLI(ctx, "version")
	if err != nil {
		return nil, newPulumiCLIError(fmt.Errorf("could not determine pulumi version: %w", err), stdout, stderr, errCode)
	}

	const skipVersionCheckVar = "PULUMI_AUTOMATION_API_SKIP_VERSION_CHECK"

	optOut := cmdutil.IsTruthy(os.Getenv(skipVersionCheckVar))
	if val, ok := cli.envVars[skipVersionCheckVar]; ok {
		optOut = optOut || cmdutil.IsTruthy(val)
	}

	version, err := semver.ParseTolerant(stdout)
	if !optOut {
		if err != nil {
			return nil, fmt.Errorf("Unable to parse Pulumi CLI version: %w", err)
		}
		if minCLIVersion.Major < version.Major {
			return nil, fmt.Errorf("Major version mismatch. You are using Pulumi CLI version %v with Automation SDK v%v. Please update the SDK.", version, minCLIVersion.Major) //nolint
		}
		if minCLIVersion.GT(version) {
			return nil, fmt.Errorf("Minimum version requirement failed. The minimum CLI version requirement is %v, your current CLI version is %v. Please update the Pulumi CLI.", minCLIVersion, version) //nolint
		}
	}
	cli.version = &version
	return &version, nil
}

func (cli *pulumiCLICommands) Debug(opts *DebugOptions) PulumiCommands {
	return &pulumiCLICommands{
		version: cli.version,
		workDir: cli.workDir,
		home:    cli.home,
		envVars: cli.envVars,
		opts:    opts,
	}
}

func (cli *pulumiCLICommands) Stack(name string) StackCommands {
	return &pulumiCLIStackCommands{
		cli:   cli,
		stack: name,
	}
}

func (cli *pulumiCLICommands) Plugin() PluginCommands {
	return &pulumiCLIPluginCommands{cli: cli}
}

func (cli *pulumiCLICommands) WhoAmI(ctx context.Context) (*WhoAmIResult, error) {
	// 3.58 added the --json flag (https://github.com/pulumi/pulumi/releases/tag/v3.58.0)
	version, err := cli.getVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version.GTE(semver.Version{Major: 3, Minor: 58}) {
		stdout, stderr, errCode, err := cli.runPulumiCLI(ctx, "whoami", "--json")
		if err != nil {
			return nil, newPulumiCLIError(
				fmt.Errorf("could not retrieve WhoAmIDetailedInfo: %w", err), stdout, stderr, errCode)
		}

		var who WhoAmIResult
		if err = json.Unmarshal([]byte(stdout), &who); err != nil {
			return nil, fmt.Errorf("unable to unmarshal WhoAmI: %w", err)
		}
		return &who, nil
	}

	stdout, stderr, errCode, err := cli.runPulumiCLI(ctx, "whoami")
	if err != nil {
		return nil, newPulumiCLIError(
			fmt.Errorf("could not determine authenticated user: %w", err), stdout, stderr, errCode)
	}
	return &WhoAmIResult{User: strings.TrimSpace(stdout)}, nil
}

func (cli *pulumiCLICommands) Version(ctx context.Context) (*semver.Version, error) {
	return cli.getVersion(ctx)
}

type pulumiCLIPluginCommands struct {
	cli *pulumiCLICommands
}

func (cli *pulumiCLIPluginCommands) Install(ctx context.Context, name, version string, opts *PluginInstallOptions) error {
	panic("NYI")
}

func (cli *pulumiCLIPluginCommands) Rm(ctx context.Context, name, version string) {
	panic("NYI")
}

func (cli *pulumiCLIPluginCommands) Ls(ctx context.Context) ([]workspace.PluginInfo, error) {
	panic("NYI")
}

type pulumiCLIStackCommands struct {
	cli *pulumiCLICommands

	stack string
}

func (cli *pulumiCLIStackCommands) Config() ConfigCommands {
	return &pulumiCLIConfigCommands{stack: cli}
}

func (cli *pulumiCLIStackCommands) Tag() TagCommands {
	return &pulumiCLITagCommands{stack: cli}
}

func (cli *pulumiCLIStackCommands) Preview(ctx context.Context, opts *StackPreviewOptions) (*StackPreviewResult, error) {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Update(ctx context.Context, opts *StackUpdateOptions) (*StackUpdateResult, error) {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Refresh(ctx context.Context, opts *StackRefreshOptions) (*StackRefreshResult, error) {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Destroy(ctx context.Context, opts *StackDestroyOptions) (*StackDestroyResult, error) {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Cancel(ctx context.Context) error {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Init(ctx context.Context, opts *StackInitOptions) error {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Select(ctx context.Context) error {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Rm(ctx context.Context, opts *StackRmOptions) error {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Ls(ctx context.Context) ([]StackSummary, error) {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Export(ctx context.Context) (apitype.UntypedDeployment, error) {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Import(ctx context.Context, state apitype.UntypedDeployment) error {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) Outputs(ctx context.Context) (map[string]OutputValue, error) {
	panic("NYI")
}

func (cli *pulumiCLIStackCommands) History(ctx context.Context, opts *StackHistoryOptions) ([]UpdateSummary, error) {
	panic("NYI")
}

type pulumiCLIConfigCommands struct {
	stack *pulumiCLIStackCommands
}

func (cli *pulumiCLIConfigCommands) Get(ctx context.Context, key string, opts *ConfigGetOptions) (ConfigValue, error) {
	panic("NYI")
}

func (cli *pulumiCLIConfigCommands) Set(ctx context.Context, key string, value ConfigValue, opts *ConfigSetOptions) error {
	panic("NYI")
}

func (cli *pulumiCLIConfigCommands) GetAll(ctx context.Context) (map[string]ConfigValue, error) {
	panic("NYI")
}

func (cli *pulumiCLIConfigCommands) SetAll(ctx context.Context, values map[string]ConfigValue) error {
	panic("NYI")
}

func (cli *pulumiCLIConfigCommands) Rm(ctx context.Context, key string, opts *ConfigRmOptions) error {
	panic("NYI")
}

func (cli *pulumiCLIConfigCommands) Refresh(ctx context.Context) error {
	panic("NYI")
}

type pulumiCLITagCommands struct {
	stack *pulumiCLIStackCommands
}

func (cli *pulumiCLITagCommands) Get(ctx context.Context, key string) (string, error) {
	panic("NYI")
}

func (cli *pulumiCLITagCommands) Set(ctx context.Context, key, value string) error {
	panic("NYI")
}

func (cli *pulumiCLITagCommands) Rm(ctx context.Context, key string) error {
	panic("NYI")
}

func (cli *pulumiCLITagCommands) Ls(ctx context.Context) (map[string]string, error) {
	panic("NYI")
}
