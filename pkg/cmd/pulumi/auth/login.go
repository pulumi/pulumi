// Copyright 2016-2026, Pulumi Corporation.
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

package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"

	survey "github.com/AlecAivazis/survey/v2"
	surveycore "github.com/AlecAivazis/survey/v2/core"
	pkgBackend "github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func NewLoginCmd(ws pkgWorkspace.Context, lm backend.LoginManager) *cobra.Command {
	var cloudURL string
	var defaultOrg string
	var localMode bool
	var insecure bool
	var interactive bool

	var oidcToken string
	var oidcOrg string
	var oidcTeam string
	var oidcUser string
	var oidcExpiration string

	cmd := &cobra.Command{
		Use:   "login [<url>]",
		Short: "Log in to the Pulumi Cloud",
		Long: "Log in to the Pulumi Cloud.\n" +
			"\n" +
			"The Pulumi Cloud manages your stack's state reliably. Simply run\n" +
			"\n" +
			"    $ pulumi login\n" +
			"\n" +
			"and this command will prompt you for an access token, including a way to launch your web browser to\n" +
			"easily obtain one. You can script by using `PULUMI_ACCESS_TOKEN` environment variable.\n" +
			"\n" +
			"By default, this will log in to the managed Pulumi Cloud backend.\n" +
			"If you prefer to log in to a self-hosted Pulumi Cloud backend, specify a URL. For example, run\n" +
			"\n" +
			"    $ pulumi login https://api.pulumi.acmecorp.com\n" +
			"\n" +
			"to log in to a self-hosted Pulumi Cloud running at the api.pulumi.acmecorp.com domain.\n" +
			"\n" +
			"For `https://` URLs, the CLI will speak REST to a Pulumi Cloud that manages state and concurrency control.\n" +
			"You can specify a default org to use when logging into the Pulumi Cloud backend or a " +
			"self-hosted Pulumi Cloud.\n" +
			"\n" +
			"If you prefer to operate Pulumi independently of a Pulumi Cloud, and entirely local to your computer,\n" +
			"pass `file://<path>`, where `<path>` will be where state checkpoints will be stored. For instance,\n" +
			"\n" +
			"    $ pulumi login file://~\n" +
			"\n" +
			"will store your state information on your computer underneath `~/.pulumi`. It is then up to you to\n" +
			"manage this state, including backing it up, using it in a team environment, and so on.\n" +
			"\n" +
			"As a shortcut, you may pass --local to use your home directory (this is an alias for `file://~`):\n" +
			"\n" +
			"    $ pulumi login --local\n" +
			"\n" +
			"Additionally, you may leverage supported object storage backends from one of the cloud providers " +
			"to manage the state independent of the Pulumi Cloud. For instance,\n" +
			"\n" +
			"AWS S3:\n" +
			"\n" +
			"    $ pulumi login s3://my-pulumi-state-bucket\n" +
			"\n" +
			"GCP GCS:\n" +
			"\n" +
			"    $ pulumi login gs://my-pulumi-state-bucket\n" +
			"\n" +
			"Azure Blob:\n" +
			"\n" +
			"    $ pulumi login azblob://my-pulumi-state-bucket\n" +
			"\n" +
			"PostgreSQL:\n" +
			"\n" +
			"    $ pulumi login postgres://username:password@hostname:5432/database\n",
		Args: cmdutil.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			displayOptions := display.Options{
				Color: cmdutil.GetGlobalColorization(),
			}

			// If a <cloud> was specified as an argument, use it.
			if len(args) > 0 {
				if cloudURL != "" {
					return errors.New("only one of --cloud-url or argument URL may be specified, not both")
				}
				cloudURL = args[0]
			}

			// For local mode, store state by default in the user's home directory.
			if localMode {
				if cloudURL != "" {
					return errors.New("a URL may not be specified when --local mode is enabled")
				}
				cloudURL = diy.FilePathPrefix + "~"
			}

			// If we're on Windows, and this is a local login path, then allow the user to provide
			// backslashes as path separators.  We will normalize them here to forward slashes as that's
			// what the gocloud blob system requires.
			if strings.HasPrefix(cloudURL, diy.FilePathPrefix) && os.PathSeparator != '/' {
				cloudURL = filepath.ToSlash(cloudURL)
			}

			// Try to read the current project
			project, _, err := ws.ReadProject()
			if err != nil && !errors.Is(err, workspace.ErrProjectNotFound) {
				return err
			}

			isInteractive := cmdutil.Interactive() && interactive
			if cloudURL == "" && isInteractive {
				creds, err := pkgWorkspace.Instance.GetStoredCredentials()
				if err != nil {
					return err
				}
				// If there are no accounts, skip this step and continue on with the default behavior below.
				if len(creds.Accounts) > 0 {
					act, err := chooseAccount(creds.Accounts, displayOptions)
					if err != nil {
						return err
					}
					cloudURL = act
				}
			}
			if cloudURL == "" {
				var err error
				cloudURL, err = pkgWorkspace.GetCurrentCloudURL(ws, env.Global(), project)
				if err != nil {
					return fmt.Errorf("could not determine current cloud: %w", err)
				}
				// If still no URL and using OIDC token, default to Pulumi Cloud
				if cloudURL == "" && oidcToken != "" {
					cloudURL = "https://api.pulumi.com"
				}
			} else if url := strings.TrimPrefix(strings.TrimPrefix(
				cloudURL, "https://"), "http://"); strings.HasPrefix(url, "app.pulumi.com/") ||
				strings.HasPrefix(url, "pulumi.com") {
				return fmt.Errorf("%s is not a valid self-hosted backend, "+
					"use `pulumi login` without arguments to log into the Pulumi Cloud backend", cloudURL)
			} else {
				// Ensure we have the correct cloudurl type before logging in
				if err := validateCloudBackendType(cloudURL); err != nil {
					return err
				}
			}

			if diy.IsDIYBackendURL(cloudURL) {
				if oidcToken != "" || oidcOrg != "" || oidcTeam != "" || oidcUser != "" || oidcExpiration != "" {
					return errors.New("oidc-token, oidc-org, oidc-team, oidc-user, " +
						"and oidc-expiration flags are not supported for this type of backend")
				}
			}

			var be pkgBackend.Backend
			if oidcToken != "" {
				// Extract defaults from JWT token before creating auth context
				resolvedOrg, resolvedTeam, resolvedUser, innerErr := extractOIDCDefaults(
					oidcOrg, oidcTeam, oidcUser, oidcToken)
				if innerErr != nil {
					return fmt.Errorf("problem logging in: %w", innerErr)
				}

				authContext, innerErr := workspace.NewAuthContextForTokenExchange(
					resolvedOrg, resolvedTeam, resolvedUser, oidcToken, oidcExpiration)
				if innerErr != nil {
					return fmt.Errorf("problem logging in: %w", innerErr)
				}
				be, err = lm.LoginFromAuthContext(
					ctx, cmdutil.Diag(), cloudURL, project, true /* setCurrent */, insecure, authContext)
			} else {
				be, err = lm.Login(
					ctx, ws, cmdutil.Diag(), cloudURL, project, true /* setCurrent */, insecure, displayOptions.Color)
			}

			if err != nil {
				return fmt.Errorf("problem logging in: %w", err)
			}

			if diy.IsDIYBackendURL(cloudURL) {
				if defaultOrg != "" {
					return errors.New("unable to set default org for this type of backend")
				}
			} else {
				// if the user has specified a default org to associate with the backend
				if defaultOrg != "" {
					if err := workspace.SetBackendConfigDefaultOrg(be.URL(), defaultOrg); err != nil {
						return err
					}
				}
			}

			if currentUser, _, _, err := be.CurrentUser(); err == nil {
				// TODO should we print the token information here? (via team MyTeam token MyToken)
				fmt.Printf("Logged in to %s as %s (%s)\n", be.Name(), currentUser, be.URL())
			} else {
				fmt.Printf("Logged in to %s (%s)\n", be.Name(), be.URL())
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&cloudURL, "cloud-url", "c", "", "A cloud URL to log in to")
	cmd.PersistentFlags().StringVar(&defaultOrg, "default-org", "", "A default org to associate with the login. "+
		"Please note, currently, only the managed and self-hosted backends support organizations")
	cmd.PersistentFlags().BoolVarP(&localMode, "local", "l", false, "Use Pulumi in local-only mode")
	cmd.PersistentFlags().BoolVar(&insecure, "insecure", false, "Allow insecure server connections when using SSL")
	cmd.PersistentFlags().BoolVar(&interactive, "interactive", false,
		"Show interactive login options based on known accounts")
	cmd.PersistentFlags().StringVar(&oidcToken, "oidc-token", "",
		"An OIDC token to exchange for a cloud backend access token. Can be either a raw token or a file path "+
			"prefixed with 'file://'.")
	cmd.PersistentFlags().StringVar(&oidcOrg, "oidc-org", "", "The organization to use for OIDC token exchange audience")
	cmd.PersistentFlags().StringVar(&oidcTeam, "oidc-team", "", "The team when exchanging for a team token")
	cmd.PersistentFlags().StringVar(&oidcUser, "oidc-user", "", "The user when exchanging for a personal token")
	cmd.PersistentFlags().StringVar(
		&oidcExpiration, "oidc-expiration", "",
		"The expiration for the cloud backend access token in duration format (e.g. '15m', '24h')")

	return cmd
}

