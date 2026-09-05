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

package do

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemainfo"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

func resourceSchemaHelp(res *schema.Resource) string {
	color := cmdutil.GetGlobalColorization()
	var b strings.Builder
	writeSection := func(title string, properties []*schema.Property, kind schemainfo.Kind) {
		if b.Len() > 0 {
			// WriteProperties output ends in a newline; add one more to separate sections.
			b.WriteByte('\n')
		}
		schemainfo.WriteProperties(&b, color, title, schemainfo.BoundProperties(properties), kind)
	}

	writeSection("Inputs", res.InputProperties, schemainfo.Inputs)
	writeSection("Outputs", res.Properties, schemainfo.Outputs)
	if res.ListInputs != nil && len(res.ListInputs.Properties) > 0 {
		writeSection("List Inputs", res.ListInputs.Properties, schemainfo.ListInputs)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (pc *packageCommand) newResourceCommand(res *schema.Resource) *cobra.Command {
	_, _, name, diags := pcl.DecomposeToken(res.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")

	shorthelp := fmt.Sprintf("Operate on the %s resource", name)
	longhelp := shorthelp + "."
	if description := schemainfo.RenderDescription(res.Comment); description != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, description)
	}
	if schemaHelp := resourceSchemaHelp(res); schemaHelp != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, schemaHelp)
	}
	if len(res.InputProperties) > 0 {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, inputFlagsHelp)
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: shorthelp,
		Long:  longhelp,
		Args:  cobra.NoArgs,
	}
	// Provider configuration applies to all sub-operations, so register here as persistent flags.
	cmd.PersistentFlags().StringVar(&pc.providerFile, "provider-file", "",
		"Path to a file containing provider configuration")
	cmd.PersistentFlags().StringVar(&pc.format, "input", "yaml",
		"Format of the provider configuration file")
	cmd.PersistentFlags().StringVar(&pc.providerURN, "provider", "",
		"The URN of a provider resource in the current stack whose inputs to use as the "+
			"base of the provider configuration (requires a stack context)")
	addPersistentInputFlags(cmd, pc.spec.Name(), pc.providerDef.InputProperties)
	// `create`/`upsert`/`patch` have different UX between stateful (takes a resource <name> and adds a
	// snippet to the stack) and stateless (uses the resource type's short name and calls the
	// provider directly), so the command trees diverge here.
	if pc.stateless {
		cmd.AddCommand(pc.newStatelessResourceCreateCommand(res))
		cmd.AddCommand(pc.newStatelessResourceUpsertCommand(res))
		cmd.AddCommand(pc.newStatelessResourcePatchCommand(res))
	} else {
		cmd.AddCommand(pc.newStatefulResourceCreateCommand(res))
		cmd.AddCommand(pc.newStatefulResourceUpsertCommand(res))
		cmd.AddCommand(pc.newStatefulResourcePatchCommand(res))
	}
	cmd.AddCommand(pc.newResourceReadCommand(res))
	cmd.AddCommand(pc.newResourceDeleteCommand(res))
	if res.ListInputs != nil {
		cmd.AddCommand(pc.newResourceListCommand(res))
	}
	return cmd
}

// newStatefulResourceCreateCommand adds a snippet to the current stack and runs the deployment
// engine targeting only that snippet. Errors if a snippet with the same (Name, Type) already
// exists — `upsert` is the command for replacing one in place.
func (pc *packageCommand) newStatefulResourceCreateCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
	var resourcesFile string
	var yes bool
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a resource",
		Long: "Create a resource.\n\n" +
			"The created resource is tracked in the stack, so Pulumi can manage its lifecycle. " +
			"Fails if a resource with the given name already exists — use `upsert` to replace " +
			"one in place.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract.Assertf(!pc.stateless, "stateful create should not be registered in stateless mode")
			return pc.runStatefulSnippetUpdate(cmd, statefulSnippetUpdate{
				res:           res,
				name:          args[0],
				inputFile:     inputFile,
				inputFormat:   inputFormat,
				resourcesFile: resourcesFile,
				yes:           yes,
				requireFresh:  true,
			})
		},
	}
	addStatefulSnippetUpdateFlags(cmd, &inputFile, &inputFormat, &resourcesFile, &yes, res.InputProperties)
	return cmd
}

