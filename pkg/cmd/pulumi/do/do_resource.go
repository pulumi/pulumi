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
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

func resourceSchemaHelp(res *schema.Resource) string {
	var b strings.Builder
	writeProperties := func(title string, properties []*schema.Property, includeRequired bool) {
		if len(properties) == 0 {
			return
		}
		if b.Len() > 0 {
			trimmed := strings.TrimSuffix(b.String(), "\n")
			b.Reset()
			b.WriteString(trimmed)
			b.WriteString("\n\n")
		}
		b.WriteString(title)
		b.WriteString(":\n")
		for _, property := range properties {
			fmt.Fprintf(&b, "  %s (%s", property.Name, unwrapType(property.Type))
			if includeRequired {
				if property.IsRequired() {
					b.WriteString(", required")
				} else {
					b.WriteString(", optional")
				}
			}
			b.WriteString(")")
			if property.Comment != "" {
				fmt.Fprintf(&b, " - %s", strings.ReplaceAll(cleanComment(property.Comment), "\n", " "))
			}
			b.WriteString("\n")
		}
	}

	writeProperties("Inputs", res.InputProperties, true)
	writeProperties("Outputs", res.Properties, false)
	if res.ListInputs != nil {
		writeProperties("List Inputs", res.ListInputs.Properties, true)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (pc *packageCommand) newResourceCommand(res *schema.Resource) *cobra.Command {
	_, _, name, diags := pcl.DecomposeToken(res.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")

	shorthelp := fmt.Sprintf("Operate on the %s resource", name)
	longhelp := shorthelp + "."
	if res.Comment != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, cleanComment(res.Comment))
	}
	if schemaHelp := resourceSchemaHelp(res); schemaHelp != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, schemaHelp)
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
	cmd.AddCommand(pc.newResourceCreateCommand(res))
	cmd.AddCommand(pc.newResourceReadCommand(res))
	cmd.AddCommand(pc.newResourcePatchCommand(res))
	cmd.AddCommand(pc.newResourceDeleteCommand(res))
	if res.ListInputs != nil {
		cmd.AddCommand(pc.newResourceListCommand(res))
	}
	return cmd
}

func (pc *packageCommand) newResourceCreateCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var yes bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !pc.stateless {
				return errStatefulNotImplemented("create")
			}
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			urn := resourceURN(res)
			var checked resource.PropertyMap
			prepare := func() (*resource.State, error) {
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
				checked, err = pc.checkResourceInputs(ctx, urn, res, nil, inputs)
				if err != nil {
					return nil, err
				}
				return operationState(urn, "", checked, nil), nil
			}
			create := func() (*resource.State, error) {
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
				}, func() (*resource.State, error) {
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
		},
	}
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", res.InputProperties)
	return cmd
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
			}, func() (*resource.State, error) {
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
				if response.Outputs == nil {
					return nil, fmt.Errorf("resource %q was not found", args[0])
				}
				if response.ID != "" {
					id = response.ID
				}
				return resultState(urn, id, nil, response.Outputs, res), nil
			})
		},
	}
}

func (pc *packageCommand) newResourcePatchCommand(res *schema.Resource) *cobra.Command {
	var inputFile string
	var inputFormat string
	var yes bool
	cmd := &cobra.Command{
		Use:   "patch <id>",
		Short: "Patch a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !pc.stateless {
				return errStatefulNotImplemented("patch")
			}
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
			if read.Outputs == nil {
				return fmt.Errorf("resource %q was not found", args[0])
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

			oldInputs := read.Inputs
			newInputs := oldInputs.Copy()
			for key, value := range patch {
				newInputs[key] = value
			}
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
			if err := pc.confirm(cmd, summary, "patch", yes); err != nil {
				return err
			}

			return pc.runDisplayedStep(cmd, displayedStep{
				Op:           deploy.OpUpdate,
				Old:          operationState(urn, id, oldInputs, read.Outputs),
				New:          operationState(urn, id, checked, nil),
				Diffs:        diff.ChangedKeys,
				DetailedDiff: diff.DetailedDiff,
			}, func() (*resource.State, error) {
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
					return nil, err
				}
				return resultState(urn, id, checked, response.Properties, res), nil
			})
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "yaml", "Format of the configuration files")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	addInputFlags(cmd, "input", res.InputProperties)
	return cmd
}

func (pc *packageCommand) newResourceDeleteCommand(res *schema.Resource) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !pc.stateless {
				return errStatefulNotImplemented("delete")
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
			if response.Outputs == nil {
				return fmt.Errorf("resource %q was not found", args[0])
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
			}, func() (*resource.State, error) {
				_, err := pc.provider.Delete(ctx, plugin.DeleteRequest{
					URN:     urn,
					Name:    urn.Name(),
					Type:    urn.Type(),
					ID:      id,
					Inputs:  response.Inputs,
					Outputs: response.Outputs,
				})
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
					Query:             query,
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
		attrs, inputType, diags := pcl.BindResourceList(file, res)
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
) *resource.State {
	return operationState(urn, id, inputs, resultOutputs(id, outputs, res))
}

func (pc *packageCommand) printResourceResult(cmd *cobra.Command, state *resource.State) error {
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

// errStatefulNotImplemented is returned from create/patch/delete when the user did not pass
// --stateless. The stateful (engine-driven) implementation of these operations is the planned
// default but isn't built yet, so for now the only working path is opting in to the stateless one.
func errStatefulNotImplemented(op string) error {
	return fmt.Errorf("`%s` is not yet implemented in stateful mode; pass --stateless to use the "+
		"direct-provider implementation", op)
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
