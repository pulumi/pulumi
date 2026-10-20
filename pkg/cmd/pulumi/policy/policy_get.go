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

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	cloudapitype "github.com/pulumi/pulumi-cloud-sdk/go/apitype"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type policyGetClient interface {
	ListPolicyPacks(
		ctx context.Context, orgName string, inContToken *string,
	) (cloudapitype.AppListPolicyPacksResponse, *string, error)
	GetPolicyPack(
		ctx context.Context, orgName, name, versionTag string,
	) (cloudapitype.AppGetPolicyPackResponse, error)
	ListPolicyGroups(
		ctx context.Context, orgName string, inContToken *string,
	) (cloudapitype.AppListPolicyGroupsResponse, *string, error)
	GetPolicyGroup(ctx context.Context, orgName, policyGroup string) (cloudapitype.PolicyGroup, error)
}

// policyGetClientFactory receives the organization parsed from the argument
// (possibly empty) and returns the client along with the resolved organization.
type policyGetClientFactory func(ctx context.Context, org string) (policyGetClient, string, error)

type policyGetRenderFunc func(w io.Writer, resp policyGetJSON) error

func newPolicyGetCmd() *cobra.Command {
	return newPolicyGetCmdWith(defaultPolicyGetClientFactory)
}

func newPolicyGetCmdWith(factory policyGetClientFactory) *cobra.Command {
	contract.Assertf(factory != nil, "policyGetClientFactory must not be nil")
	outputFormat := outputflag.OutputFlag[policyGetRenderFunc]{
		RenderForTerminal: renderPolicyGetText,
		RenderJSON:        renderPolicyGetJSON,
	}

	cmd := &cobra.Command{
		Use:   "get <org-name>/<policy-pack-name>[@<version>]",
		Short: "Get the details of a Policy Pack",
		Long: "Get the details of a Policy Pack.\n" +
			"\n" +
			"Shows the policies contained in a published version of a Policy Pack and the\n" +
			"Policy Groups it is enabled in. The organization defaults to your default\n" +
			"organization, and the version defaults to the latest published version.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi policy get acme/aws-guardrails@1.2.0\n" +
			"  pulumi policy get aws-guardrails --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyGet(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], outputFormat)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack", Usage: "[<org-name>/]<policy-pack-name>[@<version>]"},
		},
		Required: 1,
	})

	outputflag.VarP(cmd.Flags(), &outputFormat)
	return cmd
}

func defaultPolicyGetClientFactory(ctx context.Context, orgArg string) (policyGetClient, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return nil, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return nil, "", errors.New(
			"getting a Policy Pack requires the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return nil, "", err
	}

	org := orgArg
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return nil, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}

	if !slices.Contains(orgs, org) && org != userName {
		return nil, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return cloudBackend.Client(), org, nil
}

// parsePolicyPackVersionRef splits "[org/]name[@version]". A version of
// "latest" is returned as the empty string.
func parsePolicyPackVersionRef(s string) (org, name, version string, err error) {
	ref := s
	if before, after, found := strings.Cut(s, "@"); found {
		ref = before
		version = after
		if version == "" {
			return "", "", "", fmt.Errorf("invalid policy pack %q: version after '@' must not be empty", s)
		}
		if version == "latest" {
			version = ""
		}
	}

	parts := strings.Split(ref, "/")
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		org, name = parts[0], parts[1]
		if org == "" {
			return "", "", "", fmt.Errorf("invalid policy pack %q: organization must not be empty", s)
		}
	default:
		return "", "", "", fmt.Errorf("invalid policy pack %q: expected <org-name>/<policy-pack-name>[@<version>]", s)
	}
	if name == "" {
		return "", "", "", fmt.Errorf("invalid policy pack %q: policy pack name must not be empty", s)
	}
	return org, name, version, nil
}

// resolveLatestVersionTag returns the version tag of the highest-numbered
// version of the named pack, falling back to the version number when that
// version has no tag.
func resolveLatestVersionTag(
	packs []cloudapitype.AppPolicyPackWithVersions, org, name string,
) (string, error) {
	for _, pack := range packs {
		if pack.Name != name || len(pack.Versions) == 0 {
			continue
		}
		latest := 0
		for i, v := range pack.Versions {
			if v > pack.Versions[latest] {
				latest = i
			}
		}
		if latest < len(pack.VersionTags) && pack.VersionTags[latest] != "" {
			return pack.VersionTags[latest], nil
		}
		return strconv.Itoa(pack.Versions[latest]), nil
	}
	return "", fmt.Errorf("policy pack %q not found in organization %q", name, org)
}