func (pc *packageCommand) newStatelessResourceCreateCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var yes bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			contract.Assertf(pc.stateless, "stateless create should not be registered in stateful mode")
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			return pc.runStatelessCreate(cmd, res, yes, func() (resource.PropertyMap, error) {
				if err := pc.configureProvider(cmd, ctx); err != nil {
					return nil, err
				}
				inputs, err := evaluateResourceFile(
					ctx, inputFile, "input", pc.format, res, pc.evalContext(),
					pc.converter, pc.loaderTarget, pc.packageDescriptor,
					collectInputFlags(cmd, "input", res.InputProperties))
				if err != nil {
					return nil, fmt.Errorf("parse input file: %w", err)
				}
				return inputs, nil
			})
		},
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", res.InputProperties)
	return cmd
}

func (pc *packageCommand) runStatelessCreate(
	cmd *cobra.Command, res *schema.Resource, yes bool,
	prepareInputs func() (resource.PropertyMap, error),
) error {
	ctx := cmd.Context()
	urn := resourceURN(res)
	var checked resource.PropertyMap
	prepare := func() (*pkgresource.State, error) {
		inputs, err := prepareInputs()
		if err != nil {
			return nil, err
		}
		checked, err = pc.checkResourceInputs(ctx, urn, res, nil, inputs)
		if err != nil {
			return nil, err
		}
		return operationState(urn, "", checked, nil), nil
	}
	create := func() (*pkgresource.State, error) {
		response, err := pc.provider.Create(ctx, plugin.CreateRequest{
			URN:        urn,
			Name:       urn.Name(),
			Type:       urn.Type(),
			Properties: checked,
			Preview:    pc.dryrun,
		})
		if err != nil {
			return nil, err
		}
		id := response.ID
		if id == "" {
			id = resource.ID("[unknown]")
		}
		return resultState(urn, id, nil, response.Properties, res), nil
	}
	if pc.dryrun {
		return pc.runDisplayedStep(cmd, displayedStep{
			Op:  deploy.OpCreate,
			New: operationState(urn, "", nil, nil),
		}, func() (*pkgresource.State, error) {
			if _, err := prepare(); err != nil {
				return nil, err
			}
			return create()
		})
	}
	if err := pc.runDisplayedStep(cmd, displayedStep{
		Op:      deploy.OpCreate,
		New:     operationState(urn, "", nil, nil),
		Preview: true,
	}, prepare); err != nil {
		return err
	}
	if err := pc.confirm(cmd, "", "create", yes); err != nil {
		return err
	}
	return pc.runDisplayedStep(cmd, displayedStep{
		Op:  deploy.OpCreate,
		New: operationState(urn, "", checked, nil),
	}, create)
}

func (pc *packageCommand) newResourceReadCommand(res *schema.Resource) *cobra.Command {
	return &cobra.Command{
		Use:   "read <id>",
		Short: "Read a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := pc.configureProvider(cmd, ctx); err != nil {
				return err
			}
			urn := resourceURN(res)
			id := resource.ID(args[0])
			return pc.runDisplayedStep(cmd, displayedStep{
				Op:  deploy.OpRead,
				New: operationState(urn, id, nil, nil),
			}, func() (*pkgresource.State, error) {
				response, err := pc.provider.Read(ctx, plugin.ReadRequest{
					URN:    urn,
					Name:   urn.Name(),
					Type:   urn.Type(),
					ID:     id,
					Inputs: resource.PropertyMap{},
					State:  resource.PropertyMap{},
				})
				if err != nil {
					return nil, err
				}
				verdict := classifyRead(response)
				logReadVerdict(ctx, "read", res.Token, resource.ID(args[0]), verdict)
				if verdict.missing() {
					return nil, errResourceNotFound(args[0])
				}
				if response.ID != "" {
					id = response.ID
				}
				return resultState(urn, id, nil, response.Outputs, res), nil
			})
		},
	}
}

