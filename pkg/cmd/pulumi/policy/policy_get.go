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
}

// policyGetClientFactory receives the organization parsed from the argument
// (possibly empty) and returns the client along with the resolved organization.
type policyGetClientFactory func(ctx context.Context, org string) (policyGetClient, string, error)

type policyGetRenderFunc func(w io.Writer, pack cloudapitype.AppGetPolicyPackResponse) error

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
		Use:   "get",
		Short: "[EXPERIMENTAL] Get the details of a Policy Pack",
		Long: "[EXPERIMENTAL] Get the details of a Policy Pack.\n" +
			"\n" +
			"Shows the policies contained in a published version of a Policy Pack. The\n" +
			"organization defaults to your default organization, and the version defaults\n" +
			"to latest.\n" +
			"\n" +
			"Examples:\n" +
			"  pulumi policy get acme/aws-guardrails 1.2.0\n" +
			"  pulumi policy get aws-guardrails --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			version := latestKeyword
			if len(posArgs) > 1 {
				version = posArgs[1]
			}
			return runPolicyGet(cmd.Context(), cmd.OutOrStdout(), factory, posArgs[0], version, outputFormat)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "policy-pack", Usage: "[<org-name>/]<policy-pack-name>"},
			{Name: "version", Usage: "[latest|version]"},
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

// parseOrgPolicyPackName splits "[org/]name".
func parseOrgPolicyPackName(s string) (org, name string, err error) {
	parts := strings.Split(s, "/")
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		org, name = parts[0], parts[1]
		if org == "" {
			return "", "", fmt.Errorf("invalid policy pack %q: organization must not be empty", s)
		}
	default:
		return "", "", fmt.Errorf("invalid policy pack %q: expected [<org-name>/]<policy-pack-name>", s)
	}
	if name == "" {
		return "", "", fmt.Errorf("invalid policy pack %q: policy pack name must not be empty", s)
	}
	return org, name, nil
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
	ctx context.Context, w io.Writer, factory policyGetClientFactory, ref, version string,
	outputFormat outputflag.OutputFlag[policyGetRenderFunc],
) error {
	refOrg, name, err := parseOrgPolicyPackName(ref)
	if err != nil {
		return err
	}
	if version == "" {
		return fmt.Errorf("invalid version: must be %q or a version", latestKeyword)
	}

	c, org, err := factory(ctx, refOrg)
	if err != nil {
		return err
	}

	if version == latestKeyword {
		var packs []cloudapitype.AppPolicyPackWithVersions
		var token *string
		for {
			resp, next, err := c.ListPolicyPacks(ctx, org, token)
			if err != nil {
				return err
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

	return outputFormat.Get()(w, pack)
}

func renderPolicyGetJSON(w io.Writer, pack cloudapitype.AppGetPolicyPackResponse) error {
	return writeJSON(w, pack)
}

func renderPolicyGetText(w io.Writer, pack cloudapitype.AppGetPolicyPackResponse) error {
	fmt.Fprintf(w, "%-14s %s\n", "Name:", pack.Name)
	fmt.Fprintf(w, "%-14s %s\n", "Display name:", pack.DisplayName)
	if pack.VersionTag != "" {
		fmt.Fprintf(w, "%-14s %s (v%d)\n", "Version:", pack.VersionTag, pack.Version)
	} else {
		fmt.Fprintf(w, "%-14s v%d\n", "Version:", pack.Version)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "POLICIES")
	policyRows := slice.Prealloc[cmdutil.TableRow](len(pack.Policies))
	for _, p := range pack.Policies {
		policyRows = append(policyRows, cmdutil.TableRow{Columns: []string{
			p.Name, string(p.EnforcementLevel), string(p.Severity), p.Description,
		}})
	}
	ui.FprintTable(w, cmdutil.Table{
		Headers: []string{"NAME", "ENFORCEMENT", "SEVERITY", "DESCRIPTION"},
		Rows:    policyRows,
	}, nil)
	return nil
}
