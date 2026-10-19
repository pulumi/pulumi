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
	"path"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// policyExceptionClient is the subset of the Pulumi Cloud API that the policy exception commands use.
type policyExceptionClient interface {
	CreatePolicyException(
		ctx context.Context, orgName string, req apitype.CreatePolicyExceptionRequest,
	) (apitype.PolicyExceptionInfo, error)
	ListPolicyExceptions(
		ctx context.Context, orgName string, query client.ListPolicyExceptionsQuery,
	) ([]apitype.PolicyExceptionInfo, error)
	GetPolicyException(ctx context.Context, orgName, id string) (apitype.PolicyExceptionInfo, error)
	ListPolicyExceptionMatchUsage(ctx context.Context, orgName, id string) ([]apitype.PolicyExceptionMatchUsage, error)
	RemovePolicyException(ctx context.Context, orgName, id string) (apitype.PolicyExceptionInfo, error)
	CreatePolicyExceptionDraft(
		ctx context.Context, orgName string, req apitype.CreatePolicyExceptionRequest,
	) (apitype.PolicyExceptionDraft, error)
	GetPolicyExceptionDraft(ctx context.Context, orgName, changeRequestID string) (apitype.PolicyExceptionDraft, error)
	ListChangeRequests(ctx context.Context, orgName, entityType string) ([]apitype.ChangeRequest, error)
	SubmitChangeRequest(ctx context.Context, orgName, changeRequestID, description string) error
	GetStackUpdates(
		ctx context.Context, stack client.StackIdentifier, pageSize int, page int,
	) ([]apitype.UpdateInfo, error)
	GetLatestStackPreviews(ctx context.Context, stack client.StackIdentifier) ([]apitype.StackPreview, error)
	GetUpdateEngineEvents(
		ctx context.Context, update client.UpdateIdentifier, opts client.GetUpdateEngineEventsOptions,
	) (apitype.GetUpdateEventsResponse, error)
}

// policyExceptionEnv is what the policy exception commands need from the environment, so tests can replace it.
type policyExceptionEnv struct {
	client policyExceptionClient
	// org is the organization to use when neither a flag nor a stack names one.
	org        string
	consoleURL func(paths ...string) string
	// stacks resolves --stack values: the current stack when there are none, and wildcards when a value has a "*".
	stacks      func(ctx context.Context, values []string) ([]client.StackIdentifier, error)
	interactive bool
}

type policyExceptionEnvFactory func(ctx context.Context) (policyExceptionEnv, error)

func newPolicyExceptionCmd() *cobra.Command {
	return newPolicyExceptionCmdWith(defaultPolicyExceptionEnv)
}

func newPolicyExceptionCmdWith(factory policyExceptionEnvFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exception",
		Short: "[EXPERIMENTAL] Manage policy exceptions",
		Long: "[EXPERIMENTAL] Manage policy exceptions.\n" +
			"\n" +
			"A policy exception allows the violations of the policies it lists on the resources it lists, so they\n" +
			"don't fail an operation. Exceptions belong to one policy pack, and are stored in Pulumi Cloud.",
		Hidden: !env.Experimental.Value(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newPolicyExceptionNewCmd(factory))
	cmd.AddCommand(newPolicyExceptionListCmd(factory))
	cmd.AddCommand(newPolicyExceptionGetCmd(factory))
	cmd.AddCommand(newPolicyExceptionRemoveCmd(factory))
	return cmd
}

func defaultPolicyExceptionEnv(ctx context.Context) (policyExceptionEnv, error) {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}
	be, err := cmdBackend.CurrentBackend(ctx, pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return policyExceptionEnv{}, err
	}
	cloud, ok := be.(httpstate.Backend)
	if !ok {
		return policyExceptionEnv{}, errors.New("policy exceptions need Pulumi Cloud; run `pulumi login`")
	}

	org, err := cloud.GetDefaultOrg(ctx)
	if err != nil {
		return policyExceptionEnv{}, err
	}
	if org == "" {
		if org, _, _, err = cloud.CurrentUser(); err != nil {
			return policyExceptionEnv{}, err
		}
	}

	c := cloud.Client()
	return policyExceptionEnv{
		client: c,
		org:    org,
		consoleURL: func(paths ...string) string {
			return client.CloudConsoleURL(cloud.CloudURL(), paths...)
		},
		stacks: func(ctx context.Context, values []string) ([]client.StackIdentifier, error) {
			return resolveExceptionStacks(ctx, c, org, values)
		},
		interactive: cmdutil.Interactive(),
	}, nil
}