// newStatefulResourcePatchCommand patches the existing snippet for (name, res.Token) by
// overriding only the top-level PCL attributes the user supplies. The rest of the snippet's
// code — other attributes, options blocks, comments — and its References/Descriptor are
// preserved verbatim.
func (pc *packageCommand) newStatefulResourcePatchCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
	var resourcesFile string
	var yes bool
	cmd := &cobra.Command{
		Use:   "patch <name>",
		Short: "Patch a resource",
		Long: "Patch a resource.\n\n" +
			"Only the inputs supplied here override the corresponding attributes in the existing " +
			"resource snippet; all other snippet content is preserved. Fails if no resource with " +
			"the given name exists.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract.Assertf(!pc.stateless, "stateful patch should not be registered in stateless mode")
			return pc.runStatefulSnippetPatch(cmd, res, args[0], inputFile, inputFormat, resourcesFile, yes)
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "yaml",
		"Format of the resource inputs file (any language name supported by an installed converter)")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().StringVar(&resourcesFile, "resources-file", "",
		"Path to a JSON file mapping identifiers to resource URNs that input expressions may reference")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", res.InputProperties)
	return cmd
}

func (pc *packageCommand) newStatelessResourcePatchCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
	var yes bool
	cmd := &cobra.Command{
		Use:   "patch <id>",
		Short: "Patch a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			contract.Assertf(pc.stateless, "stateless patch should not be registered in stateful mode")
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := pc.configureProvider(cmd, ctx); err != nil {
				return err
			}
			urn := resourceURN(res)
			id := resource.ID(args[0])
			read, err := pc.provider.Read(ctx, plugin.ReadRequest{
				URN:    urn,
				Name:   urn.Name(),
				Type:   urn.Type(),
				ID:     id,
				Inputs: resource.PropertyMap{},
				State:  resource.PropertyMap{},
			})
			if err != nil {
				return err
			}
			verdict := classifyRead(read)
			logReadVerdict(ctx, "patch", res.Token, id, verdict)
			if verdict.missing() {
				return errResourceNotFound(args[0])
			}
			// AllowMissingProperties because a patch typically only specifies the fields being changed; the binder
			// would otherwise reject any partial patch that omits a required input.
			patch, err := evaluateResourceFile(
				ctx, inputFile, "input", inputFormat, res, pc.evalContext(),
				pc.converter, pc.loaderTarget, pc.packageDescriptor,
				collectInputFlags(cmd, "input", res.InputProperties), pcl.AllowMissingProperties)
			if err != nil {
				return fmt.Errorf("parse input file: %w", err)
			}

			newInputs := read.Inputs.Copy()
			maps.Copy(newInputs, patch)
			return pc.runStatelessUpdate(cmd, res, id, read, newInputs, "patch", reportMissing, yes)
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "yaml", "Format of the configuration files")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", res.InputProperties)
	return cmd
}

// missingPolicy says what an operation should report when its provider call fails and the resource
// turns out to have been deleted underneath it.
type missingPolicy bool

const (
	// reportMissing suits operations that have nothing left to do once the resource is gone, so a
	// resource that vanished mid-flight is reported exactly as the pre-flight Read would have
	// reported it: not found.
	reportMissing missingPolicy = true
	// keepFailure suits operations for which "not found" is not a meaningful outcome — upsert would
	// have created the resource had it known, so answering not-found would tell a caller to stop
	// when the correct response is to run again and create it.
	keepFailure missingPolicy = false
)

