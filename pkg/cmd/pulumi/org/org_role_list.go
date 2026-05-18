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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// roleListRender renders a list of roles.
type roleListRender func(w io.Writer, orgName string, roles []apitype.Role) error

func newOrgRoleListCmd() *cobra.Command {
	return newOrgRoleListCmdWith(defaultOrgRoleClientFactory)
}

func newOrgRoleListCmdWith(factory orgRoleClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "factory must not be nil")

	var (
		org     string
		purpose string
	)
	output := outputflag.OutputFlag[roleListRender]{
		RenderForTerminal: renderRoleListTable,
		RenderJSON:        renderRoleListJSON,
	}

	cmd := &cobra.Command{
		Hidden: true,
		Use:    "list",
		Short:  "List custom roles for an organization",
		Long: "[EXPERIMENTAL] List custom roles for an organization.\n" +
			"\n" +
			"Displays the ID, name, description, UX purpose, and version of each role.\n" +
			"By default the output is a human-readable table; pass --output=json for a\n" +
			"stable, machine-readable JSON envelope containing the same fields.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOrgRoleList(cmd.Context(), cmd.OutOrStdout(), factory, org, purpose, output.Get())
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVar(&org, "org", "",
		"The organization to list roles for. Defaults to the current default organization")
	cmd.Flags().StringVar(&purpose, "purpose", "role",
		"Filter by UX purpose: role, role_private, role_temporary, policy, or set")
	outputflag.Var(cmd.Flags(), &output)

	return cmd
}

func runOrgRoleList(
	ctx context.Context,
	w io.Writer,
	factory orgRoleClientFactory,
	orgFlag, purpose string,
	render roleListRender,
) error {
	c, orgName, err := factory(ctx, orgFlag)
	if err != nil {
		return err
	}

	roles, err := c.ListOrgRoles(ctx, orgName, purpose)
	if err != nil {
		return fmt.Errorf("listing organization roles: %w", err)
	}

	return render(w, orgName, roles)
}

// roleListEnvelope is the JSON shape emitted by `pulumi org role list --output=json`.
type roleListEnvelope struct {
	Organization string     `json:"organization"`
	Roles        []roleJSON `json:"roles"`
	Count        int        `json:"count"`
}

type roleJSON struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Purpose      string `json:"purpose"`
	Version      int    `json:"version"`
	IsOrgDefault bool   `json:"isOrgDefault"`
	Created      string `json:"created"`
	Modified     string `json:"modified"`
}

func toRoleJSON(r apitype.Role) roleJSON {
	return roleJSON{
		ID:           r.ID,
		Name:         r.Name,
		Description:  r.Description,
		Purpose:      r.UXPurpose,
		Version:      r.Version,
		IsOrgDefault: r.IsOrgDefault,
		Created:      r.Created,
		Modified:     r.Modified,
	}
}

func renderRoleListJSON(w io.Writer, orgName string, roles []apitype.Role) error {
	items := make([]roleJSON, 0, len(roles))
	for _, r := range roles {
		items = append(items, toRoleJSON(r))
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(roleListEnvelope{
		Organization: orgName,
		Roles:        items,
		Count:        len(items),
	})
}

func renderRoleListTable(w io.Writer, orgName string, roles []apitype.Role) error {
	if len(roles) == 0 {
		fmt.Fprintf(w, "No custom roles found for organization %q.\n", orgName)
		return nil
	}

	hasDescription, hasPurpose, hasDefault := false, false, false
	for _, r := range roles {
		hasDescription = hasDescription || r.Description != ""
		hasPurpose = hasPurpose || r.UXPurpose != ""
		hasDefault = hasDefault || r.IsOrgDefault
	}

	header := table.Row{"ID", "NAME"}
	if hasDescription {
		header = append(header, "DESCRIPTION")
	}
	if hasPurpose {
		header = append(header, "PURPOSE")
	}
	header = append(header, "VERSION")
	if hasDefault {
		header = append(header, "DEFAULT")
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(header)

	for _, r := range roles {
		row := table.Row{r.ID, r.Name}
		if hasDescription {
			row = append(row, r.Description)
		}
		if hasPurpose {
			row = append(row, r.UXPurpose)
		}
		row = append(row, r.Version)
		if hasDefault {
			defaultMark := ""
			if r.IsOrgDefault {
				defaultMark = "yes"
			}
			row = append(row, defaultMark)
		}
		t.AppendRow(row)
	}
	t.Render()

	fmt.Fprintf(w, "\n%d role(s)\n", len(roles))
	return nil
}