// resolveExceptionStacks resolves stack names like --stack does, and expands values with a "*" against the
// organization's stacks. A wildcard is matched against `stack`, `project/stack` or `org/project/stack`, depending
// on how many parts it has.
func resolveExceptionStacks(
	ctx context.Context, c *client.Client, org string, values []string,
) ([]client.StackIdentifier, error) {
	if len(values) == 0 {
		values = []string{""}
	}
	var stacks []client.StackIdentifier
	for _, v := range values {
		if !strings.Contains(v, "*") {
			_, id, err := cmdStack.RequireCloudStack(
				ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, v)
			if err != nil {
				return nil, err
			}
			stacks = append(stacks, id)
			continue
		}

		pattern, parts := v, strings.Split(v, "/")
		stackOrg := org
		if len(parts) == 3 {
			stackOrg, pattern = parts[0], parts[1]+"/"+parts[2]
		}
		matched := 0
		var token *string
		for {
			summaries, next, err := c.ListStacks(ctx, client.ListStacksFilter{Organization: &stackOrg}, token)
			if err != nil {
				return nil, err
			}
			for _, s := range summaries {
				name := s.ProjectName + "/" + s.StackName
				if len(parts) == 1 {
					name = s.StackName
				}
				if ok, _ := path.Match(pattern, name); !ok {
					continue
				}
				stackName, err := tokens.ParseStackName(s.StackName)
				if err != nil {
					return nil, err
				}
				stacks = append(stacks, client.StackIdentifier{Owner: s.OrgName, Project: s.ProjectName, Stack: stackName})
				matched++
			}
			if next == nil || *next == "" {
				break
			}
			token = next
		}
		if matched == 0 {
			return nil, fmt.Errorf("no stacks in %s match %q", stackOrg, v)
		}
	}
	slices.SortFunc(stacks, func(a, b client.StackIdentifier) int { return strings.Compare(a.String(), b.String()) })
	return slices.CompactFunc(stacks, func(a, b client.StackIdentifier) bool { return a == b }), nil
}

// parsePolicyPackArg splits `[<org>/]<pack>`. The org is empty when the argument doesn't name one.
func parsePolicyPackArg(s string) (string, string, error) {
	org, pack, found := strings.Cut(s, "/")
	if !found {
		return "", s, nil
	}
	if org == "" || pack == "" || strings.Contains(pack, "/") {
		return "", "", cmdCmd.ConfigurationError{
			Message: fmt.Sprintf("invalid policy pack %q: expected [<org>/]<policy-pack>", s),
		}
	}
	return org, pack, nil
}

// apiErrorCode returns the HTTP status of a Pulumi Cloud API error, or 0 for any other error.
func apiErrorCode(err error) int {
	if apiErr, ok := errors.AsType[*apitype.ErrorResponse](err); ok {
		return apiErr.Code
	}
	return 0
}

func exceptionConsoleURL(env policyExceptionEnv, org, id string) string {
	return env.consoleURL(org, "insights", "policy", "exceptions", id)
}

func changeRequestConsoleURL(env policyExceptionEnv, org, id string) string {
	return env.consoleURL(org, "change-requests", id)
}

// exceptionDefinition is the part of an exception a user writes: the same shape as in a version 2 policy pack
// config file, and in the API.
type exceptionDefinition struct {
	Reason string                         `json:"reason" yaml:"reason"`
	Match  []apitype.PolicyExceptionMatch `json:"match" yaml:"match"`
}

// configSnippet renders an exception as a version 2 policy pack config file.
func configSnippet(id string, def exceptionDefinition) string {
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(struct {
		Version    int                            `yaml:"version"`
		Exceptions map[string]exceptionDefinition `yaml:"exceptions"`
	}{2, map[string]exceptionDefinition{id: def}}); err != nil {
		return err.Error()
	}
	return b.String()
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