func (pc *packageCommand) runStatelessUpdate(
	cmd *cobra.Command, res *schema.Resource, id resource.ID,
	read plugin.ReadResponse, newInputs resource.PropertyMap, operation string,
	missing missingPolicy, yes bool,
) error {
	ctx := cmd.Context()
	urn := resourceURN(res)
	oldInputs := read.Inputs
	checked, err := pc.checkResourceInputs(ctx, urn, res, oldInputs, newInputs)
	if err != nil {
		return err
	}

	diff, err := pc.provider.Diff(ctx, plugin.DiffRequest{
		URN:        urn,
		Name:       urn.Name(),
		Type:       urn.Type(),
		ID:         id,
		OldInputs:  oldInputs,
		OldOutputs: read.Outputs,
		NewInputs:  checked,
	})
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	summary := formatPatchSummary(
		res, id, oldInputs, checked, diff, pc.showSecrets, cmdutil.GetGlobalColorization())
	if err := pc.confirm(cmd, summary, operation, yes); err != nil {
		return err
	}

	return pc.runDisplayedStep(cmd, displayedStep{
		Op:           deploy.OpUpdate,
		Old:          operationState(urn, id, oldInputs, read.Outputs),
		New:          operationState(urn, id, checked, nil),
		Diffs:        diff.ChangedKeys,
		DetailedDiff: diff.DetailedDiff,
	}, func() (*pkgresource.State, error) {
		response, err := pc.provider.Update(ctx, plugin.UpdateRequest{
			URN:        urn,
			Name:       urn.Name(),
			Type:       urn.Type(),
			ID:         id,
			OldInputs:  oldInputs,
			OldOutputs: read.Outputs,
			NewInputs:  checked,
			Preview:    pc.dryrun,
		})
		if err != nil {
			// Same race as the delete path: the resource can be removed between our pre-flight Read
			// and this call, leaving a provider error a caller cannot classify.
			if missing == reportMissing && pc.resourceGoneAfterFailure(ctx, operation, urn, id) {
				return nil, errResourceNotFound(string(id))
			}
			return nil, err
		}
		return resultState(urn, id, checked, response.Properties, res), nil
	})
}

func (pc *packageCommand) newResourceDeleteCommand(res *schema.Resource) *cobra.Command {
	var yes bool
	use := "delete <id>"
	if !pc.stateless {
		use = "delete <name>"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: "Delete a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !pc.stateless {
				return pc.runStatefulSnippetDelete(cmd, res, args[0], yes)
			}
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := pc.configureProvider(cmd, ctx); err != nil {
				return err
			}
			urn := resourceURN(res)

			// First we need to read the resource. The ID given here is an "import id", while the actual
			// Delete call needs the real ID + any inputs/outputs. terraform-pf bridge for example will fail to
			// delete if just passed the ID and no state.
			response, err := pc.provider.Read(ctx, plugin.ReadRequest{
				URN:    urn,
				Name:   urn.Name(),
				Type:   urn.Type(),
				ID:     resource.ID(args[0]),
				Inputs: resource.PropertyMap{},
				State:  resource.PropertyMap{},
			})
			if err != nil {
				return err
			}
			verdict := classifyRead(response)
			logReadVerdict(ctx, "delete", res.Token, resource.ID(args[0]), verdict)
			if verdict.missing() {
				return errResourceNotFound(args[0])
			}
			id := response.ID
			if id == "" {
				id = resource.ID(args[0])
			}

			if err := pc.confirm(cmd, formatDeleteSummary(res, id, pc.dryrun), string(id), yes); err != nil {
				return err
			}
			// The provider protocol has no preview mode for Delete, so the summary above is the whole dry run.
			if pc.dryrun {
				return nil
			}

			return pc.runDisplayedStep(cmd, displayedStep{
				Op:  deploy.OpDelete,
				Old: operationState(urn, id, nil, nil),
			}, func() (*pkgresource.State, error) {
				_, err := pc.provider.Delete(ctx, plugin.DeleteRequest{
					URN:     urn,
					Name:    urn.Name(),
					Type:    urn.Type(),
					ID:      id,
					Inputs:  response.Inputs,
					Outputs: response.Outputs,
				})
				if err != nil && pc.resourceGoneAfterFailure(ctx, "delete", urn, id) {
					return nil, errResourceNotFound(args[0])
				}
				return nil, err
			})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	return cmd
}

