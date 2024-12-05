// Copyright 2016-2018, Pulumi Corporation.
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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newPolicyPublishCmd() *cobra.Command {
	var policyPublishCmd policyPublishCmd
	cmd := &cobra.Command{
		Use:   "publish [org-name]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Publish a Policy Pack to the Pulumi Cloud",
		Long: "Publish a Policy Pack to the Pulumi Cloud\n" +
			"\n" +
			"If an organization name is not specified, the default org (if set) or the current user account is used.",
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			return policyPublishCmd.Run(cmd.Context(), cmdBackend.DefaultLoginManager, args)
		}),
	}

	return cmd
}

type policyPublishCmd struct {
	getwd      func() (string, error)
	defaultOrg func(*workspace.Project) (string, error)
}

func (cmd *policyPublishCmd) Run(ctx context.Context, lm cmdBackend.LoginManager, args []string) error {
	if cmd.getwd == nil {
		cmd.getwd = os.Getwd
	}
	if cmd.defaultOrg == nil {
		cmd.defaultOrg = pkgWorkspace.GetBackendConfigDefaultOrg
	}
	var orgName string
	if len(args) > 0 {
		orgName = args[0]
	} else if len(args) == 0 {
		project, _, err := pkgWorkspace.Instance.ReadProject()
		if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
			return err
		}
		org, err := cmd.defaultOrg(project)
		if err != nil {
			return err
		}
		orgName = org
	}

	//
	// Construct a policy pack reference of the form `<org-name>/<policy-pack-name>`
	// with the org name and an empty policy pack name. The policy pack name is empty
	// because it will be determined as part of the publish operation. If the org name
	// is empty, the current user account is used.
	//

	if strings.Contains(orgName, "/") {
		return errors.New("organization name must not contain slashes")
	}
	policyPackRef := orgName + "/"

	//
	// Obtain current PolicyPack, tied to the Pulumi Cloud backend.
	//

	policyPack, err := requirePolicyPack(ctx, policyPackRef, lm)
	if err != nil {
		return err
	}

	//
	// Load metadata about the current project.
	//

	pwd, err := cmd.getwd()
	if err != nil {
		return err
	}

	proj, _, root, err := readPolicyProject(pwd)
	if err != nil {
		return err
	}

	projinfo := &engine.PolicyPackInfo{Proj: proj, Root: root}
	pwd, _, err = projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	plugctx, err := plugin.NewContextWithRoot(cmdutil.Diag(), cmdutil.Diag(), nil, pwd, projinfo.Root,
		projinfo.Proj.Runtime.Options(), false, nil, nil, nil, nil)
	if err != nil {
		return err
	}

	//
	// Attempt to publish the PolicyPack.
	//

	err = policyPack.Publish(ctx, backend.PublishOperation{
		Root: root, PlugCtx: plugctx, PolicyPack: proj, Scopes: backend.CancellationScopes,
	})
	if err != nil {
		return err
	}

	return nil
}

func requirePolicyPack(
	ctx context.Context,
	policyPack string,
	lm cmdBackend.LoginManager,
) (backend.PolicyPack, error) {
	//
	// Attempt to log into cloud backend.
	//

	// Try to read the current project
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject()
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}

	cloudURL, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("`pulumi policy` command requires the user to be logged into the Pulumi Cloud: %w", err)
	}

	displayOptions := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	b, err := lm.Login(ctx, ws, cmdutil.Diag(), cloudURL, project, true /* setCurrent*/, displayOptions.Color)
	if err != nil {
		return nil, err
	}

	//
	// Obtain PolicyPackReference.
	//

	policy, err := b.GetPolicyPack(ctx, policyPack, cmdutil.Diag())
	if err != nil {
		return nil, err
	}
	if policy != nil {
		return policy, nil
	}

	return nil, fmt.Errorf("Could not find PolicyPack %q", policyPack)
}