func validateCloudBackendType(typ string) error {
	kind := strings.SplitN(typ, ":", 2)[0]
	supportedKinds := []string{"azblob", "gs", "s3", "file", "https", "http", "postgres"}
	for _, supportedKind := range supportedKinds {
		if kind == supportedKind {
			return nil
		}
	}
	return fmt.Errorf("unknown backend cloudUrl format '%s' (supported Url formats are: "+
		"azblob://, gs://, s3://, file://, https://, http:// and postgres://)",
		kind)
}

// chooseAccount will prompt the user to choose amongst the available accounts.
func chooseAccount(accounts map[string]workspace.Account, opts display.Options) (string, error) {
	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true

	acts := make([]string, 0, len(accounts))
	for url := range accounts {
		acts = append(acts, url)
	}

	sort.Strings(acts)

	nopts := len(acts)
	pageSize := cmd.OptimalPageSize(cmd.OptimalPageSizeOpts{Nopts: nopts})
	message := fmt.Sprintf("\rPlease choose an account (%d total):\n", nopts)
	message = opts.Color.Colorize(colors.SpecPrompt + message + colors.Reset)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message:  message,
		Options:  acts,
		PageSize: pageSize,
	}, &option, ui.SurveyIcons(opts.Color)); err != nil {
		return "", errors.New("no account selected; please use `pulumi login --interactive` to choose one")
	}

	return option, nil
}