func (pc *packageCommand) newResourceListCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
	var all bool
	var count int64
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List resources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && count > 0 {
				return errors.New("--all and --count are mutually exclusive")
			}
			ctx := cmd.Context()
			listing := startSpinner(fmt.Sprintf("Listing %s resources", res.Token))
			defer listing()
			if err := pc.configureProvider(cmd, ctx); err != nil {
				return err
			}

			query, err := evaluateResourceListFile(
				ctx, inputFile, "input", inputFormat, res, pc.evalContext(),
				pc.converter, pc.loaderTarget, pc.packageDescriptor,
				collectInputFlags(cmd, "input", res.ListInputs.Properties))
			if err != nil {
				return fmt.Errorf("parse input file: %w", err)
			}

			var results []plugin.ListResult
			var continuation string
			for {
				limit := int64(0)
				if count > 0 {
					limit = count - int64(len(results))
				}
				stream, err := pc.provider.List(ctx, plugin.ListRequest{
					Token:             tokens.Type(res.Token),
					Query:             resource.FromResourcePropertyMap(query),
					Limit:             limit,
					ContinuationToken: continuation,
				})
				if err != nil {
					return err
				}
				for item, err := range stream.Items {
					if err != nil {
						return err
					}
					results = append(results, item)
				}
				if stream.Computed {
					listing()
					output, err := jsonifyProperty(resource.NewProperty("<unknown>"), pc.showSecrets)
					if err != nil {
						return err
					}
					fmt.Fprint(cmd.OutOrStdout(), output)
					return nil
				}
				continuation = stream.ContinuationToken
				if count > 0 && int64(len(results)) >= count {
					results = results[:int(count)]
					break
				}
				if continuation == "" {
					break
				}
				if count == 0 && !all {
					break
				}
			}

			listing()
			return pc.printListResults(cmd, results)
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "yaml", "Input file format")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource list inputs")
	cmd.Flags().BoolVar(&all, "all", false, "Enumerate all matching resources")
	cmd.Flags().Int64Var(&count, "count", 0, "Enumerate up to count matching resources")
	addInputFlags(cmd, "input", res.ListInputs.Properties)
	return cmd
}

func evaluateResourceListFile(
	ctx context.Context, path, fileType, inputFormat string, res *schema.Resource, evalContext functionEvalContext,
	loadConverter func(string) (plugin.Converter, error), loaderTarget string,
	packageDescriptor *codegenrpc.GetSchemaRequest,
	inputFlags map[string]inputFlagValue,
) (resource.PropertyMap, error) {
	contract.Assertf(res.ListInputs != nil, "should not call evaluateResourceListFile for resources without list inputs")

	bind := func(file *hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics) {
		attrs, inputType, diags := pcl.BindResourceList(ctx, file, res)
		return attrs, inputType, res.ListInputs.Properties, diags
	}
	return evaluateFile(
		ctx, path, fileType, inputFormat, res.Token, bind, loadConverter, loaderTarget, packageDescriptor, evalContext,
		inputFlags,
	)
}

func (pc *packageCommand) checkResourceInputs(
	ctx context.Context, urn resource.URN, res *schema.Resource, olds, news resource.PropertyMap,
) (resource.PropertyMap, error) {
	checked, err := pc.provider.Check(ctx, plugin.CheckRequest{
		URN:  urn,
		Type: tokens.Type(res.Token),
		Olds: olds,
		News: news,
	})
	if err != nil {
		return nil, err
	}
	if len(checked.Failures) > 0 {
		var b strings.Builder
		b.WriteString("resource inputs failed validation:")
		for _, failure := range checked.Failures {
			fmt.Fprintf(&b, "\n- %s: %s", failure.Property, failure.Reason)
		}
		return nil, fmt.Errorf("%s", b.String())
	}
	return checked.Properties, nil
}

