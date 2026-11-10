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
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type policyExceptionRemoveArgs struct {
	org          string
	yes          bool
	outputFormat outputflag.OutputFlag[policyExceptionRemoveRenderFunc]
}

func defaultPolicyExceptionRemoveOutputFormat() outputflag.OutputFlag[policyExceptionRemoveRenderFunc] {
	return outputflag.OutputFlag[policyExceptionRemoveRenderFunc]{
		RenderForTerminal: renderPolicyExceptionRemoveText,
		RenderJSON:        renderPolicyExceptionRemoveJSON,
	}
}

func newPolicyExceptionRemoveCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var args policyExceptionRemoveArgs
	args.outputFormat = defaultPolicyExceptionRemoveOutputFormat()

	cmd := &cobra.Command{
		Use:     "remove <exception-id>",
		Aliases: []string{"rm", "delete"},
		Short:   "[EXPERIMENTAL] Remove a policy exception",
		Long: "[EXPERIMENTAL] Remove a policy exception.\n" +
			"\n" +
			"Removes an active policy exception, so the violations it exempted fail operations again.\n" +
			"Pulumi Cloud keeps the removed exception for auditing.\n" +
			"You will be prompted to confirm unless --yes is passed.",
		Example: "  pulumi policy exception remove EXC-12\n" +
			"  pulumi policy exception remove EXC-12 --yes",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			args.yes = args.yes || env.SkipConfirmations.Value()
			return runPolicyExceptionRemove(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(),
				newPolicyExceptionClientFactory(ws, lm), posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "exception-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the exception")
	cmd.Flags().BoolVarP(&args.yes, "yes", "y", false, "Skip confirmation prompts")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

func runPolicyExceptionRemove(
	ctx context.Context, stdout, stderr io.Writer,
	factory policyExceptionClientFactory, exceptionID string, args policyExceptionRemoveArgs,
) error {
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}
	prompt := fmt.Sprintf("This will remove policy exception %s, so the violations it exempts fail operations again!",
		exceptionID)
	if err := ui.ConfirmDeletion(args.yes, cmdutil.Interactive(), prompt, exceptionID, stderr, opts); err != nil {
		return err
	}

	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	err = policyExceptionsError(c.RemovePolicyException(ctx, org, exceptionID), org)
	apiErr, _ := errors.AsType[*apitype.ErrorResponse](err)
	switch {
	case apiErr != nil && apiErr.Code == http.StatusNotFound:
		return fmt.Errorf("policy exception %s not found in %s", exceptionID, org)
	// Removing an exception that's already removed does what was asked for.
	case apiErr != nil && apiErr.Code == http.StatusConflict && strings.Contains(apiErr.Message, "already removed"):
		fmt.Fprintf(stderr, "Policy exception %s is already removed.\n", exceptionID)
		return args.outputFormat.Get()(stdout, io.Discard, org, exceptionID)
	case err != nil:
		return err
	}

	return args.outputFormat.Get()(stdout, stderr, org, exceptionID)
}

// policyExceptionRemoveRenderFunc writes the result to stdout, and status messages to stderr.
type policyExceptionRemoveRenderFunc func(stdout, stderr io.Writer, org, exceptionID string) error

func renderPolicyExceptionRemoveText(_, stderr io.Writer, org, exceptionID string) error {
	fmt.Fprintf(stderr, "Removed policy exception %s from organization %s.\n", exceptionID, org)
	return nil
}

type policyExceptionRemoveJSON struct {
	OrganizationName string `json:"organizationName"`
	ID               string `json:"id"`
}

func renderPolicyExceptionRemoveJSON(stdout, _ io.Writer, org, exceptionID string) error {
	return ui.FprintJSON(stdout, policyExceptionRemoveJSON{
		OrganizationName: org,
		ID:               exceptionID,
	})
}
