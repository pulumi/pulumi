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

package whoami

import (
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/needle"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func NewWhoAmICmd(ctx needle.Context) *cobra.Command {
	var verbose bool
	var b backend.Backend

	output := outputflag.OutputFlag[whoAmIRenderFunc]{
		RenderForTerminal: func(
			w io.Writer, b backend.Backend, name string, orgs []string, tokenInfo *workspace.TokenInformation,
		) error {
			return renderWhoAmIText(w, b, name, orgs, tokenInfo, verbose)
		},
		RenderJSON: func(
			w io.Writer, b backend.Backend, name string, orgs []string, tokenInfo *workspace.TokenInformation,
		) error {
			return ui.FprintJSON(w, whoAmIJSON{
				User:             name,
				Organizations:    orgs,
				URL:              b.URL(),
				TokenInformation: tokenInfo,
			})
		},
	}

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current logged-in user",
		Long: "Display the currenqt logged-in user\n" +
			"\n" +
			"Displays the username of the currently logged in user.\n" +
			"\n" +
			"When the current token is a Pulumi Cloud team token or an organization token, " +
			"the command will return the name of the organization with which the token is associated.",

		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()

			name, orgs, tokenInfo, err := b.CurrentUser()
			if err != nil {
				return err
			}

			return output.Get()(stdout, b, name, orgs, tokenInfo)
		},
	}

	needle.Inject(cmd, ctx, needle.NeedBackend(&b))

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	outputflag.VarWithJSONAlias(cmd, cmd.PersistentFlags(), &output)

	cmd.PersistentFlags().BoolVarP(
		&verbose, "verbose", "v", false,
		"Print detailed whoami information",
	)

	return cmd
}

type whoAmIRenderFunc func(
	w io.Writer, b backend.Backend, name string, orgs []string, tokenInfo *workspace.TokenInformation,
) error

func renderWhoAmIText(
	w io.Writer, b backend.Backend, name string, orgs []string,
	tokenInfo *workspace.TokenInformation, verbose bool,
) error {
	if !verbose {
		fmt.Fprintf(w, "%s\n", name)
		return nil
	}

	fmt.Fprintf(w, "User: %s\n", name)
	fmt.Fprintf(w, "Organizations: %s\n", strings.Join(orgs, ", "))
	fmt.Fprintf(w, "Backend URL: %s\n", b.URL())
	if tokenInfo == nil {
		fmt.Fprintf(w, "Token type: personal\n")
		return nil
	}
	tokenType := "unknown"
	if tokenInfo.Team != "" {
		tokenType = "team: " + tokenInfo.Team
	} else if tokenInfo.Organization != "" {
		tokenType = "organization: " + tokenInfo.Organization
	}
	fmt.Fprintf(w, "Token type: %s\n", tokenType)
	fmt.Fprintf(w, "Token name: %s\n", tokenInfo.Name)
	return nil
}

// whoAmIJSON is the shape of the --json output of this command.
type whoAmIJSON struct {
	User             string                      `json:"user"`
	Organizations    []string                    `json:"organizations,omitempty"`
	URL              string                      `json:"url"`
	TokenInformation *workspace.TokenInformation `json:"tokenInformation,omitempty"`
}