// readNotFound reports whether a Read response describes a resource that is no longer there.
//
// Providers signal absence in more than one shape. The obvious ones are a nil state bag or a blank
// ID. Bridged providers add a third: when the refresh behind Read 404s they warn ("Automatically
// removing from Terraform State"), drop the state, and still echo the requested import ID back,
// leaving a non-blank ID paired with an empty property bag. Reading that as "found" is what makes a
// repeated delete of an already-deleted resource issue a real Delete against emptied state, which
// the provider rejects with its own error (`MissingParameter: groupName or groupId`) instead of the
// not-found a caller can branch on.
//
// So an ID on its own does not count as found. Delete and Update both need the resource's real
// inputs/outputs — as the Read-before-Delete comment above notes, the terraform-pf bridge fails
// outright when handed an ID and no state — so a response carrying no state is one we could not act
// on even if the resource did exist.
//
// Explicitly NOT covered: the converse half of #23916, where `aws:cloudwatch/eventTarget` reports a
// resource that is still there as missing. Its create hands back an ID joined with a dash
// (`<rule>-<target>`) while its Read expects the slash form, so Read is handed an ID it cannot
// resolve, legitimately finds nothing, and we correctly conclude "not found" from what we were
// given. That is an ID-format bug on the provider side and is invisible from here.
//
// Mind the direction before extending this function to chase it: that failure is a resource we
// skipped deleting, so anything that widens not-found detection makes it *worse*, not better —
// more live resources silently orphaned, which is the more damaging of the two failure modes. The
// two halves of that issue pull in opposite directions and only one of them lives in this file.
func readNotFound(read plugin.ReadResponse) bool {
	return classifyRead(read).missing()
}

// Stable identifiers for the structured records this package emits. Consumers key off these rather
// than off message text, which is not a compatibility surface.
const (
	// eventReadVerdict is emitted once per pre-flight Read, whatever the outcome, so a log pipeline
	// can see the normal paths as well as the missing ones.
	eventReadVerdict = "pulumi.do.read.verdict"
	// eventRecheck is emitted only when a Delete or Update failed and resourceGoneAfterFailure ran,
	// which makes its presence the signal that the re-read path fired.
	eventRecheck = "pulumi.do.mutation.recheck"
)

// readVerdict records which rule decided whether a Read described a live resource. Keeping the
// reason rather than a bare bool is what lets the logs distinguish an ordinary absence (the
// provider blanked the ID) from the #23916 shape (an echoed ID over an empty bag), which are the
// same outcome but very different provider behaviour.
type readVerdict string

const (
	readPresent readVerdict = "present"
	// readBlankID is the shape a well-behaved provider returns for a missing resource.
	readBlankID readVerdict = "blank-id"
	// readNilState is a provider that returned no property bag at all.
	readNilState readVerdict = "nil-state"
	// readEmptyState is the #23916 shape: a non-blank ID over a bag with nothing in it. A spike
	// here is the signal that a provider is echoing IDs for resources that are gone.
	readEmptyState readVerdict = "empty-state"
)

func (v readVerdict) missing() bool { return v != readPresent }

// classifyRead is the decision behind readNotFound, split out so the reason survives for logging
// and so the rules can be tested exhaustively without going through a command.
//
// The three rules do not rest on equally firm ground, which is worth knowing before changing them:
//
//   - readNilState is the documented contract. plugin.ReadResult.Outputs says "if this field is
//     nil, the resource does not exist", and Provider.Read repeats it. Any provider, any version.
//   - readBlankID is undocumented — ReadResult.ID claims it "will always be populated" — but it is
//     what the engine's refresh has always done (see RefreshStep.Apply, "if the ID is blank treat
//     this as a delete"). A provider that violated it would have visibly broken `pulumi refresh`
//     long ago, so it is safe in practice even though the doc comment disagrees.
//   - readEmptyState is neither documented nor used by the engine. It is inferred from observed
//     bridge behaviour and is the one assumption here that could misfire: it reads a live resource
//     that genuinely has no properties as missing. Note that it can only trigger where a provider
//     has already broken the documented contract by returning non-nil Outputs for a resource that
//     is gone — for a conforming provider this branch is unreachable on the missing path.
//
// None of this keys on a provider name or version; the rules are about response shape only.
func classifyRead(read plugin.ReadResponse) readVerdict {
	switch {
	case read.ID == "":
		return readBlankID
	case read.Outputs == nil:
		return readNilState
	case !hasResourceState(read.Inputs) && !hasResourceState(read.Outputs):
		return readEmptyState
	default:
		return readPresent
	}
}

