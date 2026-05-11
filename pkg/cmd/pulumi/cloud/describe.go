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

package cloud

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// newDescribeCmd builds `pulumi cloud api describe <path>`. api carries the
// parent command's persistent flags (--refresh-spec).
func newDescribeCmd(api *apiCommand) *cobra.Command {
	var method string
	var format string

	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Show the parameters and schemas for a Pulumi Cloud API operation",
		Long: "Show the parameters, request body, and response schema for a Pulumi Cloud\n" +
			"API operation.\n" +
			"\n" +
			"The argument may be either a path (with optional template variables, e.g.\n" +
			"`/api/stacks/{orgName}`) or an operation ID as shown in `list` (e.g.\n" +
			"`ListAccounts`). Operation IDs are matched case-insensitively.\n" +
			"\n" +
			"Default output is a human-readable schema render. Pass --format=json for the\n" +
			"stable agent envelope, including the inlined JSON schema.",
		Example: "  # Describe an operation by its ID.\n" +
			"  pulumi cloud api describe ListOrgMembers\n\n" +
			"  # Describe by path — use --method when the same path maps to multiple ops.\n" +
			"  pulumi cloud api describe /api/orgs/{orgName}/members --method=POST\n\n" +
			"  # Paste-friendly: copy a METHOD + path row from `list` verbatim.\n" +
			"  pulumi cloud api describe 'GET /api/user'\n\n" +
			"  # Extract just the request body schema.\n" +
			"  pulumi cloud api describe CreateStackTag --format=json | jq '.operation.requestBody.jsonSchema'\n\n" +
			"  # Pull the parameter list for scripting.\n" +
			"  pulumi cloud api describe GetStack --format=json | jq '.operation.parameters[] | {name, in, required}'",
	}
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "path-or-operation-id"}},
		Required:  1,
	})
	cmd.Flags().StringVarP(&method, "method", "X", "GET",
		"HTTP method to look up (a path can map to multiple ops by method)")
	cmd.Flags().StringVar(&format, "format", "",
		"Output format: default is a human-readable schema render; "+
			"`markdown` emits a markdown document (piping friendly, renders in IDEs/glow); "+
			"`json` emits the stable agent envelope")

	cmd.RunE = runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return runDescribe(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), args, method,
			cmd.Flags().Changed("method"), format, api.refreshSpec)
	})
	return cmd
}

func runDescribe(
	ctx context.Context,
	w, warnW io.Writer,
	args []string, method string, methodExplicit bool, format string, refresh bool,
) error {
	mode, err := resolveOutput(format)
	if err != nil {
		return err
	}

	idx, err := LoadIndex(ctx, warnW, refresh)
	if err != nil {
		return err
	}

	method = strings.ToUpper(method)
	userArg := strings.TrimSpace(args[0])

	if verb, rest, ok := splitLeadingHTTPMethod(userArg); ok {
		if methodExplicit && method != verb {
			return NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
				fmt.Sprintf("conflicting methods: argument has %q, --method is %q", verb, method)).
				WithField("method")
		}
		method = verb
		userArg = rest
	}

	var mr *MatchResult
	if looksLikeOperationID(userArg) {
		mr, err = MatchByOperationID(idx, userArg)
		if err != nil {
			return err
		}
	} else {
		userPath, _ := splitPathQuery(userArg)
		mr, err = MatchPath(idx, method, userPath)
		if err != nil {
			return err
		}
	}

	//exhaustive:ignore // outputDefault/outputTable/outputRaw all render text.
	switch mode {
	case outputJSON:
		return emitDescribeJSON(w, mr.Op)
	case outputMarkdown:
		return emitDescribeMarkdown(w, mr.Op)
	default:
		return emitDescribeText(w, mr.Op)
	}
}

// emitDescribeMarkdown writes a markdown description of the operation to w.
func emitDescribeMarkdown(w io.Writer, op *Operation) error {
	s := RenderDescribeMarkdown(op)
	if _, err := io.WriteString(w, s); err != nil {
		return err
	}
	if !strings.HasSuffix(s, "\n") {
		fmt.Fprintln(w)
	}
	return nil
}

