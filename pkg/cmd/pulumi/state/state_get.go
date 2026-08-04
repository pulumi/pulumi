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

package state

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/autonames"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type stateGetRenderFunc func(cmd *cobra.Command, res *pkgresource.State, showSecrets bool) error

func newStateGetCommand(ws pkgWorkspace.Context, lm cmdBackend.LoginManager) *cobra.Command {
	var stackName string
	var showSecrets bool

	output := outputflag.OutputFlag[stateGetRenderFunc]{
		RenderForTerminal: renderResourceStateText,
		RenderJSON:        renderResourceStateJSON,
	}

	cmd := &cobra.Command{
		Use:   "get [resource]",
		Short: "Show a resource's state",
		Long: `Show a resource's state

Display the state tracked for a single resource in a stack's state, including its URN,
ID, inputs and outputs. The resource may be referenced by its URN or by the identifier
auto-assigned to it (as listed by "pulumi do --resources"). If no resource is given,
this command will prompt for one.`,
		Example: "pulumi state get myBucket\n" +
			"pulumi state get 'urn:pulumi:stage::demo::aws:s3/bucket:Bucket::myBucket'",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			sink := cmdutil.Diag()
			opts := display.Options{Color: cmdutil.GetGlobalColorization()}

			s, err := cmdStack.RequireStack(ctx, sink, ws, lm, stackName, cmdStack.LoadOnly, opts, "")
			if err != nil {
				return fmt.Errorf("load stack: %w", err)
			}
			snap, err := s.Snapshot(ctx, secrets.DefaultProvider)
			if err != nil {
				return fmt.Errorf("load stack snapshot: %w", err)
			}
			if snap == nil {
				return fmt.Errorf("no state found for stack %s", s.Ref())
			}

			var res *pkgresource.State
			if len(args) == 0 {
				if !cmdutil.Interactive() {
					return missingNonInteractiveArg("resource")
				}
				urn, err := getURNFromState(ctx, sink, ws, lm, stackName, &snap,
					"Select the resource to show")
				if err != nil {
					return fmt.Errorf("failed to select resource: %w", err)
				}
				res, err = locateStackResource(opts, snap, urn)
				if err != nil {
					return err
				}
			} else {
				res, err = resolveResourceRef(snap, args[0], cmd.ErrOrStderr())
				if err != nil {
					return err
				}
			}

			return output.Get()(cmd, res, showSecrets)
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "resource"},
		},
		Required: 0,
	})

	cmd.PersistentFlags().StringVarP(
		&stackName, "stack", "s", "",
		"The name of the stack to operate on. Defaults to the current stack")
	outputflag.VarP(cmd.Flags(), &output)
	cmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Show secret values in plaintext instead of [secret]")
	return cmd
}

// resolveResourceRef resolves a CLI argument to a resource in the snapshot. A valid URN is looked
// up directly; anything else is treated as an auto-assigned resource reference identifier, and
// failing that, as a resource's own name when that name uniquely identifies a resource.
func resolveResourceRef(snap *deploy.Snapshot, arg string, warn io.Writer) (*pkgresource.State, error) {
	if urn := resource.URN(arg); urn.IsValid() {
		candidates := edit.LocateResource(snap, urn)
		if len(candidates) == 0 {
			return nil, resourceNotFoundError(snapshotURNs(snap), urn)
		}
		var live []*pkgresource.State
		for _, c := range candidates {
			if !c.Delete {
				live = append(live, c)
			}
		}
		if len(live) == 0 {
			return candidates[0], nil
		}
		if pending := len(candidates) - len(live); pending > 0 {
			fmt.Fprintf(warn, "warning: %d copy(ies) of this resource are pending deletion; "+
				"showing the live resource (`pulumi stack export` lists all copies)\n", pending)
		}
		return live[0], nil
	}
	if strings.HasPrefix(arg, "urn:") {
		return nil, fmt.Errorf("%q is not a valid resource URN\n%s", arg, listURNsHint(""))
	}

	refURN, isRef := autonames.ResourceNames(snap)[arg]
	named := namedResourceURNs(snap, arg)

	if isRef {
		var others []string
		for _, urn := range named {
			if string(urn) != refURN {
				others = append(others, string(urn))
			}
		}
		if len(others) > 0 {
			return nil, fmt.Errorf("%q is ambiguous: it is the reference identifier for\n    %s\n"+
				"but also the name of\n    %s\nspecify the resource by URN instead",
				arg, refURN, strings.Join(others, "\n    "))
		}
		return resolveResourceRef(snap, refURN, warn)
	}
	switch len(named) {
	case 0:
		return nil, fmt.Errorf("no resource identified by %q found in the stack; "+
			"run `pulumi do --resources` to list resource reference identifiers", arg)
	case 1:
		return resolveResourceRef(snap, string(named[0]), warn)
	default:
		urns := make([]string, len(named))
		for i, urn := range named {
			urns[i] = string(urn)
		}
		return nil, fmt.Errorf("%d resources in the stack are named %q:\n    %s\n"+
			"specify one by URN",
			len(named), arg, strings.Join(urns, "\n    "))
	}
}

