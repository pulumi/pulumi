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

package config

import (
	"context"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"os"
	"path"
	"strings"

	"github.com/pkg/browser"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

// cloudURLProvider is the subset of the Pulumi Cloud backend interface needed
// to construct the console URL. Keeping it local avoids importing the httpstate package.
type cloudURLProvider interface {
	CloudURL() string
}

func newConfigWebCmd(ws pkgWorkspace.Context, stackRef *string) *cobra.Command {
	impl := &configWebCmd{
		ws:       ws,
		stackRef: stackRef,
		stdout:   os.Stdout,
	}

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Open the ESC environment for a service-backed stack in the Pulumi Cloud console",
		Long: `Opens the ESC environment for a service-backed stack in the Pulumi Cloud console.

This command is only supported for service-backed stacks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return impl.run(cmd.Context())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	return cmd
}

type configWebCmd struct {
	ws       pkgWorkspace.Context
	stackRef *string
	stdout   io.Writer

	// openURL is overridable for testing.
	openURL func(url string) error
}

func (cmd *configWebCmd) run(ctx context.Context) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	stack, err := cmdStack.RequireStack(
		ctx,
		cmdutil.Diag(),
		cmd.ws,
		cmdBackend.DefaultLoginManager,
		*cmd.stackRef,
		cmdStack.OfferNew|cmdStack.SetCurrent,
		opts,
	)
	if err != nil {
		return err
	}

	loc := stack.ConfigLocation()
	if !loc.IsRemote || loc.EscEnv == nil {
		return errors.New("config web is only supported for service-backed stacks;\n" +
			"  use `pulumi config` to view local stack configuration")
	}

	cloudBe, ok := stack.Backend().(cloudURLProvider)
	if !ok {
		return errors.New("config web requires a Pulumi Cloud backend")
	}

	orgName, err := stackOrgName(stack)
	if err != nil {
		return err
	}
	envProject, envName, err := parseEscEnvRef(*loc.EscEnv)
	if err != nil {
		return err
	}

	consoleURL := escEnvironmentConsoleURL(cloudBe.CloudURL(), orgName, envProject, envName)
	if consoleURL == "" {
		return errors.New("could not determine Pulumi Cloud console URL")
	}

	openFn := cmd.openURL
	if openFn == nil {
		openFn = browser.OpenURL
	}
	if err := openFn(consoleURL); err != nil {
		// Browser open failed — print URL so the user can open it manually.
		fmt.Fprintf(cmd.stdout, "Could not open browser: %v\n%s\n", err, consoleURL)
	} else {
		fmt.Fprintln(cmd.stdout, consoleURL)
	}

	return nil
}

// escEnvironmentConsoleURL constructs the Pulumi Cloud console URL for an ESC environment.
// It transforms the API URL (e.g. https://api.pulumi.com) to the console URL (https://app.pulumi.com).
func escEnvironmentConsoleURL(cloudURL, orgName, envProject, envName string) string {
	u, err := neturl.Parse(cloudURL)
	if err != nil {
		return ""
	}
	if strings.HasPrefix(u.Host, "api.") {
		u.Host = "app." + strings.TrimPrefix(u.Host, "api.")
	}
	u.Path = path.Join(u.Path, orgName, "esc", envProject, envName)
	return u.String()
}
