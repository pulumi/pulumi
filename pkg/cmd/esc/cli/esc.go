// Copyright 2023, Pulumi Corporation.
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
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Options struct {
	ParentPath string
	UserAgent  string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Colors colors.Colorization

	Login httpstate.LoginManager

	fs      escFS
	environ environ
	exec    cmdExec
	pager   pager
	ws      pkgWorkspace.Context

	newClient func(userAgent, backendURL, accessToken string, insecure bool) client.Client
}

type escCommand struct {
	fs      escFS
	environ environ
	exec    cmdExec
	pager   pager
	ws      pkgWorkspace.Context

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	command string
	colors  colors.Colorization

	login httpstate.LoginManager

	userAgent string
	newClient func(userAgent, backendURL, accessToken string, insecure bool) client.Client
	client    client.Client
	account   Account
}

func newESC(opts *Options) *escCommand {
	fs := valueOrDefault(opts.fs, newFS())

	esc := &escCommand{
		fs:        fs,
		environ:   valueOrDefault(opts.environ, newEnviron()),
		exec:      valueOrDefault(opts.exec, newCmdExec()),
		pager:     valueOrDefault(opts.pager, newPager()),
		stdin:     valueOrDefault(opts.Stdin, io.Reader(os.Stdin)),
		stdout:    valueOrDefault(opts.Stdout, io.Writer(os.Stdout)), //nolint:forbidigo,lll // default writer for the ESC CLI root command
		stderr:    valueOrDefault(opts.Stderr, io.Writer(os.Stderr)), //nolint:forbidigo,lll // default writer for the ESC CLI root command
		command:   valueOrDefault(opts.ParentPath, "esc"),
		colors:    valueOrDefault(opts.Colors, cmdutil.GetGlobalColorization()),
		login:     valueOrDefault(opts.Login, httpstate.NewLoginManager()),
		ws:        valueOrDefault(opts.ws, pkgWorkspace.Instance),
		userAgent: valueOrDefault(opts.UserAgent, fmt.Sprintf("esc-cli/1 (%s; %s)", version.Version, runtime.GOOS)),
		newClient: opts.newClient,
	}
	if esc.newClient == nil {
		esc.newClient = client.New
	}

	return esc
}

// New creates a new ESC CLI instance.
func New(opts *Options) *cobra.Command {
	if opts == nil {
		opts = &Options{}
	}

	command := "esc"
	if opts.ParentPath != "" {
		command = opts.ParentPath + " esc"
	}

	cmd := &cobra.Command{
		Use:   "esc",
		Short: "Pulumi ESC command line",
		Long: fmt.Sprintf("Pulumi ESC - Manage environments, secrets, and configuration\n"+
			"\n"+
			"To begin working with Pulumi ESC, run the `%[1]s env init` command:\n"+
			"\n"+
			"    $ %[1]s env init\n"+
			"\n"+
			"This will prompt you to create a new environment to hold secrets and configuration.\n"+
			"\n"+
			"The most common commands from there are:\n"+
			"\n"+
			"    - %[1]s env get  : Get a property in an environment definition\n"+
			"    - %[1]s env set  : Set a property in an environment definition\n"+
			"    - %[1]s env edit : Edit an environment definition\n"+
			"    - %[1]s env ls   : List available environments\n"+
			"    - %[1]s run      : Run a command within the context of an environment\n"+
			"    - %[1]s open     : Open an environment and access its contents\n"+
			"\n"+
			"For more information, please visit the project page: https://www.pulumi.com/docs/esc", command),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	esc := newESC(opts)

	env := newEnvCmd(esc)
	cmd.AddCommand(env)

	// Add top-level open/run aliases. We copy the commands to new struct values because
	// `AddCommand` mutates the command, modifying its parent, which can cause issues
	// with generated docs.
	openCmdCopy := *getCommand(env, "open")
	runCmdCopy := *getCommand(env, "run")
	cmd.AddCommand(&openCmdCopy)
	cmd.AddCommand(&runCmdCopy)

	cmd.AddCommand(newVersionCmd(esc))
	cmd.AddCommand(newGenDocsCmd(cmd))

	return cmd
}

func getCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func valueOrDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}

// Looks up the default org.
// Prefers default org that the user has configured locally in their ~/.pulumi/config.json
// If unset, then it will attempt to make an API call to the backend to determine the service's opinion
// of which user organization should be the default; defaults to individual org otherwise if unset.
func (esc *escCommand) lookupDefaultOrg(ctx context.Context, backendURL, username string) (string, error) {
	// Read the locally-configured default org from Pulumi's shared config, exactly as the rest of
	// the Pulumi CLI does (see pkg/backend/organizations.go).
	cfg, err := workspace.GetPulumiConfig()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if bc, ok := cfg.BackendConfig[backendURL]; ok && bc.DefaultOrg != "" {
		return bc.DefaultOrg, nil
	}

	if esc.client != nil {
		backendDefaultOrg, err := esc.client.GetDefaultOrg(ctx)
		if err != nil {
			return backendDefaultOrg, err
		} else if backendDefaultOrg != "" {
			return backendDefaultOrg, err
		}
	}

	// If client is unset, or if neither user nor backend have default configured, return the individual org.
	return username, nil
}
