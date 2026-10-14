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

// Package local implements sandbox stacks, which deploy against local cloud emulators instead of real clouds.
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	sandboxrules "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers/sandbox"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

// ApplySandbox sets up sandbox mode for an operation on the given stack. For a sandbox stack it redirects the
// operation's providers at local cloud emulators, starting each emulator the first time a provider needs it. Other
// stacks are left untouched.
func ApplySandbox(ctx context.Context, w io.Writer, s backend.Stack, opts *engine.UpdateOptions) error {
	if !backend.IsSandboxStack(s) {
		return nil
	}

	autostart := !env.SandboxNoAutostart.Value()
	mode, err := sandboxrules.New(sandboxrules.Options{
		Ensure: func(ctx context.Context, name string, start bool, sink diag.Sink) (sandboxrules.Emulator, error) {
			emulator, ok := LookupEmulator(name)
			if !ok {
				return sandboxrules.Emulator{}, fmt.Errorf("unknown local emulator %q", name)
			}
			floci, err := NewFloci(emulator, sinkWriter{sink})
			if err != nil {
				return sandboxrules.Emulator{}, err
			}
			if !start {
				return sandboxrules.Emulator{Endpoint: floci.Endpoint}, nil
			}
			if err := floci.EnsureRunning(ctx, autostart); err != nil {
				return sandboxrules.Emulator{}, err
			}
			return describeEmulator(ctx, floci, sink), nil
		},
		AllowedPackages: strings.Split(env.SandboxAllowPackages.Value(), ","),
	})
	if err != nil {
		return err
	}
	opts.Sandbox = mode

	fmt.Fprintf(w, "Stack %s is a sandbox stack; deploying against local cloud emulators\n", s.Ref())
	if os.Getenv("PULUMI_DEBUG_PROVIDERS") != "" {
		fmt.Fprintln(w, "warning: providers attached with PULUMI_DEBUG_PROVIDERS don't receive sandbox mode's "+
			"environment variables")
	}
	return nil
}

// describeEmulator reports which emulator a deployment uses, warning when it's older than the release Pulumi pins.
func describeEmulator(ctx context.Context, floci *Floci, sink diag.Sink) sandboxrules.Emulator {
	described := sandboxrules.Emulator{Endpoint: floci.Endpoint, DisplayName: floci.DisplayName}
	status, err := floci.Status(ctx)
	if err != nil || status.Version == "" {
		return described
	}
	described.DisplayName = floci.Container + " " + status.Version
	described.Services = status.Services

	if sink != nil {
		sink.Infof(diag.RawMessage("", fmt.Sprintf("Using %s at %s", described.DisplayName, floci.Endpoint)))
		if floci.OlderThanPinned(status.Version) {
			sink.Warningf(diag.RawMessage("", fmt.Sprintf(
				"%s is running, but sandbox stacks are tested with %s; to upgrade, run `pulumi sandbox stop` "+
					"and `docker rm %s`, and the next deployment starts %s",
				described.DisplayName, floci.PinnedVersion(), floci.Container, floci.Image)))
		}
	}
	return described
}

// sinkWriter reports progress written while a deployment is running as diagnostics, so it doesn't break up the
// update's display.
type sinkWriter struct{ sink diag.Sink }

func (w sinkWriter) Write(p []byte) (int, error) {
	if w.sink != nil {
		w.sink.Infof(diag.RawMessage("", strings.TrimRight(string(p), "\n")))
	}
	return len(p), nil
}

// NewSandboxCmd returns the `pulumi sandbox` command group.
func NewSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage the local cloud emulators used by sandbox stacks",
		Long: "Manage the local cloud emulators used by sandbox stacks.\n" +
			"\n" +
			"Sandbox stacks, created with `pulumi stack init --sandbox` or `pulumi new --sandbox`, deploy\n" +
			"against the floci emulators for AWS and Google Cloud instead of the real clouds. Pulumi\n" +
			"starts an emulator automatically the first time a stack needs it, when the floci CLI or\n" +
			"Docker is installed.",
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newSandboxStatusCmd(), newSandboxStopCmd(), newSandboxEnvCmd())
	return cmd
}

func newSandboxStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the local cloud emulators are running",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			w := cmd.OutOrStdout()
			for i, emulator := range Emulators {
				floci, err := NewFloci(emulator, cmd.ErrOrStderr())
				if err != nil {
					return err
				}

				state := "not running"
				status, err := floci.Status(ctx)
				running := err == nil
				if running {
					state = "running"
				}
				launcher := string(floci.Launcher(ctx))
				if launcher == "" {
					launcher = "none (install the floci CLI or Docker)"
				}

				if i > 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintf(w, "%s: %s\n", floci.DisplayName, state)
				fmt.Fprintf(w, "  Endpoint:  %s\n", floci.Endpoint)
				if running && status.Version != "" {
					version := fmt.Sprintf("%s (Pulumi starts %s)", status.Version, floci.PinnedVersion())
					if floci.OlderThanPinned(status.Version) {
						version += "; older than expected, run `pulumi sandbox stop` and `docker rm " +
							floci.Container + "` to upgrade"
					}
					fmt.Fprintf(w, "  Version:   %s\n", version)
					fmt.Fprintf(w, "  Services:  %d available\n", countAvailable(status.Services))
				}
				fmt.Fprintf(w, "  Launcher:  %s\n", launcher)
				fmt.Fprintf(w, "  Data:      %s\n", floci.DataDir)
			}
			return nil
		},
	}
}

func countAvailable(services map[string]bool) int {
	n := 0
	for _, available := range services {
		if available {
			n++
		}
	}
	return n
}

func newSandboxStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the local cloud emulators",
		Long: "Stop the local cloud emulators.\n" +
			"\n" +
			"Emulated resources are kept on disk and are available again the next time the\n" +
			"emulators start.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			stopped := 0
			for _, emulator := range Emulators {
				floci, err := NewFloci(emulator, cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				if !floci.Healthy(ctx) {
					continue
				}
				if err := floci.Stop(ctx); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Stopped %s\n", floci.DisplayName)
				stopped++
			}
			if stopped == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No local cloud emulators are running")
			}
			return nil
		},
	}
}
