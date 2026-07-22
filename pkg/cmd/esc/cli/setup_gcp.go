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
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	cloudsetup "github.com/pulumi/pulumi/pkg/v3/cloudsetup/common"
	"github.com/pulumi/pulumi/pkg/v3/cloudsetup/gcpsetup"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// The --policy presets. For GCP the official role name is itself the role granted to the service
// account, so each preset maps to itself and a custom role passes straight through untranslated.
var gcpPolicyChoices = []policyChoice{
	{name: "roles/editor", id: "roles/editor", alias: policyAliasAdmin, desc: policyAdminAccess},
	{name: "roles/viewer", id: "roles/viewer", alias: policyAliasReadonly, desc: policyReadonlyAccess},
}

var gcpResourceNames = map[string]string{
	gcpsetup.ResourceTypeGCPWorkloadIdentityPool:     "Workload Identity Pool",
	gcpsetup.ResourceTypeGCPWorkloadIdentityProvider: "Workload Identity Provider",
	gcpsetup.ResourceTypeGCPServiceAccount:           "Service Account",
	gcpsetup.ResourceTypeGCPIAMBinding:               "IAM Binding",
}

// gcpLoginPath is the property path under `values` where the login block is written,
// matching the default of `env provider gcp-login`.
const gcpLoginPath = "gcp.login"

// gcpServiceAccountName is the name of the service account created in each project.
const gcpServiceAccountName = "pulumi-oidc"

// gcpResource returns the ID of the first setup resource of the given type.
func gcpResource(result *cloudsetup.CloudSetupResult, resourceType string) (string, bool) {
	if result == nil {
		return "", false
	}
	for _, res := range result.Resources {
		if res.Type == resourceType && res.ID != "" {
			return res.ID, true
		}
	}
	return "", false
}

// selectGCPProjects resolves which projects to configure. A GCP project has no role to choose;
// the role is the same --policy for all.
func selectGCPProjects(
	esc *escCommand, projects []cloudsetup.CloudAccount, projectIDs []string, yes bool,
) ([]cloudsetup.CloudAccount, error) {
	if len(projects) == 0 {
		return nil, errors.New("no GCP projects are accessible with these credentials")
	}

	if len(projectIDs) > 0 {
		var chosen []cloudsetup.CloudAccount
		for _, id := range projectIDs {
			i := slices.IndexFunc(projects, func(p cloudsetup.CloudAccount) bool { return p.ID == id })
			if i < 0 {
				return nil, fmt.Errorf("project %s is not accessible with these credentials", id)
			}
			chosen = append(chosen, projects[i])
		}
		return chosen, nil
	}

	if len(projects) == 1 {
		return projects, nil
	}
	if yes {
		return nil, errors.New("multiple projects are accessible; pass --project-id to choose without prompting")
	}

	labels := make([]string, len(projects))
	for i, p := range projects {
		labels[i] = fmt.Sprintf("%s (%s)", p.Name, p.ID)
	}
	picked := ui.PromptUserMulti("Which GCP projects should be set up?", labels, nil, esc.colors)
	if len(picked) == 0 {
		return nil, errors.New("no projects selected")
	}
	var chosen []cloudsetup.CloudAccount
	for _, label := range picked {
		chosen = append(chosen, projects[slices.Index(labels, label)])
	}
	return chosen, nil
}

// createGCPEnvironment writes the `fn::open::gcp-login` OIDC block for one project, reusing the
// same helpers as `env provider gcp-login oidc`.
func createGCPEnvironment(
	ctx context.Context, setup *setupCommand, org, projectName string, r accountSetupResult,
) error {
	poolID, ok := gcpResource(r.result, gcpsetup.ResourceTypeGCPWorkloadIdentityPool)
	if !ok {
		return errors.New("workload identity pool missing from the setup result")
	}
	providerID, ok := gcpResource(r.result, gcpsetup.ResourceTypeGCPWorkloadIdentityProvider)
	if !ok {
		return errors.New("workload identity provider missing from the setup result")
	}
	serviceAccount, ok := gcpResource(r.result, gcpsetup.ResourceTypeGCPServiceAccount)
	if !ok {
		return errors.New("service account missing from the setup result")
	}

	path, err := resource.ParsePropertyPath(gcpLoginPath)
	if err != nil {
		return fmt.Errorf("invalid provider path %q: %w", gcpLoginPath, err)
	}

	envName := fmt.Sprintf("%s/%s/%s", org, projectName, sanitizeEnvName(r.account.Name, r.account.ID))
	ref := setup.env.parseRef(envName)
	fmt.Fprintf(setup.esc().stdout, "\nConfiguring environment %s for project %s:\n", ref.String(), r.account.ID)

	node := buildGCPLoginOIDCNode(r.account.Number, poolID, providerID, serviceAccount, "", "", nil)
	if err := ensureProviderEnv(ctx, setup.env, ref, true); err != nil {
		return err
	}
	return applyProviderUpdate(ctx, setup.env, ref, "", path, node, gcpLoginEnvVars(propertyPathRef(path)))
}

