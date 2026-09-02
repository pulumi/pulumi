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

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

type setupCommand struct {
	// env is held rather than escCommand so that setup can reuse the provider
	// helpers (ensureProviderEnv, applyProviderUpdate) to write the login block.
	env *envCommand
}

func newEnvSetupCmd(env *envCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "[EXPERIMENTAL] Set up cloud provider OIDC integrations",
		Long: "[EXPERIMENTAL] Set up cloud provider OIDC integrations\n" +
			"\n" +
			"Creates the identity resources a cloud provider needs in order to trust Pulumi\n" +
			"Cloud as an OIDC identity provider, so that environments can obtain short-lived\n" +
			"credentials without any long-lived secrets.\n",
		Args: cobra.NoArgs,
	}

	setup := &setupCommand{env: env}

	cmd.AddCommand(newSetupAWSCmd(setup))
	cmd.AddCommand(newSetupAzureCmd(setup))
	cmd.AddCommand(newSetupGCPCmd(setup))

	return cmd
}

// esc returns the underlying esc command, for stdout/stderr and the API client.
func (s *setupCommand) esc() *escCommand {
	return s.env.esc
}

// printHeading writes a colorized section heading to stdout, preceded by a blank line.
func (s *setupCommand) printHeading(title string) {
	esc := s.esc()
	fmt.Fprintln(esc.stdout)
	fmt.Fprintln(esc.stdout, esc.colors.Colorize(colors.SpecHeadline+title+colors.Reset))
}

// oidcIssuer returns the OIDC issuer URL of the currently logged-in backend.
//
// The issuer is scheme://host/oidc, matching how pulumi-service derives it from its API domain.
func (s *setupCommand) oidcIssuer() (string, error) {
	backendURL := s.esc().account.BackendURL
	if backendURL == "" {
		return "", errors.New("could not determine the current backend; run `pulumi login`")
	}
	parsed, err := url.Parse(backendURL)
	if err != nil {
		return "", fmt.Errorf("parsing backend URL %q: %w", backendURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("backend URL %q is not an absolute URL", backendURL)
	}
	return parsed.Scheme + "://" + parsed.Host + "/oidc", nil
}

// org returns the Pulumi organization to configure OIDC for, preferring an explicit flag.
func (s *setupCommand) org(orgFlag string) (string, error) {
	if orgFlag != "" {
		return orgFlag, nil
	}
	if s.esc().account.DefaultOrg != "" {
		return s.esc().account.DefaultOrg, nil
	}
	return "", errors.New("could not determine the organization; pass --org or set a default organization")
}

// orgID returns the unique identifier of the Pulumi organization.
func (s *setupCommand) orgID(ctx context.Context, orgName string) (string, error) {
	id, err := s.esc().client.GetOrganizationID(ctx, orgName)
	if err != nil {
		return "", fmt.Errorf("looking up organization %s: %w", orgName, err)
	}
	return id, nil
}

const orgIDPrefixLen = 8

// envHash abbreviates an ESC environment name to fit the provider's name length limits.
func envHash(escEnvironmentName string, length int) string {
	sum := sha256.Sum256([]byte(escEnvironmentName))
	return hex.EncodeToString(sum[:])[:length]
}

func orgIDPrefix(orgID string) string {
	return orgID[:min(len(orgID), orgIDPrefixLen)]
}

// accountLabels sorts accounts by name in place and returns the chooser label for each.
func accountLabels(accounts []cloudsetup.CloudAccount) []string {
	slices.SortFunc(accounts, func(a, b cloudsetup.CloudAccount) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})

	labels := make([]string, len(accounts))
	for i, a := range accounts {
		labels[i] = fmt.Sprintf("%s (%s)", a.Name, a.ID)
	}
	return labels
}

// printSetupTarget writes the header line for one cloud account in the confirmation summary.
func printSetupTarget(esc *escCommand, heading string) {
	fmt.Fprintf(esc.stdout, "  %s\n", esc.colors.Colorize(colors.SpecSubHeadline+heading+colors.Reset))
}

// planEnvLine describes what setup will do to the ESC environment, either create or update.
func (s *setupCommand) planEnvLine(ctx context.Context, ref environmentRef, loginPath string) string {
	exists, err := s.env.esc.client.EnvironmentExists(ctx, ref.orgName, ref.projectName, ref.envName)
	switch {
	case err != nil && !client.IsNotFound(err):
		// The check is best-effort, so do not fail setup over it; just do not promise a create.
		return "create or update ESC environment " + ref.String()
	case exists:
		return fmt.Sprintf("update ESC environment %s (exists; its `%s` block will be replaced)",
			ref.String(), loginPath)
	default:
		return "create ESC environment " + ref.String()
	}
}

// escNameChars is the character set for ESC project and environment names.
const escNameChars = `a-zA-Z0-9._-`

var (
	escNameRE     = regexp.MustCompile(`^[` + escNameChars + `]+$`)
	envNameUnsafe = regexp.MustCompile(`[^` + escNameChars + `]`)
)

// validateESCProject checks that an ESC project name is valid and returns it lowercased.
func validateESCProject(projectName string) (string, error) {
	if !escNameRE.MatchString(projectName) {
		return "", fmt.Errorf("--project %q must contain only letters, digits, and the characters . _ -",
			projectName)
	}
	return strings.ToLower(projectName), nil
}