// logReadVerdict records how a pre-flight Read was classified. Emitted on every operation that
// reads before acting, present or missing, so the absence of a record is never ambiguous.
func logReadVerdict(ctx context.Context, operation, resourceType string, id resource.ID, v readVerdict) {
	slog.InfoContext(ctx, "do: classified resource read",
		"event", eventReadVerdict,
		"operation", operation,
		"resourceType", resourceType,
		"resourceId", string(id),
		"verdict", string(v),
		"missing", v.missing(),
	)
}

// resourceNotFoundError reports that the resource an operation targeted is not there.
//
// It carries cmdCmd.ExitResourceNotFound so a caller can classify the outcome from the process exit
// code rather than by matching error text, which is not a stable interface across releases — the
// distinction between "already gone, stop retrying" and "this failed, retry" is exactly what a
// reconcile loop needs and exactly what a shared exit code of 1 destroys.
//
// The message is byte-for-byte what the plain fmt.Errorf produced before, so callers already
// matching on the text keep working; the exit code is additive.
type resourceNotFoundError struct {
	id string
}

func (e resourceNotFoundError) Error() string {
	return fmt.Sprintf("resource %q was not found", e.id)
}

// CustomExitCode implements cmdCmd.CustomExitCodeError.
func (e resourceNotFoundError) CustomExitCode() int {
	return cmdCmd.ExitResourceNotFound
}

// errResourceNotFound builds the not-found error every operation in this package reports, so the
// message and the exit code stay in step across all of them.
func errResourceNotFound(id string) error {
	return resourceNotFoundError{id: id}
}

// resourceGoneAfterFailure re-reads a resource whose Delete or Update has just failed, to tell "the
// operation failed" apart from "something else deleted it in between". Callers that retry on their
// own timers can have two invocations in flight at once: both clear the pre-flight Read, one
// deletes, and the other gets whatever the provider says about operating on an object that is
// already gone — a message with no not-found marker to branch on, which is the same classification
// problem readNotFound solves for the emptied-read case.
//
// The bound here is structural rather than a retry budget: this issues exactly one Read and never
// re-attempts the failed operation, so an invocation makes at most three provider calls (Read,
// Delete or Update, Read) no matter what any of them return. There is no edge back to the mutation,
// so there is no loop to bound.
//
// The failed operation's error is the default answer; this only overrides it on positive evidence of
// absence. If the re-read errors, or comes back carrying any state at all — including the partial
// state an eventually-consistent backend may serve while it settles — the original error is
// reported unchanged. A caller retrying on a timer then gets the clean not-found from a later
// invocation's pre-flight Read once the backend has converged, which is the right place for that
// wait: the caller already has a retry policy, and a second one nested inside a single invocation
// would only fight it.
func (pc *packageCommand) resourceGoneAfterFailure(
	ctx context.Context, operation string, urn resource.URN, id resource.ID,
) bool {
	// One record per re-read, covering every outcome. Emitting on the failure paths too is what
	// makes this usable as a rate: a rising share of "read-failed" means the re-read is not
	// answering the question, not that the race stopped happening.
	log := func(outcome string, v readVerdict, detail error) {
		attrs := []any{
			"event", eventRecheck,
			"operation", operation,
			"resourceType", string(urn.Type()),
			"resourceId", string(id),
			"outcome", outcome,
			"reclassified", outcome == "gone",
		}
		if v != "" {
			attrs = append(attrs, "verdict", string(v))
		}
		if detail != nil {
			attrs = append(attrs, "err", detail.Error())
		}
		slog.InfoContext(ctx, "do: re-read resource after failed mutation", attrs...)
	}

	// An incident lever, not a circuit breaker — a breaker cannot work here, because each
	// invocation is one process performing one mutation and so issues at most one re-read; there is
	// no sequence of failures for in-process state to observe. What this does buy is the ability to
	// shed the extra call fleet-wide, without redeploying, while a provider is rate limiting and
	// the re-read is likely to be throttled too (adding load and returning no classification).
	// Still logged, so a dashboard shows the switch being thrown rather than going quiet in a way
	// that looks like the race stopped happening.
	if env.Global().GetBool(env.DoSkipRecheck) {
		log("skipped", "", nil)
		return false
	}

	response, err := pc.provider.Read(ctx, plugin.ReadRequest{
		URN:    urn,
		Name:   urn.Name(),
		Type:   urn.Type(),
		ID:     id,
		Inputs: resource.PropertyMap{},
		State:  resource.PropertyMap{},
	})
	if err != nil {
		log("read-failed", "", err)
		return false
	}
	verdict := classifyRead(response)
	if verdict.missing() {
		log("gone", verdict, nil)
		return true
	}
	log("present", verdict, nil)
	return false
}

