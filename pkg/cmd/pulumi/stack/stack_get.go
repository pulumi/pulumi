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

package stack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// stackGetClient is the interface the get command needs from the API client.
type stackGetClient interface {
	GetStack(ctx context.Context, stackID client.StackIdentifier) (apitype.Stack, error)
}

// stackGetClientFactory builds a stackGetClient from the environment.
// It returns the client, the resolved StackIdentifier, and any error.
type stackGetClientFactory func(
	ctx context.Context, stackFlag string,
) (stackGetClient, client.StackIdentifier, error)

func newStackGetCmd() *cobra.Command {
	return newStackGetCmdWith(nil)
}

func newStackGetCmdWith(factory stackGetClientFactory) *cobra.Command {
	var (
		stack  string
		output string
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Retrieve detailed information about a stack",
		Long: "[EXPERIMENTAL] Retrieve detailed information about a stack.\n" +
			"\n" +
			"Displays the organization, project, and stack name, the current version,\n" +
			"all associated tags, any active update operation (with its kind, author,\n" +
			"and start time), and the active update UUID. By default the output is\n" +
			"human-readable; pass --output=json for a stable, machine-readable JSON\n" +
			"envelope.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if factory == nil {
				factory = defaultStackGetClientFactory
			}
			return runStackGet(cmd.Context(), cmd.OutOrStdout(), factory, stack, output)
		},
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.Flags().StringVarP(&stack, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	cmd.Flags().StringVarP(&output, "output", "o", "default",
		"The output format: default (human-readable) or json")

	return cmd
}

func defaultStackGetClientFactory(
	ctx context.Context, stackFlag string,
) (stackGetClient, client.StackIdentifier, error) {
	return RequireCloudStack(ctx, cmdutil.Diag(), pkgWorkspace.Instance, cmdBackend.DefaultLoginManager, stackFlag)
}

func runStackGet(
	ctx context.Context,
	w io.Writer,
	factory stackGetClientFactory,
	stackFlag string,
	output string,
) error {
	renderer, err := stackGetRenderer(output)
	if err != nil {
		return err
	}

	c, stackID, err := factory(ctx, stackFlag)
	if err != nil {
		return err
	}

	stack, err := c.GetStack(ctx, stackID)
	if err != nil {
		return fmt.Errorf("getting stack: %w", err)
	}

	return renderer(w, stack)
}

type stackGetRenderFunc func(w io.Writer, stack apitype.Stack) error

func stackGetRenderer(output string) (stackGetRenderFunc, error) {
	switch output {
	case "", "default":
		return renderStackGetDefault, nil
	case "json":
		return renderStackGetJSON, nil
	default:
		return nil, fmt.Errorf("invalid --output value %q: expected \"default\" or \"json\"", output)
	}
}

// stackGetEnvelope is the JSON shape emitted by `pulumi stack get --output=json`.
type stackGetEnvelope struct {
	Organization     string                 `json:"organization"`
	Project          string                 `json:"project"`
	Stack            string                 `json:"stack"`
	Version          int                    `json:"version"`
	ActiveUpdate     string                 `json:"activeUpdate"`
	CurrentOperation *stackGetOperationJSON `json:"currentOperation,omitempty"`
	Tags             map[string]string      `json:"tags"`
}

type stackGetOperationJSON struct {
	Kind    string `json:"kind"`
	Author  string `json:"author"`
	Started string `json:"started"`
}

func toStackGetEnvelope(s apitype.Stack) stackGetEnvelope {
	tags := make(map[string]string, len(s.Tags))
	for k, v := range s.Tags {
		tags[k] = v
	}
	env := stackGetEnvelope{
		Organization: s.OrgName,
		Project:      s.ProjectName,
		Stack:        string(s.StackName),
		Version:      s.Version,
		ActiveUpdate: s.ActiveUpdate,
		Tags:         tags,
	}
	if op := s.CurrentOperation; op != nil {
		env.CurrentOperation = &stackGetOperationJSON{
			Kind:    string(op.Kind),
			Author:  op.Author,
			Started: time.Unix(op.Started, 0).UTC().Format(time.RFC3339),
		}
	}
	return env
}

func renderStackGetJSON(w io.Writer, stack apitype.Stack) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(toStackGetEnvelope(stack))
}

func renderStackGetDefault(w io.Writer, stack apitype.Stack) error {
	fmt.Fprintf(w, "Organization:   %s\n", stack.OrgName)
	fmt.Fprintf(w, "Project:        %s\n", stack.ProjectName)
	fmt.Fprintf(w, "Stack:          %s\n", stack.StackName)
	fmt.Fprintf(w, "Version:        %d\n", stack.Version)
	if stack.ActiveUpdate != "" {
		fmt.Fprintf(w, "Active update:  %s\n", stack.ActiveUpdate)
	}

	if op := stack.CurrentOperation; op != nil {
		started := time.Unix(op.Started, 0)
		fmt.Fprintln(w, "Update in progress:")
		fmt.Fprintf(w, "    Kind:    %s\n", op.Kind)
		fmt.Fprintf(w, "    Author:  %s\n", op.Author)
		fmt.Fprintf(w, "    Started: %s (%s)\n", humanize.Time(started), started.Format(time.RFC3339))
	}

	if len(stack.Tags) > 0 {
		keys := make([]string, 0, len(stack.Tags))
		for k := range stack.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		maxKey := 0
		for _, k := range keys {
			if len(k) > maxKey {
				maxKey = len(k)
			}
		}

		fmt.Fprintln(w, "Tags:")
		for _, k := range keys {
			fmt.Fprintf(w, "    %-*s  %s\n", maxKey, k, stack.Tags[k])
		}
	}

	return nil
}
