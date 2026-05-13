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

package env

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cloud"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// envNewClient is the minimal interface needed to create an ESC environment.
// `*httpstate.cloudBackend` already satisfies this through its
// `CreateEnvironment` method declared on `backend.EnvironmentsBackend`.
type envNewClient interface {
	CreateEnvironment(ctx context.Context, org, project, name string, yaml []byte) (any, error)
}

type envNewFactory func(ctx context.Context, orgOverride string) (envNewClient, string, error)

func newEnvNewCmd() *cobra.Command {
	return newEnvNewCmdWith(nil)
}

func newEnvNewCmdWith(factory envNewFactory) *cobra.Command {
	var org string

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new Pulumi ESC environment",
		Long: "[EXPERIMENTAL] Create a new Pulumi ESC environment.\n" +
			"\n" +
			"The environment is created within the given project and starts with an\n" +
			"empty YAML definition. Names must be unique within a project and may only\n" +
			"contain alphanumeric characters, hyphens, underscores, and periods.\n" +
			"\n" +
			"Wraps the `CreateEnvironment` Pulumi Cloud REST endpoint.",
		Example: "  # Create an empty environment in the current default org.\n" +
			"  pulumi env new my-project my-env\n\n" +
			"  # Pin the organization explicitly.\n" +
			"  pulumi env new --org acme my-project my-env",
		RunE: func(cmd *cobra.Command, args []string) error {
			f := factory
			if f == nil {
				f = defaultEnvNewFactory
			}
			return runEnvNew(cmd.Context(), cmd.OutOrStdout(), f, org, args[0], args[1])
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "project"}, {Name: "name"}},
		Required:  2,
	})

	cmd.Flags().StringVar(&org, "org", "",
		"The organization to create the environment in (defaults to the current default org)")

	return cmd
}

func runEnvNew(
	ctx context.Context, out io.Writer, factory envNewFactory, orgOverride, project, name string,
) error {
	client, org, err := factory(ctx, orgOverride)
	if err != nil {
		return err
	}
	if _, err := client.CreateEnvironment(ctx, org, project, name, nil); err != nil {
		return fmt.Errorf("creating environment %s/%s in org %s: %w", project, name, org, err)
	}
	fmt.Fprintf(out, "Created environment %s/%s/%s\n", org, project, name)
	return nil
}

func defaultEnvNewFactory(ctx context.Context, orgOverride string) (envNewClient, string, error) {
	resolved, err := cloud.ResolveContext(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("resolving cloud context: %w", err)
	}
	if !resolved.LoggedIn {
		return nil, "", errors.New("not logged in to Pulumi Cloud; run `pulumi login` first")
	}
	org := orgOverride
	if org == "" {
		org = resolved.OrgName
	}
	if org == "" {
		return nil, "", errors.New(
			"no organization available; pass --org or set a default with `pulumi org set-default`")
	}
	be, err := cmdBackend.CurrentBackend(ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager,
		resolved.Project, display.Options{Color: cmdutil.GetGlobalColorization()})
	if err != nil {
		return nil, "", fmt.Errorf("resolving backend: %w", err)
	}
	envBe, ok := be.(backend.EnvironmentsBackend)
	if !ok {
		return nil, "", errors.New("the current backend does not support Pulumi ESC environments")
	}
	return envBackendAdapter{envBe}, org, nil
}

// envBackendAdapter is a thin shim that returns `any` so the interface compiles
// against `backend.EnvironmentsBackend.CreateEnvironment`, which returns
// `apitype.EnvironmentDiagnostics`. We don't surface diagnostics on a create-
// with-empty-yaml call because the create path doesn't run evaluation.
type envBackendAdapter struct{ inner backend.EnvironmentsBackend }

func (a envBackendAdapter) CreateEnvironment(
	ctx context.Context, org, project, name string, yaml []byte,
) (any, error) {
	return a.inner.CreateEnvironment(ctx, org, project, name, yaml)
}
