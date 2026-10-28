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
	"slices"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	esc_client "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type policyExceptionClient interface {
	CreatePolicyException(
		ctx context.Context, orgName string, req apitype.CreatePolicyExceptionRequest,
	) (apitype.PolicyExceptionInfo, bool, error)
	ListPolicyExceptions(
		ctx context.Context, orgName string, opts client.ListPolicyExceptionsOptions,
	) (apitype.ListPolicyExceptionsResponse, error)
	GetPolicyException(ctx context.Context, orgName, exceptionID string) (apitype.PolicyExceptionInfo, error)
	RemovePolicyException(ctx context.Context, orgName, exceptionID string) error
	CreatePolicyExceptionDraft(
		ctx context.Context, orgName string, req apitype.CreatePolicyExceptionRequest,
	) (apitype.PolicyExceptionDraft, error)
	// URL is the Pulumi Cloud API URL, which console links are derived from.
	URL() string
	GetStackUpdates(
		ctx context.Context, stack client.StackIdentifier, pageSize int, page int,
	) ([]apitype.UpdateInfo, error)
	GetLatestStackPreviews(ctx context.Context, stack client.StackIdentifier) ([]apitype.StackPreview, error)
	GetUpdateEngineEvents(
		ctx context.Context, update client.UpdateIdentifier, opts client.GetUpdateEngineEventsOptions,
	) (apitype.GetUpdateEventsResponse, error)
}

// changeRequestClient submits and reads change requests. The ESC client has these routes, which policy exceptions
// share.
type changeRequestClient interface {
	SubmitChangeRequest(ctx context.Context, orgName, changeRequestID string, description *string) error
	GetChangeRequest(
		ctx context.Context, orgName, changeRequestID string,
	) (*esc_client.GetChangeRequestResponse, error)
}

// policyExceptionClients are the Pulumi Cloud clients of the policy exception commands.
type policyExceptionClients struct {
	policyExceptionClient
	// changeRequests requests exceptions when the organization requires approval.
	changeRequests changeRequestClient
}

type policyExceptionClientFactory func(
	ctx context.Context, orgFlag string,
) (policyExceptionClients, string, error)

func newPolicyExceptionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exception",
		Short: "[EXPERIMENTAL] Manage policy exceptions",
		Long: "[EXPERIMENTAL] Manage policy exceptions.\n" +
			"\n" +
			"A policy exception allows violations of the policies it lists on the resources it lists, so\n" +
			"they don't fail an operation. Each exception belongs to one Policy Pack, and is stored in\n" +
			"Pulumi Cloud.",
		Hidden: !env.Experimental.Value(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.AddCommand(newPolicyExceptionNewCmd())
	cmd.AddCommand(newPolicyExceptionListCmd())
	cmd.AddCommand(newPolicyExceptionGetCmd())
	cmd.AddCommand(newPolicyExceptionRemoveCmd())
	return cmd
}

func defaultPolicyExceptionClientFactory(
	ctx context.Context, orgFlag string,
) (policyExceptionClients, string, error) {
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	be, err := cmdBackend.CurrentBackend(ctx, ws, cmdBackend.DefaultLoginManager, nil, opts)
	if err != nil {
		return policyExceptionClients{}, "", err
	}
	cloudBackend, ok := be.(httpstate.Backend)
	if !ok {
		return policyExceptionClients{}, "",
			errors.New("policy exceptions require the Pulumi Cloud backend; run `pulumi login`")
	}

	userName, orgs, _, err := cloudBackend.CurrentUser()
	if err != nil {
		return policyExceptionClients{}, "", err
	}

	org := orgFlag
	if org == "" {
		defaultOrg, err := cloudBackend.GetDefaultOrg(ctx)
		if err != nil {
			return policyExceptionClients{}, "", err
		}
		org = defaultOrg
	}
	if org == "" {
		org = userName
	}

	if !slices.Contains(orgs, org) && org != userName {
		return policyExceptionClients{}, "", fmt.Errorf("user %s is not a member of organization %s", userName, org)
	}

	return policyExceptionClients{cloudBackend.Client(), cloudBackend.ESCClient()}, org, nil
}