// extractOIDCDefaults extracts organization, team, and user defaults from a JWT token.
// Returns the resolved org, team, user values after applying JWT defaults to CLI flags.
// Security Note: ParseUnverified is safe here because:
// 1. We only extract metadata (org/team/user) to validate against explicit flags
// 2. The full JWT is sent to Pulumi Cloud for proper signature verification
// 3. We never use these claims for authentication - only for validation and user convenience
func extractOIDCDefaults(organization, team, user, token string) (string, string, string, error) {
	if token == "" {
		return organization, team, user, nil
	}

	// Parse JWT claims to extract organization and scope
	var jwtOrg, jwtTeam, jwtUser string
	parser := jwt.NewParser()
	claims := jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(token, claims)
	if err == nil {
		// Extract organization from aud claim
		// Format: urn:pulumi:org:<org-name>
		if aud, ok := claims["aud"].(string); ok {
			if strings.HasPrefix(aud, "urn:pulumi:org:") {
				parts := strings.Split(aud, ":")
				if len(parts) == 4 {
					jwtOrg = parts[3]
				}
			}
		}

		// Extract team, user, or runner from scope claim
		// Scope can be a string or array of strings
		// Formats: "team:<team-name>", "user:<user-login>", "runner:<runner-name>"
		if scopeClaim, ok := claims["scope"]; ok {
			var scopes []string
			switch v := scopeClaim.(type) {
			case string:
				// Single scope as string, may be space-separated
				scopes = strings.Fields(v)
			case []any:
				// Array of scopes
				for _, s := range v {
					if str, ok := s.(string); ok {
						scopes = append(scopes, str)
					}
				}
			}
			for _, scope := range scopes {
				if strings.HasPrefix(scope, "team:") {
					jwtTeam = strings.TrimPrefix(scope, "team:")
				} else if strings.HasPrefix(scope, "user:") {
					jwtUser = strings.TrimPrefix(scope, "user:")
				}
				// Note: runner scope not yet implemented in CLI
			}
		}
	}

	// Validate that JWT doesn't contain both team and user in scope
	// This is not supported by the backend - team and user are mutually exclusive
	if jwtTeam != "" && jwtUser != "" {
		return "", "", "", fmt.Errorf(
			"JWT scope contains both team '%s' and user '%s'. "+
				"Only one of team or user may be specified for token exchange",
			jwtTeam, jwtUser)
	}

	// Validate that explicit flags don't conflict with JWT claims
	if organization != "" && jwtOrg != "" && organization != jwtOrg {
		return "", "", "", fmt.Errorf(
			"--oidc-org '%s' conflicts with JWT aud claim organization '%s'. "+
				"The JWT aud claim takes precedence during token exchange. "+
				"Either omit --oidc-org to use the JWT value, or ensure they match",
			organization, jwtOrg)
	}
	if team != "" && jwtTeam != "" && team != jwtTeam {
		return "", "", "", fmt.Errorf(
			"--oidc-team '%s' conflicts with JWT scope team '%s'. "+
				"The JWT scope takes precedence during token exchange. "+
				"Either omit --oidc-team to use the JWT value, or ensure they match",
			team, jwtTeam)
	}
	if user != "" && jwtUser != "" && user != jwtUser {
		return "", "", "", fmt.Errorf(
			"--oidc-user '%s' conflicts with JWT scope user '%s'. "+
				"The JWT scope takes precedence during token exchange. "+
				"Either omit --oidc-user to use the JWT value, or ensure they match",
			user, jwtUser)
	}

	// Use JWT values if explicit flags not provided
	if organization == "" && jwtOrg != "" {
		organization = jwtOrg
	}
	if team == "" && jwtTeam != "" {
		team = jwtTeam
	}
	if user == "" && jwtUser != "" {
		user = jwtUser
	}

	return organization, team, user, nil
}
