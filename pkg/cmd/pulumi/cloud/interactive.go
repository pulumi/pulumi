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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// TUIConfig carries the bits the interactive TUI needs that are derived
// from the local context (selected stack, current project, default org).
// Populated at the cobra boundary so the tui subpackage never has to
// resolve any of it itself.
type TUIConfig struct {
	Client   *client.Client
	CloudURL string
	Org      string
	Project  string
	Stack    string
}

// InteractiveRunner is the interactive TUI entry point. The cloud/tui
// subpackage registers its Run here via init(); when interactive mode is
// requested this variable is non-nil.
var InteractiveRunner func(ctx context.Context, idx *Index, cfg TUIConfig) (stdoutBytes []byte, err error)

// RequireNonEmpty is the Required-style validator used for inputs whose
// empty value would be meaningless (required path parameters, required
// query parameters).
func RequireNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value required")
	}
	return nil
}

// runInteractive implements the zero-args TTY experience: launches the
// TUI so the user can browse endpoints, fill in request params, send,
// and view the response — all without leaving the program.
func runInteractive(cmd *cobra.Command, api *apiCommand) error {
	idx, err := LoadIndex(cmd.Context(), cmd.ErrOrStderr(), api.refreshSpec)
	if err != nil {
		return err
	}
	if InteractiveRunner == nil {
		return NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			"interactive TUI not linked into this binary")
	}

	rctx, err := ResolveContext(cmd.Context())
	if err != nil {
		return NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("resolving cloud context: %v", err))
	}
	if !rctx.LoggedIn {
		return NewAPIError(cmdutil.ExitAuthenticationError, ErrNotLoggedIn,
			"not logged in to Pulumi Cloud").
			WithSuggestions("run `pulumi login` first")
	}

	cfg := TUIConfig{
		Client:   rctx.Client,
		CloudURL: rctx.CloudURL,
		Org:      defaultOrg(rctx),
		Project:  defaultProject(rctx),
		Stack:    rctx.StackName,
	}

	stdoutBytes, err := InteractiveRunner(cmd.Context(), idx, cfg)
	if err != nil {
		return err
	}
	if stdoutBytes != nil {
		if _, werr := os.Stdout.Write(stdoutBytes); werr != nil {
			return werr
		}
		if len(stdoutBytes) > 0 && stdoutBytes[len(stdoutBytes)-1] != '\n' {
			fmt.Fprintln(os.Stdout)
		}
	}
	return nil
}

// PathParamsInOrder returns the spec path's template segments in order.
func PathParamsInOrder(op *Operation) []string {
	var out []string
	for _, seg := range splitSegments(op.Path) {
		if isTemplateSegment(seg) {
			out = append(out, trimBraces(seg))
		}
	}
	return out
}

// InteractiveDefault computes the default string shown in the prompt for a
// named path parameter. Returns "" when no sensible default is available.
func InteractiveDefault(name string, cfg TUIConfig) string {
	switch templateVarKind(name) {
	case kindOrg:
		return cfg.Org
	case kindProject:
		return cfg.Project
	case kindStack:
		return cfg.Stack
	case kindNone:
	}
	return ""
}

// SubstituteInteractivePath is like buildConcretePath but without the
// URL-encoding pass, because we show the path to a human and also need it to
// round-trip through the runAPI arg parser (which will re-encode).
func SubstituteInteractivePath(op *Operation, values map[string]string) string {
	segs := splitSegments(op.Path)
	for i, seg := range segs {
		if isTemplateSegment(seg) {
			segs[i] = values[trimBraces(seg)]
		}
	}
	return strings.Join(segs, "/")
}

// BodySkeletonSeed returns the initial body text for the editor. Prefers a
// schema-derived skeleton with required fields populated; falls back to an
// empty JSON object.
func BodySkeletonSeed(op *Operation) string {
	if raw := GenerateBodySkeleton(op.BodySchemaJSON); raw != nil {
		return string(raw) + "\n"
	}
	return "{}\n"
}

// templateVarKind classifies a path-template variable by name so callers
// can pick the right context-derived default.
type varKind int

const (
	kindNone varKind = iota
	kindOrg
	kindProject
	kindStack
)

func templateVarKind(name string) varKind {
	switch name {
	case "orgName", "org":
		return kindOrg
	case "projectName", "project":
		return kindProject
	case "stackName", "stack":
		return kindStack
	}
	return kindNone
}

// defaultOrg picks the best-known org name for filling in {orgName} in the
// interactive picker. Prefers a stack-derived org over the resolved default.
func defaultOrg(rctx *ResolvedContext) string {
	if rctx.StackOrg != "" {
		return rctx.StackOrg
	}
	return rctx.OrgName
}

// defaultProject picks the best-known project name for filling in
// {projectName}. Prefers the current project file over a stack-derived one.
func defaultProject(rctx *ResolvedContext) string {
	if rctx.Project != nil && rctx.Project.Name != "" {
		return string(rctx.Project.Name)
	}
	return rctx.StackProj
}
