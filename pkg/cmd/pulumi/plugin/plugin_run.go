// Copyright 2016-2024, Pulumi Corporation.
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

package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/blang/semver"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type pluginRunCmd struct {
	kind string
}

func (cmd *pluginRunCmd) run(ctx context.Context, args []string) error {
	if !apitype.IsPluginKind(cmd.kind) {
		return fmt.Errorf("unrecognized plugin kind: %s", cmd.kind)
	}
	kind := apitype.PluginKind(cmd.kind)

	// Parse the name and version from the second argument in the form of "NAME[@VERSION]".
	name := args[0]
	var version *semver.Version
	if namePart, versionPart, ok := strings.Cut(name, "@"); ok {
		v, err := semver.ParseTolerant(versionPart)
		if err != nil {
			return fmt.Errorf("invalid plugin version %q: %w", versionPart, err)
		}
		name = namePart
		version = &v
	}

	if !tokens.IsName(name) {
		return fmt.Errorf("invalid plugin name %q", name)
	}

	pluginDesc := fmt.Sprintf("%s %s", kind, name)
	if version != nil {
		pluginDesc = fmt.Sprintf("%s@%s", pluginDesc, version)
	}

	d := diag.DefaultSink(os.Stdout, os.Stderr, diag.FormatOptions{Color: cmdutil.GetGlobalColorization()})

	path, err := workspace.GetPluginPath(d, kind, name, version, nil)
	if err != nil {
		// Try to install the plugin, unless auto plugin installs are turned off.
		var me *workspace.MissingError
		if !errors.As(err, &me) || env.DisableAutomaticPluginAcquisition.Value() {
			// Not a MissingError, return the original error.
			return fmt.Errorf("could not get plugin path: %w", err)
		}

		// TODO: Add support for --server and --checksums.
		pluginSpec, err := workspace.NewPluginSpec(args[0], kind, nil, "", nil)
		if err != nil {
			return err
		}

		log := func(sev diag.Severity, msg string) {
			d.Logf(sev, diag.RawMessage("", msg))
		}

		_, err = pkgWorkspace.InstallPlugin(ctx, pluginSpec, log)
		if err != nil {
			return err
		}

		path, err = workspace.GetPluginPath(d, kind, name, version, nil)
		if err != nil {
			return fmt.Errorf("could not get plugin path: %w", err)
		}
	}

	pluginArgs := args[1:]

	pluginCmd := exec.Command(path, pluginArgs...)
	pluginCmd.Stdout = os.Stdout
	pluginCmd.Stderr = os.Stderr
	pluginCmd.Stdin = os.Stdin
	if err := pluginCmd.Run(); err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			syscallErr, ok := pathErr.Err.(syscall.Errno)
			if ok && syscallErr == syscall.ENOENT {
				return fmt.Errorf("could not find execute plugin %s, binary not found at %s", pluginDesc, path)
			}
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}

		return fmt.Errorf("could not execute plugin %s (%s): %w", pluginDesc, path, err)
	}

	return nil
}

func newPluginRunCmd() *cobra.Command {
	var c pluginRunCmd

	cmd := &cobra.Command{
		Use:    "run NAME[@VERSION] [ARGS]",
		Args:   cmdutil.MinimumNArgs(1),
		Hidden: !env.Dev.Value(),
		Short:  "Run a command on a plugin binary",
		Long: "[EXPERIMENTAL] Run a command on a plugin binary.\n" +
			"\n" +
			"Directly executes a plugin binary, if VERSION is not specified " +
			"the latest installed plugin will be used.",
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			return c.run(cmd.Context(), args)
		}),
	}

	cmd.PersistentFlags().StringVar(&c.kind,
		"kind", "tool", "The plugin kind")

	return cmd
}
