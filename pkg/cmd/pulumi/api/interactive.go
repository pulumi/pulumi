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

package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// TUIConfig carries the subset of apiFlags the interactive TUI consumes.
// It's populated at the cobra boundary so the tui subpackage never has to
// know the flag parsing shape.
type TUIConfig struct {
	Org     string
	Project string
	Stack   string
}

// InteractiveRunner is the interactive TUI entry point. The api/tui
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
func runInteractive(cmd *cobra.Command, flags *apiFlags) error {
	idx, err := LoadIndex(cmd.Context(), refreshSpecFlag(cmd))
	if err != nil {
		return err
	}
	if InteractiveRunner == nil {
		return NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			"interactive TUI not linked into this binary")
	}
	cfg := TUIConfig{Org: flags.org, Project: flags.project, Stack: flags.stack}
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
// named path parameter, using the same precedence the non-interactive code
// applies: explicit flag > currently-selected stack's qualified ref >
// project file / default org.
func InteractiveDefault(name string, cfg TUIConfig) string {
	stackOrg, stackProj, stackName := currentStackSelection()

	switch templateVarKind(name) {
	case kindOrg:
		if cfg.Org != "" {
			return cfg.Org
		}
		if stackOrg != "" {
			return stackOrg
		}
		if org, err := pkgWorkspace.GetBackendConfigDefaultOrg(nil); err == nil {
			return org
		}
	case kindProject:
		if cfg.Project != "" {
			return cfg.Project
		}
		if proj, _, err := pkgWorkspace.Instance.ReadProject(); err == nil && proj != nil {
			return string(proj.Name)
		}
		if stackProj != "" {
			return stackProj
		}
	case kindStack:
		if cfg.Stack != "" {
			return cfg.Stack
		}
		if stackName != "" {
			return stackName
		}
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
