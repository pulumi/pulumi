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

package org

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newOrgRoleNewCmd() *cobra.Command {
	return newOrgRoleNewCmdWith(defaultOrgRoleClientFactory)
}

func newOrgRoleNewCmdWith(factory orgRoleClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "factory must not be nil")

	var (
		org         string
		description string
		purpose     string
	)
	output := defaultRoleSingleOutput()

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "new <name> <details-file>",
		Short:  "Create a new custom role for an organization",
		Long: "[EXPERIMENTAL] Create a new custom role for an organization.\n" +
			"\n" +
			"The role's permission tree is read from the JSON file at <details-file>.\n" +
			"Pass `-` to read the JSON from stdin instead.\n" +
			"\n" +
			"Both `--output default` and `--output json` print the same fields for the\n" +
			"newly created role (id, name, description, purpose, version, etc.).",
		Example: "  # Create a role from a JSON file\n" +
			"  pulumi org role new stack-reader ./reader.json \\\n" +
			"      --description \"Read-only stack access\"\n\n" +
			"  # Create a role from stdin and get the result as JSON\n" +
			"  cat reader.json | pulumi org role new stack-reader - --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			details, err := readRoleDetails(cmd.InOrStdin(), args[1])
			if err != nil {
				return err
			}
			return runOrgRoleNew(cmd.Context(), cmd.OutOrStdout(), factory, org, orgRoleNewArgs{
				Name:        args[0],
				Description: description,
				Purpose:     purpose,
				Details:     details,
			}, output.Get())
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "name"},
			{Name: "details-file"},
		},
		Required: 2,
	})

	cmd.Flags().StringVar(&org, "org", "",
		"The organization to create the role in. Defaults to the current default organization")
	cmd.Flags().StringVar(&description, "description", "", "A description for the role")
	cmd.Flags().StringVar(&purpose, "purpose", "",
		"The UX purpose for the role: organization, team, or token")
	outputflag.VarP(cmd.Flags(), &output)

	return cmd
}

type orgRoleNewArgs struct {
	Name        string
	Description string
	Purpose     string
	Details     json.RawMessage
}

// readRoleDetails reads a JSON document describing the role's permission tree
// from the given file path (or stdin when the path is "-").
func readRoleDetails(stdin io.Reader, path string) (json.RawMessage, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("reading role details from stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading role details from %q: %w", path, err)
		}
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("role details from %q is not valid JSON", path)
	}
	return json.RawMessage(data), nil
}

func runOrgRoleNew(
	ctx context.Context,
	w io.Writer,
	factory orgRoleClientFactory,
	orgFlag string,
	args orgRoleNewArgs,
	render roleSingleRender,
) error {
	c, orgName, err := factory(ctx, orgFlag)
	if err != nil {
		return err
	}

	created, err := c.CreateOrgRole(ctx, orgName, apitype.CreateRoleRequest{
		Name:        args.Name,
		Description: args.Description,
		UXPurpose:   args.Purpose,
		Details:     args.Details,
	})
	if err != nil {
		return fmt.Errorf("creating organization role: %w", err)
	}

	return render(w, orgName, "Created", created)
}