// hasResourceState reports whether a property bag carries anything beyond the resource's own ID.
// A lone "id" is ignored: it just duplicates ReadResponse.ID and says nothing about whether the
// remote object is still there.
func hasResourceState(props resource.PropertyMap) bool {
	for key := range props {
		if key != "id" {
			return true
		}
	}
	return false
}

func resultOutputs(id resource.ID, outputs resource.PropertyMap, res *schema.Resource) resource.PropertyMap {
	contract.Requiref(id != "", "id", "id should not be blank")
	if res.Properties != nil {
		outputs = filterOutputs(outputs, res.Properties)
	} else {
		outputs = outputs.Copy()
	}
	outputs["id"] = resource.NewProperty(string(id))
	return outputs
}

func resultState(
	urn resource.URN, id resource.ID, inputs, outputs resource.PropertyMap, res *schema.Resource,
) *pkgresource.State {
	return operationState(urn, id, inputs, resultOutputs(id, outputs, res))
}

func (pc *packageCommand) printResourceResult(cmd *cobra.Command, state *pkgresource.State) error {
	output, err := jsonifyProperty(resource.NewProperty(state.Outputs), pc.showSecrets)
	if err != nil {
		return fmt.Errorf("failed to convert outputs to JSON: %w", err)
	}
	fmt.Fprint(cmd.OutOrStdout(), output)
	return nil
}

func (pc *packageCommand) printListResults(cmd *cobra.Command, results []plugin.ListResult) error {
	values := make([]resource.PropertyValue, len(results))
	for i, result := range results {
		values[i] = resource.NewProperty(resource.PropertyMap{
			"id":   resource.NewProperty(string(result.ID)),
			"name": resource.NewProperty(result.Name),
		})
	}
	output, err := jsonifyProperty(resource.NewProperty(values), pc.showSecrets)
	if err != nil {
		return fmt.Errorf("failed to convert outputs to JSON: %w", err)
	}
	fmt.Fprint(cmd.OutOrStdout(), output)
	return nil
}

func formatDeleteSummary(res *schema.Resource, id resource.ID, dryrun bool) string {
	if dryrun {
		return fmt.Sprintf("This would delete %s %q.", res.Token, id)
	}
	return fmt.Sprintf("This will delete %s %q.", res.Token, id)
}

// formatPatchSummary renders a human-readable summary of the changes a patch will apply. The value-level diff is
// produced by display.PrintObjectDiff — the same renderer the engine uses for `pulumi up` / `pulumi preview` —
// so the output is shaped identically (e.g. "  ~ name: \"old\" => \"new\""). The provider's DiffResult informs
// the "no changes" shortcut and the replacement notice.
func formatPatchSummary(
	res *schema.Resource, id resource.ID,
	oldInputs, newInputs resource.PropertyMap,
	providerDiff plugin.DiffResult,
	showSecrets bool, color colors.Colorization,
) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This will update %s %q.\n", res.Token, id)

	objDiff := oldInputs.Diff(newInputs)
	if providerDiff.Changes == plugin.DiffNone || objDiff == nil {
		b.WriteString("No changes.\n")
		return b.String()
	}

	var diffBuf bytes.Buffer
	display.PrintObjectDiff(&diffBuf, *objDiff, nil, /*include*/
		true /*planning*/, 1 /*indent*/, false /*summary*/, false, /*truncateOutput*/
		false /*debug*/, showSecrets, nil /*hidden*/)
	b.WriteString(color.Colorize(diffBuf.String()))

	if len(providerDiff.ReplaceKeys) > 0 {
		b.WriteString("This change replaces the resource (")
		for i, k := range providerDiff.ReplaceKeys {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(string(k))
		}
		b.WriteString(").\n")
	}
	return b.String()
}
