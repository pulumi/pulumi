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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/state"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
)

func NewWhoAmICmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var verbose bool

	output := outputflag.OutputFlag[whoAmIRenderFunc]{
		RenderForTerminal: func(
			w io.Writer, b backend.Backend, name string, orgs []string, tokenInfo *workspace.TokenInformation,
			credentials func() credentialSource,
		) error {
			if err := renderWhoAmIText(w, b, name, orgs, tokenInfo, verbose); err != nil {
				return err
			}
			if verbose {
				source := credentials()
				if source.helperPath != "" {
					fmt.Fprintf(w, "Credential helper: %s\n", source.helperPath)
				}
				fmt.Fprintf(w, "Access token source: %s\n", source.accessToken)
			}
			return nil
		},
		RenderJSON: func(
			w io.Writer, b backend.Backend, name string, orgs []string, tokenInfo *workspace.TokenInformation,
			credentials func() credentialSource,
		) error {
			source := credentials()
			return ui.FprintJSON(w, whoAmIJSON{
				User:              name,
				Organizations:     orgs,
				URL:               b.URL(),
				TokenInformation:  tokenInfo,
				CredentialHelper:  source.helperPath,
				AccessTokenSource: source.accessToken,
			})
		},
	}

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current logged-in user",
		Long: "Display the current logged-in user\n" +
			"\n" +
			"Displays the username of the currently logged in user.\n" +
			"\n" +
			"When the current token is a Pulumi Cloud team token or an organization token, " +
			"the command will return the name of the organization with which the token is associated.",

		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			stdout := cmd.OutOrStdout()

			opts := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current working directory: %w", err)
			}

			// Try to read the current project
			project, _, err := ws.ReadProject(cwd)
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			b, err := cmdBackend.CurrentBackend(ctx, ws, lm, project, opts)
			if err != nil {
				return err
			}

			name, orgs, tokenInfo, err := b.CurrentUser()
			if err != nil {
				return err
			}

			credentials := func() credentialSource {
				session := lm.Session()
				source := credentialSource{accessToken: accessTokenSource(session, state.BackendURLKey(b))}
				if helper, err := session.Helper(); err == nil && helper != nil {
					source.helperPath = helper.Path
				}
				return source
			}
			return output.Get()(stdout, b, name, orgs, tokenInfo, credentials)
		},
	}

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
	credentials func() credentialSource,
) error

// credentialSource describes where the backend's credentials come from, without exposing them.
type credentialSource struct {
	// helperPath is the credential helper in use, if any.
	helperPath  string
	accessToken string
}

// accessTokenSource names where the backend's access token comes from, without exposing its value.
func accessTokenSource(session *auth.Session, backendURL string) string {
	switch {
	case !auth.IsHTTPBackend(backendURL):
		return "none"
	case env.AccessToken.Value() != "":
		return "PULUMI_ACCESS_TOKEN"
	case session.HTTPAuth(backendURL).AccessToken() != "":
		return "credential helper"
	default:
		return "account credentials"
	}
}

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
	// CredentialHelper is the path of the credential helper in use, if any.
	CredentialHelper  string `json:"credentialHelper,omitempty"`
	AccessTokenSource string `json:"accessTokenSource,omitempty"`
}
