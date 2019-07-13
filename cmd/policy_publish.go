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

package cmd

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/engine"

	"github.com/pulumi/pulumi/pkg/backend"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/spf13/cobra"
)

func newPolicyPublishCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "publish <orgName>/<policyPackName>",
		Args:  cmdutil.ExactArgs(1),
		Short: "Publish resource policies to the Pulumi service",
		Long:  "Publish resource policies to the Pulumi service",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			//
			// Obtain current PolicyPack, tied to the Pulumi service backend.
			//

			policyPack, err := requirePolicyPack(args[0])
			if err != nil {
				return err
			}

			//
			// Load metadata about the current project.
			//

			// TODO: `readPolicyProject` instead?
			proj, root, err := readProject()
			if err != nil {
				return err
			}

			projinfo := &engine.Projinfo{Proj: proj, Root: root}
			_ /*pwd*/, _ /*main*/, plugctx, err := engine.ProjectInfoContext(
				projinfo, nil, nil, cmdutil.Diag(), cmdutil.Diag(), nil)
			if err != nil {
				return err
			}

			//
			// Attempt to publish the PolicyPack.
			//

			res := policyPack.Publish(commandContext(), backend.PublishOperation{
				Root: root, PlugCtx: plugctx, Scopes: cancellationScopes})
			if res != nil && res.Error() != nil {
				return res.Error()
			}

			return nil
		}),
	}

	return cmd
}

func requirePolicyPack(policyPack string) (backend.PolicyPack, error) {
	//
	// Attempt to log into cloud backend.
	//

	cloudURL, err := workspace.GetCurrentCloudURL()
	if err != nil {
		return nil, errors.Wrap(err,
			"`pulumi policy` command requires the user to be logged into the Pulumi service")
	}

	displayOptions := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	b, err := httpstate.Login(commandContext(), cmdutil.Diag(), cloudURL, displayOptions)
	if err != nil {
		return nil, err
	}

	//
	// Obtain PolicyPackReference.
	//

	policy, err := b.GetPolicyPack(commandContext(), policyPack, cmdutil.Diag())
	if err != nil {
		return nil, err
	}
	if policy != nil {
		return policy, nil
	}

	return nil, fmt.Errorf("Could not find PolicyPack %q", policyPack)
}