func emitDescribeJSON(w io.Writer, op *Operation) error {
	payload := describedOp{
		OperationID:  op.OperationID,
		Method:       op.Method,
		Path:         op.Path,
		Summary:      op.Summary,
		Description:  op.Description,
		Tag:          op.Tag,
		Preview:      op.IsPreview,
		Deprecated:   op.IsDeprecated,
		SupersededBy: op.SupersededBy,
		Parameters:   op.Params,
	}
	if op.HasBody {
		payload.RequestBody = &bodyJSON{
			ContentType: op.BodyContentType,
			Schema:      op.BodySchemaText,
			JSONSchema:  op.BodySchemaJSON,
		}
	}
	if op.ResponseSchemaText != "" || op.ResponseContentType != "" {
		payload.SuccessResponse = &bodyJSON{
			ContentType: op.ResponseContentType,
			Schema:      op.ResponseSchemaText,
			JSONSchema:  op.ResponseSchemaJSON,
		}
	}
	return WriteJSON(w, describeEnvelope{
		SchemaVersion: SchemaVersion,
		Operation:     payload,
	}, cmdutil.Interactive())
}

// emitDescribeText writes a human-readable view to w.
func emitDescribeText(w io.Writer, op *Operation) error {
	s := RenderDescribeText(op)
	if _, err := io.WriteString(w, s); err != nil {
		return err
	}
	if !strings.HasSuffix(s, "\n") {
		fmt.Fprintln(w)
	}
	return nil
}

// RenderDescribeText builds the human-readable view of an operation and
// returns it as a string. Used by emitDescribeText for the CLI and by the
// TUI's Browse tab to fill the details viewport.
func RenderDescribeText(op *Operation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", op.Method, op.Path)
	if op.Tag != "" {
		fmt.Fprintf(&b, "Tag: %s\n", op.Tag)
	}
	if op.IsPreview {
		fmt.Fprintf(&b, "Status: PREVIEW — not yet stable, may change without notice\n")
	}
	if op.IsDeprecated {
		if op.SupersededBy != "" {
			fmt.Fprintf(&b, "Status: DEPRECATED — use %s instead\n", op.SupersededBy)
		} else {
			fmt.Fprintf(&b, "Status: DEPRECATED — will be removed in a future version\n")
		}
	}
	if op.OperationID != "" {
		fmt.Fprintf(&b, "Operation: %s\n", op.OperationID)
	}
	if op.Summary != "" {
		fmt.Fprintf(&b, "\n%s\n", op.Summary)
	}
	if op.Description != "" && op.Description != op.Summary {
		fmt.Fprintf(&b, "\n%s\n", op.Description)
	}

	if len(op.Params) > 0 {
		b.WriteString("\nParameters:\n")
		for _, p := range op.Params {
			req := ""
			if p.Required {
				req = "*"
			}
			fmt.Fprintf(&b, "  [%s] %s%s (%s) — %s\n",
				p.In, p.Name, req, p.Type, p.Description)
			if len(p.Values) > 0 {
				fmt.Fprintf(&b, "      allowed: %s\n", strings.Join(p.Values, ", "))
			}
		}
	}

	if op.HasBody {
		fmt.Fprintf(&b, "\nRequest body (%s):\n", op.BodyContentType)
		b.WriteString(indent(op.BodySchemaText, "  "))
	}
	if op.ResponseSchemaText != "" {
		fmt.Fprintf(&b, "\nSuccess response (%s):\n", op.ResponseContentType)
		b.WriteString(indent(op.ResponseSchemaText, "  "))
	}
	return b.String()
}

// indent prefixes every non-empty line of s with prefix. Used to visually
// nest rendered schema blocks under their "Request body:" / "Response:"
// headers — structural indentation that terminal wrapping can't replicate.
func indent(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	var out strings.Builder
	for i, ln := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}
		if ln != "" {
			out.WriteString(prefix)
		}
		out.WriteString(ln)
	}
	if !strings.HasSuffix(s, "\n") {
		out.WriteByte('\n')
	}
	return out.String()
}