func validateESCEnvName(envName string) (string, error) {
	if envName == "" {
		return "", nil
	}
	if !escNameRE.MatchString(envName) {
		return "", fmt.Errorf("--env-name %q must contain only letters, digits, and the characters . _ -",
			envName)
	}
	return strings.ToLower(envName), nil
}

// sanitizeEnvName derives a default environment name from a cloud account name,
// matching the naming the Pulumi Cloud console uses.
func sanitizeEnvName(accountName, accountID string) string {
	base := accountName
	if base == "" {
		base = accountID
	}
	return strings.ToLower(envNameUnsafe.ReplaceAllString(base, "-")) + "-env"
}

type envNaming struct {
	project string
	envName string
}

func (n envNaming) escEnvName(account cloudsetup.CloudAccount) string {
	name := n.envName
	if name == "" {
		name = sanitizeEnvName(account.Name, account.ID)
	}
	return n.project + "/" + name
}

func checkDuplicateEnvNames(naming envNaming, accounts []cloudsetup.CloudAccount) error {
	if naming.envName != "" && len(accounts) > 1 {
		return fmt.Errorf(
			"--env-name names a single environment, but %d accounts were selected; "+
				"omit it to derive a name from each account", len(accounts))
	}
	seen := map[string]cloudsetup.CloudAccount{}
	for _, a := range accounts {
		name := naming.escEnvName(a)
		if prev, ok := seen[name]; ok {
			return fmt.Errorf(
				"%q (%s) and %q (%s) have the same name. This would result in the same ESC environment name '%s'",
				prev.Name, prev.ID, a.Name, a.ID, name)
		}
		seen[name] = a
	}
	return nil
}

// oidcSubjectAttributes is written into every generated login block so the environment presents
// the per-environment subject that the setup call scopes its cloud trust to.
var oidcSubjectAttributes = []string{"currentEnvironment.name"}

// policyChoice is one of the presets offered for --policy.
type policyChoice struct {
	// name is the official cloud name, e.g. AWS "AdministratorAccess".
	name string
	// policy id from the provider, e.g. AWS policy ARN, Azure role definition ID, GCP role name.
	id string
	// desc describes what the policy grants on this provider, shown after name in the prompt.
	// The presets differ between clouds, so each provider spells its own out.
	desc string
}

// label returns the prompt line for this choice.
func (c policyChoice) label() string {
	return c.name + " - " + c.desc
}

// resolvePolicy resolves --policy to a provider-native id, prompting when it was omitted.
func (s *setupCommand) resolvePolicy(policy string, choices []policyChoice, yes bool) (string, error) {
	for _, c := range choices {
		if strings.EqualFold(policy, c.name) {
			return c.id, nil
		}
	}
	if policy != "" {
		return policy, nil
	}
	if len(choices) == 0 {
		return "", errors.New("no policy choices were offered")
	}

	if yes {
		names := make([]string, len(choices))
		for i, c := range choices {
			names[i] = c.name
		}
		return "", fmt.Errorf("--policy must be set when using --yes; pass one of %s", strings.Join(names, ", "))
	}

	labels := make([]string, len(choices))
	for i, c := range choices {
		labels[i] = c.label()
	}
	selected := ui.PromptUser("What level of access should the OIDC identity have?",
		labels, labels[0], s.esc().colors)
	for i, l := range labels {
		if l == selected {
			return choices[i].id, nil
		}
	}
	return "", errors.New("no policy selected")
}

// accountSetupResult pairs a cloud account with the setup result.
type accountSetupResult struct {
	account cloudsetup.CloudAccount
	result  *cloudsetup.CloudSetupResult
	err     error
}

// succeeded reports whether every resource for this account was created or already existed.
func (r accountSetupResult) succeeded() bool {
	return r.err == nil && r.result != nil && r.result.Success
}

// label returns a human-readable identifier for the account.
func (r accountSetupResult) label() string {
	switch {
	case r.account.Name != "" && r.account.ID != "":
		return fmt.Sprintf("%s (%s)", r.account.Name, r.account.ID)
	case r.account.Name != "":
		return r.account.Name
	default:
		return r.account.ID
	}
}

// renderSetupResults writes a per-account summary of what was created, existed, or failed.
// resourceNames maps provider-specific resource types to display names.
func renderSetupResults(w io.Writer, results []accountSetupResult, resourceNames map[string]string) {
	for _, r := range results {
		fmt.Fprintln(w)

		if r.result == nil {
			fmt.Fprintf(w, "%s: failed: %v\n", r.label(), r.err)
			continue
		}

		status := "done"
		if !r.result.Success {
			status = "incomplete"
		}
		fmt.Fprintf(w, "%s: %s\n", r.label(), status)

		for _, res := range r.result.Resources {
			name, ok := resourceNames[res.Type]
			if !ok {
				name = res.Type
			}
			fmt.Fprintf(w, "  %-32s %s", name, res.Status)
			switch {
			case res.Error != "":
				fmt.Fprintf(w, ": %s", res.Error)
			case res.ID != "":
				fmt.Fprintf(w, "  %s", res.ID)
			}
			fmt.Fprintln(w)
		}

		if r.result.Message != "" {
			fmt.Fprintf(w, "  %s\n", r.result.Message)
		}
		if r.err != nil {
			fmt.Fprintf(w, "  %v\n", r.err)
		}
	}
}
