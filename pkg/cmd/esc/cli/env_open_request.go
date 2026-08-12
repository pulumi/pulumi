// Copyright 2025, Pulumi Corporation.
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
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

const (
	defaultGrantExpiration = 90000 * time.Second
	defaultAccessDuration  = 259200 * time.Second

	approvalPollInterval = 5 * time.Second
	defaultWaitTimeout   = 5 * time.Minute
)

func newEnvOpenRequestCmd(envcmd *envCommand) *cobra.Command {
	var grantExpiration time.Duration
	var accessDuration time.Duration
	var reason string
	var output string

	cmd := &cobra.Command{
		Use:   "open-request [<org-name>/][<project-name>/]<environment-name>[@<version>]",
		Args:  cobra.ExactArgs(1),
		Short: "Create a request for opening a protected environment.",
		Long: "Create a request for opening a protected environment with the given name.\n" +
			"\n" +
			"This command creates a request to open a protected environment. The request must be\n" +
			"approved before the environment can be accessed.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			format, err := parseOutputFormat(output)
			if err != nil {
				return err
			}

			if err := envcmd.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, _, err := envcmd.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}

			changeRequests, err := envcmd.submitOpenRequest(ctx, ref, grantExpiration, accessDuration, reason)
			if err != nil {
				return err
			}

			if format == outputJSON {
				return writeJSON(envcmd.esc.stdout, struct {
					ChangeRequestID string `json:"changeRequestId"`
				}{changeRequests[0].ChangeRequestID})
			}

			for i := range changeRequests {
				cr := changeRequests[i]
				crRef := environmentRef{
					orgName:     ref.orgName,
					projectName: cr.ProjectName,
					envName:     cr.EnvironmentName,
				}
				fmt.Fprintf(
					envcmd.esc.stdout,
					"Created environment open request with ID: %s\n",
					cr.ChangeRequestID,
				)
				fmt.Fprintf(
					envcmd.esc.stdout,
					"Change request URL: %v\n",
					envcmd.esc.changeRequestURL(crRef, cr.ChangeRequestID),
				)
				fmt.Fprintln(envcmd.esc.stdout, "Change request submitted")
			}

			return nil
		},
	}

	cmd.Flags().DurationVar(
		&grantExpiration, "grant-expiration-seconds", defaultGrantExpiration,
		"expiration time for the grant in seconds")
	cmd.Flags().DurationVar(
		&accessDuration, "access-duration-seconds", defaultAccessDuration,
		"duration of access in seconds")
	cmd.Flags().StringVar(
		&reason, "reason", "",
		"an optional reason explaining why the environment is being opened, shown to approvers")
	addOutputFlag(cmd, &output)

	return cmd
}

func addRequestApprovalFlags(cmd *cobra.Command, opts *openApprovalOptions) {
	cmd.Flags().BoolVar(
		&opts.requestApproval, "request-approval", false,
		"if the environment requires approval to open, submit an open request")
	cmd.Flags().BoolVar(
		&opts.waitForApproval, "wait-for-approval", false,
		"wait for the submitted open request to be approved, then continue; implies --request-approval")
	cmd.Flags().DurationVar(
		&opts.waitTimeout, "wait-timeout", defaultWaitTimeout,
		"how long --wait-for-approval waits before giving up (e.g. 30s, 5m)")
	cmd.Flags().DurationVar(
		&opts.accessDuration, "access-duration", defaultAccessDuration,
		"how long access to the environment lasts once the request is approved (e.g. 5m, 2h)")
	cmd.Flags().StringVar(
		&opts.reason, "reason", "",
		"an optional reason explaining why the environment is being opened, shown to approvers")
}

// submitOpenRequest submits every change request the open request produces: one for the target
// environment and one for each gated import.
func (env *envCommand) submitOpenRequest(
	ctx context.Context,
	ref environmentRef,
	grantExpiration, accessDuration time.Duration,
	reason string,
) ([]client.EnvironmentOpenRequestChangeRequest, error) {
	resp, err := env.esc.client.CreateEnvironmentOpenRequest(
		ctx,
		ref.orgName,
		ref.projectName,
		ref.envName,
		int(grantExpiration.Seconds()),
		int(accessDuration.Seconds()),
	)
	if err != nil {
		return nil, err
	}
	if len(resp.ChangeRequests) == 0 {
		return nil, errors.New("no open request was created for this environment; " +
			"check that an open approval rule applies to it")
	}

	var description *string
	if reason != "" {
		description = &reason
	}
	for i := range resp.ChangeRequests {
		if err := env.esc.client.SubmitChangeRequest(
			ctx, ref.orgName, resp.ChangeRequests[i].ChangeRequestID, description,
		); err != nil {
			return nil, fmt.Errorf("submitting change request: %w", err)
		}
	}
	return resp.ChangeRequests, nil
}

type openApprovalOptions struct {
	requestApproval bool
	waitForApproval bool
	accessDuration  time.Duration
	waitTimeout     time.Duration
	reason          string
}

// withOpenApproval runs attempt and, if it fails and an open request was asked for, submits one
// and retries. A successful attempt is the only approval signal available: the service exposes no
// change request status API.
func (env *envCommand) withOpenApproval(
	ctx context.Context,
	ref environmentRef,
	opts openApprovalOptions,
	attempt func() error,
) error {
	err := attempt()
	if err == nil || (!opts.requestApproval && !opts.waitForApproval) {
		return err
	}

	accessDuration := valueOrDefault(opts.accessDuration, defaultAccessDuration)
	interval := valueOrDefault(env.pollInterval, approvalPollInterval)
	timeout := valueOrDefault(opts.waitTimeout, defaultWaitTimeout)

	changeRequests, requestErr := env.submitOpenRequest(
		ctx, ref, defaultGrantExpiration, accessDuration, opts.reason)
	if requestErr != nil {
		// attempt may have failed for a reason that has nothing to do with approvals.
		return err
	}

	for i := range changeRequests {
		cr := changeRequests[i]
		crRef := environmentRef{orgName: ref.orgName, projectName: cr.ProjectName, envName: cr.EnvironmentName}
		fmt.Fprintf(env.esc.stderr, "Submitted environment open request: %v\n",
			env.esc.changeRequestURL(crRef, cr.ChangeRequestID))
	}
	if !opts.waitForApproval {
		return err
	}
	fmt.Fprintf(env.esc.stderr, "Waiting up to %v for approval...\n", timeout)

	for waited := time.Duration(0); waited < timeout; waited += interval {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
		if err = attempt(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("timed out waiting for the open request to be approved: %w", err)
}
