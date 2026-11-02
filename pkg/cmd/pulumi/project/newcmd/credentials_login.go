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

package newcmd

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/blang/semver"
	"github.com/kballard/go-shellquote"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

// cloudCLI runs a cloud's CLI on behalf of the credentials preflight.
type cloudCLI interface {
	// Output runs a command and returns its standard output.
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
	// RunAttached runs a command connected to the user's terminal, so that it can prompt.
	RunAttached(ctx context.Context, name string, args ...string) error
}

type execCLI struct{}

func (execCLI) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execCLI) RunAttached(ctx context.Context, name string, args ...string) error {
	// The cobra writers may be buffered or piped; the sign-in needs the real terminal.
	cmd := exec.CommandContext(ctx, name, args...)
	// `aws login` otherwise follows a successful sign-in with an offer to set up AI coding agents,
	// which would interrupt `pulumi new` with an unrelated wizard.
	cmd.Env = append(os.Environ(), "AWS_CLI_AGENT_TOOLKIT_HINT_DISABLED=true")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout //nolint:forbidigo
	cmd.Stderr = os.Stderr //nolint:forbidigo
	return cmd.Run()
}

// awsSignInCommand is an AWS CLI sign-in that pulumi-aws recommends in some of its credentials errors.
type awsSignInCommand struct {
	// hint is how the provider's error recommends the sign-in.
	hint string
	// command is the AWS CLI command that signs in.
	command []string
	// minVersion is the first AWS CLI release with the command.
	minVersion semver.Version
	// takesRegion is whether the command asks for a region unless given one, which the user has
	// just given `pulumi new`.
	takesRegion bool
	// noBrowserFlag makes the command sign in on another device when there is no local browser.
	noBrowserFlag string
}

var awsSignInCommands = []awsSignInCommand{
	// Recommended when there are no credentials at all or the login session has expired, and only by
	// releases whose SDK can use the session that `aws login` creates.
	{
		hint: "`aws login", command: []string{"login"}, minVersion: semver.MustParse("2.32.0"),
		takesRegion: true, noBrowserFlag: "--remote",
	},
	// Recommended when the SSO session of the selected profile has expired.
	{
		hint: "`aws sso login", command: []string{"sso", "login"}, minVersion: semver.MustParse("2.0.0"),
		noBrowserFlag: "--no-browser",
	},
}

// awsSignInCommandFor returns the sign-in that the AWS provider recommended to fix the problem, if any.
func awsSignInCommandFor(problem *credentialsProblem) (awsSignInCommand, bool) {
	for _, c := range awsSignInCommands {
		for _, line := range problem.details {
			if strings.Contains(line, c.hint) {
				return c, true
			}
		}
	}
	return awsSignInCommand{}, false
}

// awsCLIVersion parses the output of `aws --version`, such as
// "aws-cli/2.32.0 Python/3.13.9 Darwin/24.6.0 source/arm64".
func awsCLIVersion(versionOutput []byte) (semver.Version, bool) {
	fields := strings.Fields(string(versionOutput))
	if len(fields) == 0 {
		return semver.Version{}, false
	}
	version, ok := strings.CutPrefix(fields[0], "aws-cli/")
	if !ok {
		return semver.Version{}, false
	}
	v, err := semver.ParseTolerant(version)
	return v, err == nil
}

// awsSignIn offers to run the AWS CLI sign-in that the AWS provider recommends for a credentials problem.
type awsSignIn struct {
	cli     cloudCLI
	cfg     config.Map
	stdout  io.Writer
	confirm func(prompt string) bool
	getenv  func(key string) string
}

// offer runs the sign-in if the provider recommended one and the user agrees, and reports whether
// the user signed in.
func (s awsSignIn) offer(ctx context.Context, cp cloudProvider, problem *credentialsProblem) bool {
	if cp.pkg != "aws" {
		return false
	}
	c, ok := awsSignInCommandFor(problem)
	if !ok {
		return false
	}
	// This also fails when the AWS CLI is not installed.
	out, err := s.cli.Output(ctx, "aws", "--version")
	if version, ok := awsCLIVersion(out); err != nil || !ok || version.LT(c.minVersion) {
		slog.DebugContext(ctx, "not offering AWS sign-in", "command", c.command, "version", string(out), "err", err)
		return false
	}

	args := s.args(c)
	command := shellquote.Join(append([]string{"aws"}, args...)...)
	if !s.confirm(fmt.Sprintf("Sign in to %s now? This runs `%s`.", cp.displayName, command)) {
		fmt.Fprintln(s.stdout)
		return false
	}
	if err := s.cli.RunAttached(ctx, "aws", args...); err != nil {
		if ctx.Err() == nil {
			fmt.Fprintf(s.stdout, "`%s` did not complete; run it yourself, then `pulumi up`.\n\n", command)
		}
		return false
	}
	return true
}

// args builds the AWS CLI arguments that sign in the profile and region the stack uses.
func (s awsSignIn) args(c awsSignInCommand) []string {
	cfg := providerConfigProperties("aws", s.cfg)
	setting := func(key string, envVars ...string) string {
		if v := cfg.Get(key); v.IsString() && v.AsString() != "" {
			return v.AsString()
		}
		for _, name := range envVars {
			if v := s.getenv(name); v != "" {
				return v
			}
		}
		return ""
	}

	args := slices.Clone(c.command)
	if profile := setting("profile", "AWS_PROFILE"); profile != "" {
		args = append(args, "--profile", profile)
	}
	if region := setting("region", "AWS_REGION", "AWS_DEFAULT_REGION"); c.takesRegion && region != "" {
		args = append(args, "--region", region)
	}
	if s.getenv("SSH_CONNECTION") != "" || s.getenv("SSH_TTY") != "" {
		args = append(args, c.noBrowserFlag)
	}
	return args
}