func runPolicyGet(
	ctx context.Context, w io.Writer, factory policyGetClientFactory, ref string,
	outputFormat outputflag.OutputFlag[policyGetRenderFunc],
) error {
	refOrg, name, version, err := parsePolicyPackVersionRef(ref)
	if err != nil {
		return err
	}

	c, org, err := factory(ctx, refOrg)
	if err != nil {
		return err
	}

	if version == "" {
		var packs []cloudapitype.AppPolicyPackWithVersions
		var token *string
		for {
			resp, next, err := c.ListPolicyPacks(ctx, org, token)
			if err != nil {
				return fmt.Errorf("listing policy packs: %w", err)
			}
			packs = append(packs, resp.PolicyPacks...)
			if next == nil || *next == "" {
				break
			}
			token = next
		}
		version, err = resolveLatestVersionTag(packs, org, name)
		if err != nil {
			return err
		}
	}

	pack, err := c.GetPolicyPack(ctx, org, name, version)
	if err != nil {
		return err
	}
	if pack.Policies == nil {
		pack.Policies = []cloudapitype.AppPolicy{}
	}

	groups, err := policyGroupsEnabling(ctx, c, org, name)
	if err != nil {
		return err
	}

	return outputFormat.Get()(w, policyGetJSON{
		Organization:             org,
		AppGetPolicyPackResponse: pack,
		PolicyGroups:             groups,
	})
}

func policyGroupsEnabling(
	ctx context.Context, c policyGetClient, org, packName string,
) ([]policyPackGroupJSON, error) {
	groups := []policyPackGroupJSON{}
	var token *string
	for {
		resp, next, err := c.ListPolicyGroups(ctx, org, token)
		if err != nil {
			return nil, fmt.Errorf("listing policy groups: %w", err)
		}
		for _, summary := range resp.PolicyGroups {
			if summary.NumEnabledPolicyPacks == 0 {
				continue
			}
			group, err := c.GetPolicyGroup(ctx, org, summary.Name)
			if err != nil {
				return nil, fmt.Errorf("getting policy group %q: %w", summary.Name, err)
			}
			for _, applied := range group.AppliedPolicyPacks {
				if applied.Name == packName {
					groups = append(groups, policyPackGroupJSON{
						Name:         group.Name,
						Mode:         group.Mode,
						IsOrgDefault: group.IsOrgDefault,
						Version:      applied.Version,
						VersionTag:   applied.VersionTag,
					})
					break
				}
			}
		}
		if next == nil || *next == "" {
			return groups, nil
		}
		token = next
	}
}

type policyGetJSON struct {
	Organization string `json:"organization"`
	cloudapitype.AppGetPolicyPackResponse
	PolicyGroups []policyPackGroupJSON `json:"policyGroups"`
}

type policyPackGroupJSON struct {
	Name         string                       `json:"name"`
	Mode         cloudapitype.PolicyGroupMode `json:"mode"`
	IsOrgDefault bool                         `json:"isOrgDefault"`
	Version      int                          `json:"version"`
	VersionTag   string                       `json:"versionTag"`
}

func renderPolicyGetJSON(w io.Writer, resp policyGetJSON) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func renderPolicyGetText(w io.Writer, resp policyGetJSON) error {
	fmt.Fprintf(w, "%-14s %s\n", "Name:", resp.Name)
	fmt.Fprintf(w, "%-14s %s\n", "Display name:", resp.DisplayName)
	fmt.Fprintf(w, "%-14s %s (%d)\n", "Version:", resp.VersionTag, resp.Version)
	fmt.Fprintf(w, "%-14s %s\n", "Organization:", resp.Organization)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "POLICIES")
	policyRows := slice.Prealloc[cmdutil.TableRow](len(resp.Policies))
	for _, p := range resp.Policies {
		policyRows = append(policyRows, cmdutil.TableRow{Columns: []string{
			p.Name, string(p.EnforcementLevel), string(p.Severity), p.Description,
		}})
	}
	ui.FprintTable(w, cmdutil.Table{
		Headers: []string{"NAME", "ENFORCEMENT", "SEVERITY", "DESCRIPTION"},
		Rows:    policyRows,
	}, nil)

	fmt.Fprintln(w)
	if len(resp.PolicyGroups) == 0 {
		fmt.Fprintln(w, "Not enabled in any policy group.")
		return nil
	}
	fmt.Fprintln(w, "ENABLED IN POLICY GROUPS")
	groupRows := slice.Prealloc[cmdutil.TableRow](len(resp.PolicyGroups))
	for _, g := range resp.PolicyGroups {
		version := g.VersionTag
		if version == "" {
			version = strconv.Itoa(g.Version)
		}
		groupRows = append(groupRows, cmdutil.TableRow{Columns: []string{g.Name, string(g.Mode), version}})
	}
	ui.FprintTable(w, cmdutil.Table{
		Headers: []string{"NAME", "MODE", "VERSION"},
		Rows:    groupRows,
	}, nil)
	return nil
}
