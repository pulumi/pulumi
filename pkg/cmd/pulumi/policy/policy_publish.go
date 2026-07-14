// Copyright 2016, Pulumi Corporation.
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

package policy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/metadata"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/codegen/convert"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	pkghost "github.com/pulumi/pulumi/pkg/v3/host"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func newPolicyPublishCmd() *cobra.Command {
	var policyPublishCmd policyPublishCmd
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a Policy Pack to the Pulumi Cloud",
		Long: "Publish a Policy Pack to the Pulumi Cloud\n" +
			"\n" +
			"If an organization name is not specified, the default org (if set) or the current user account is used.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return policyPublishCmd.Run(cmd.Context(), cmdBackend.DefaultLoginManager, args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "org-name"},
		},
		Required: 0,
	})

	cmd.Flags().StringArrayVar(&policyPublishCmd.binaryFlags, "binary", nil,
		"Pre-built analyzer binary to publish for a platform, as <os>-<arch>=<path>, "+
			"where <path> is relative to the policy pack directory "+
			"(repeatable; overrides bin/ discovery)")
	cmd.Flags().BoolVar(&policyPublishCmd.sourceOnly, "source-only", false,
		"Publish only the source archive, skipping binary discovery")

	return cmd
}

type policyPublishCmd struct {
	getwd func() (string, error)

	binaryFlags []string
	sourceOnly  bool
}

// resolveBinaries validates the --binary/--source-only flags and returns explicit
// binary overrides (nil means discover by convention) and the source-only setting.
func (cmd *policyPublishCmd) resolveBinaries() (map[string]string, bool, error) {
	if cmd.sourceOnly && len(cmd.binaryFlags) > 0 {
		return nil, false, errors.New("--source-only cannot be combined with --binary")
	}
	if cmd.sourceOnly {
		return nil, true, nil
	}
	binaries, err := workspace.ParsePolicyBinaryOverrides(cmd.binaryFlags)
	if err != nil {
		return nil, false, err
	}
	return binaries, false, nil
}

func (cmd *policyPublishCmd) Run(ctx context.Context, lm cmdBackend.LoginManager, args []string) error {
	if cmd.getwd == nil {
		cmd.getwd = os.Getwd
	}

	b, err := loginToCloudBackend(ctx, lm)
	if err != nil {
		return err
	}

	var orgName string
	if len(args) > 0 {
		orgName = args[0]
	} else if len(args) == 0 {
		org, err := b.GetDefaultOrg(ctx)
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

	policyPack, err := requirePolicyPackForBackend(ctx, policyPackRef, b)
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

	proj, _, root, err := ReadPolicyProject(pwd)
	if err != nil {
		return err
	}

	projinfo := &engine.PolicyPackInfo{Proj: proj, Root: root}
	pwd, _, err = projinfo.GetPwdMain()
	if err != nil {
		return err
	}

	reg := cmdCmd.NewDefaultRegistry(ctx, lm, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())
	pluginHost, err := pkghost.New(context.WithoutCancel(ctx), cmdutil.Diag(), cmdutil.Diag(), nil,
		pkgWorkspace.EnsureLanguageInstalled, schema.NewLoaderServerFromContext, convert.NewMapperServerFromContext,
		packageworkspace.NewResolverServer(reg))
	if err != nil {
		return err
	}
	// host is owned here, closed after the context
	defer contract.IgnoreClose(pluginHost)
	plugctx, err := plugin.NewContextWithRoot(ctx, cmdutil.Diag(), cmdutil.Diag(), pluginHost, pwd, projinfo.Root,
		projinfo.Proj.Runtime.Options(), false, nil, nil, nil, nil)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(plugctx)

	// Get optional data about the environment performing the publish operation,
	// e.g. the current source code control commit information.
	m := metadata.GetPolicyPublishMetadata(root)

	binaries, sourceOnly, err := cmd.resolveBinaries()
	if err != nil {
		return err
	}

	//
	// Attempt to publish the PolicyPack.
	//

	err = policyPack.Publish(ctx, backend.PublishOperation{
		Root:       root,
		PlugCtx:    plugctx,
		PolicyPack: proj,
		Scopes:     backend.CancellationScopes,
		Metadata:   m,
		Binaries:   binaries,
		SourceOnly: sourceOnly,
	})
	if err != nil {
		return err
	}

	return nil
}

func loginToCloudBackend(
	ctx context.Context,
	lm cmdBackend.LoginManager,
) (backend.Backend, error) {
	// Try to read the current project
	ws := pkgWorkspace.Instance
	project, _, err := ws.ReadProject("")
	if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
		return nil, err
	}
	cloudURL, err := pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
	if err != nil {
		return nil, fmt.Errorf("`pulumi policy` command requires the user to be logged into the Pulumi Cloud: %w", err)
	}

	return lm.Current(ctx, ws, cmdutil.Diag(), cloudURL, project, true /* setCurrent*/)
}

// requirePolicyPack attempts to log into the cloud backend and retrieves the requested policy
// pack.
func requirePolicyPack(
	ctx context.Context,
	policyPack string,
	lm cmdBackend.LoginManager,
) (backend.PolicyPack, error) {
	b, err := loginToCloudBackend(ctx, lm)
	if err != nil {
		return nil, err
	}

	return requirePolicyPackForBackend(ctx, policyPack, b)
}

// requirePolicyPackForBackend retrieves a requested policy pack against a provided backend.
func requirePolicyPackForBackend(
	ctx context.Context,
	policyPack string,
	b backend.Backend,
) (backend.PolicyPack, error) {
	policy, err := b.GetPolicyPack(ctx, policyPack, cmdutil.Diag())
	if err != nil {
		return nil, err
	}
	if policy != nil {
		return policy, nil
	}

	return nil, fmt.Errorf("could not find PolicyPack %q", policyPack)
}