func newSetupGCPCmd(setup *setupCommand) *cobra.Command {
	var (
		projectIDs  []string
		policy      string
		orgName     string
		yes         bool
		projectName string
	)

	cmd := &cobra.Command{
		Use:   "gcp",
		Short: "Set up GCP OIDC integration for Pulumi ESC",
		Long: "[EXPERIMENTAL] Set up GCP OIDC integration for Pulumi ESC\n" +
			"\n" +
			"Creates, in each selected GCP project:\n" +
			"  - a workload identity pool and provider trusting Pulumi Cloud\n" +
			"  - a service account with the chosen role\n" +
			"  - the IAM bindings that let Pulumi Cloud impersonate it\n" +
			"\n" +
			"Authenticates with Google Application Default Credentials. Run\n" +
			"`gcloud auth application-default login` first if you have not already.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi env setup gcp --policy roles/editor\n" +
			"  pulumi env setup gcp --policy roles/viewer --project-id my-project --yes\n",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			esc := setup.esc()

			if err := esc.getCachedClient(ctx); err != nil {
				return err
			}

			oidcIssuer, err := setup.oidcIssuer()
			if err != nil {
				return err
			}
			org, err := setup.org(orgName)
			if err != nil {
				return err
			}

			role, err := setup.resolvePolicy(policy, gcpPolicyChoices, yes)
			if err != nil {
				return err
			}

			client, err := gcpsetup.NewClientFromADC(ctx, oidcIssuer)
			if err != nil {
				return fmt.Errorf(
					"no usable GCP credentials (%w); run `gcloud auth application-default login`", err)
			}

			projects, err := client.ListAccounts(ctx)
			if err != nil {
				return fmt.Errorf("listing GCP projects: %w", err)
			}
			selected, err := selectGCPProjects(esc, projects, projectIDs, yes)
			if err != nil {
				return err
			}

			fmt.Fprintf(esc.stdout, "\nAbout to configure OIDC for organization %s:\n", org)
			for _, p := range selected {
				fmt.Fprintf(esc.stdout, "  project %s (%s): grant %s\n", p.Name, p.ID, role)
			}
			if !yes {
				if ui.PromptUser("Proceed?", []string{"yes", "no"}, "no", esc.colors) != "yes" {
					return errors.New("cancelled")
				}
			}

			setup.printHeading("Setting up Infrastructure")
			results := make([]accountSetupResult, 0, len(selected))
			for _, project := range selected {
				fmt.Fprintf(esc.stdout, "\nSetting up project %s...\n", project.ID)
				// Both a result and an error can come back: the result records which resources
				// were created before the failure.
				result, err := client.SetupOIDCInfrastructure(ctx, org, project.ID, gcpServiceAccountName, role)
				results = append(results, accountSetupResult{account: project, result: result, err: err})
			}
			renderSetupResults(esc.stdout, results, gcpResourceNames)

			if !slices.ContainsFunc(results, accountSetupResult.succeeded) {
				return errors.New("failed to configure OIDC in any project")
			}

			setup.printHeading("Setting up Environment(s)")
			var envErr error
			for _, r := range results {
				if !r.succeeded() {
					continue
				}
				if err := createGCPEnvironment(ctx, setup, org, projectName, r); err != nil {
					fmt.Fprintf(esc.stderr, "  %s: %v\n", r.label(), err)
					envErr = err
				}
			}
			return envErr
		},
	}

	cmd.Flags().StringVar(&policy, "policy", "",
		"the role granted to the service account: roles/editor (required for Deployments), "+
			"roles/viewer (required for Insights), or any other role; prompted for when omitted")
	cmd.Flags().StringArrayVar(&projectIDs, "project-id", nil,
		"a GCP project to set up (repeatable; prompted for when omitted)")
	cmd.Flags().StringVar(&orgName, "org", "", "the Pulumi organization to configure OIDC for")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip all confirmation prompts")
	cmd.Flags().StringVar(&projectName, "project", "gcp-login",
		"the ESC project that per-project environments are created in")

	return cmd
}