// namedResourceURNs returns the distinct URNs of the snapshot's resources whose name is exactly
// name, in snapshot order.
func namedResourceURNs(snap *deploy.Snapshot, name string) []resource.URN {
	var urns []resource.URN
	seen := map[resource.URN]bool{}
	for _, s := range snap.Resources {
		if s == nil || seen[s.URN] || s.URN.Name() != name {
			continue
		}
		seen[s.URN] = true
		urns = append(urns, s.URN)
	}
	return urns
}

func renderResourceStateJSON(cmd *cobra.Command, res *pkgresource.State, showSecrets bool) error {
	inputs, err := renderProperties(cmd.Context(), res.Inputs, showSecrets)
	if err != nil {
		return err
	}
	outputs, err := renderProperties(cmd.Context(), res.Outputs, showSecrets)
	if err != nil {
		return err
	}
	envelope := map[string]any{
		"urn":     res.URN,
		"id":      res.ID,
		"type":    res.Type,
		"inputs":  inputs,
		"outputs": outputs,
	}
	if res.Delete {
		envelope["pendingDeletion"] = true
	}
	return ui.FprintJSON(cmd.OutOrStdout(), envelope)
}

func renderResourceStateText(cmd *cobra.Command, res *pkgresource.State, showSecrets bool) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Type: %s\n", res.Type)
	fmt.Fprintf(w, "URN:  %s\n", res.URN)
	if res.ID != "" {
		fmt.Fprintf(w, "ID:   %s\n", res.ID)
	}
	if res.Delete {
		fmt.Fprintf(w, "Pending deletion: yes\n")
	}
	color := cmdutil.GetGlobalColorization()
	for _, section := range []struct {
		title string
		props resource.PropertyMap
	}{
		{"Inputs", res.Inputs},
		{"Outputs", res.Outputs},
	} {
		if len(section.props) == 0 {
			continue
		}
		var buf bytes.Buffer
		display.PrintObject(&buf, section.props, false /* planning */, 1 /* indent */, deploy.OpSame,
			false /* prefix */, false /* truncateOutput */, false /* debug */, showSecrets)
		fmt.Fprintf(w, "%s:\n%s\n", section.title, color.Colorize(buf.String()))
	}
	return nil
}

// renderProperties converts a property map to plain JSON-marshalable values via the same
// MassageSecrets+SerializeProperties path as `pulumi stack output`, after dropping internal
// (double-underscore) keys.
func renderProperties(ctx context.Context, props resource.PropertyMap, showSecrets bool) (map[string]any, error) {
	visible := make(resource.PropertyMap, len(props))
	for k, v := range props {
		if !resource.IsInternalPropertyKey(k) {
			visible[k] = v
		}
	}
	return stack.SerializeProperties(ctx, display.MassageSecrets(visible, showSecrets),
		config.NewPanicCrypter(), showSecrets)
}
