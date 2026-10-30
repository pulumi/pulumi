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
	"time"

	"github.com/spf13/cobra"

	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type policyExceptionGetRenderFunc func(w io.Writer, exception apitype.PolicyExceptionInfo) error

type policyExceptionGetArgs struct {
	org          string
	outputFormat outputflag.OutputFlag[policyExceptionGetRenderFunc]
}

func defaultPolicyExceptionGetOutputFormat() outputflag.OutputFlag[policyExceptionGetRenderFunc] {
	return outputflag.OutputFlag[policyExceptionGetRenderFunc]{
		RenderForTerminal: renderPolicyExceptionGetText,
		RenderJSON: func(w io.Writer, exception apitype.PolicyExceptionInfo) error {
			return ui.FprintJSON(w, exception)
		},
	}
}

func newPolicyExceptionGetCmd(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var args policyExceptionGetArgs
	args.outputFormat = defaultPolicyExceptionGetOutputFormat()

	cmd := &cobra.Command{
		Use:   "get <exception-id>",
		Short: "[EXPERIMENTAL] Get the details of a policy exception",
		Long: "[EXPERIMENTAL] Get the details of a policy exception.\n" +
			"\n" +
			"Shows a policy exception, including what it exempts as a Policy Pack exceptions\n" +
			"configuration entry.\n" +
			"\n" +
			"Default output is a human-readable summary; pass --output=json for the\n" +
			"full response as JSON.",
		Example: "  pulumi policy exception get EXC-12\n" +
			"  pulumi policy exception get EXC-12 --output json",
		RunE: func(cmd *cobra.Command, posArgs []string) error {
			return runPolicyExceptionGet(
				cmd.Context(), cmd.OutOrStdout(), newPolicyExceptionClientFactory(ws, lm), posArgs[0], args)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "exception-id"},
		},
		Required: 1,
	})

	cmd.Flags().StringVar(&args.org, "org", "", "The organization that owns the exception")
	outputflag.VarP(cmd.Flags(), &args.outputFormat)

	return cmd
}

func runPolicyExceptionGet(
	ctx context.Context, w io.Writer,
	factory policyExceptionClientFactory, exceptionID string, args policyExceptionGetArgs,
) error {
	c, org, err := factory(ctx, args.org)
	if err != nil {
		return err
	}

	exception, err := c.GetPolicyException(ctx, org, exceptionID)
	err = policyExceptionsError(err, org)
	if apiErr, ok := errors.AsType[*apitype.ErrorResponse](err); ok && apiErr.Code == http.StatusNotFound {
		return fmt.Errorf("policy exception %s not found in %s", exceptionID, org)
	} else if err != nil {
		return err
	}

	return args.outputFormat.Get()(w, exception)
}

func renderPolicyExceptionGetText(w io.Writer, e apitype.PolicyExceptionInfo) error {
	fmt.Fprintf(w, "%-14s %s\n", "ID:", e.ID)
	fmt.Fprintf(w, "%-14s %s\n", "Policy pack:", e.PolicyPack)
	fmt.Fprintf(w, "%-14s %s\n", "Status:", e.Status)
	if e.ChangeRequestID != "" {
		fmt.Fprintf(w, "%-14s %s\n", "Requested in:", "change request "+e.ChangeRequestID)
	}
	for _, event := range []struct {
		label string
		at    time.Time
		by    apitype.UserInfo
	}{{"Created:", e.CreatedAt, e.CreatedBy}, {"Removed:", e.RemovedAt, e.RemovedBy}} {
		if event.at.IsZero() {
			continue
		}
		fmt.Fprintf(w, "%-14s %s", event.label, event.at.UTC().Format(time.RFC3339))
		if event.by.GitHubLogin != "" {
			fmt.Fprintf(w, " by %s", event.by.GitHubLogin)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "\nPolicy Pack exceptions configuration:")
	return ui.FprintJSON(w, e.PolicyException)
}
