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
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	hclsyntax "github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
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
			fmt.Fprintf(&b, "  %s (%s", property.Name, schemaTypeString(property.Type))
			if includeRequired {
				if property.IsRequired() {
					b.WriteString(", required")
				} else {
					b.WriteString(", optional")
				}
			}
			b.WriteString(")")
			if property.Comment != "" {
				fmt.Fprintf(&b, " - %s", strings.ReplaceAll(property.Comment, "\n", " "))
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
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, res.Comment)
	}
	if schemaHelp := resourceSchemaHelp(res); schemaHelp != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, schemaHelp)
	}

	cmd := &cobra.Command{
		Use:     name,
		GroupID: "Resources",
		Short:   shorthelp,
		Long:    longhelp,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
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
	var inputFormat string
	var yes bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := pc.configureProvider(ctx); err != nil {
				return err
			}
			urn := resourceURN(res)
			inputs, err := evaluateResourceFile(
				ctx, inputFile, "input", inputFormat, res, pc.evalContext,
				pc.converter, pc.loaderTarget, pc.packageDescriptor)
			if err != nil {
				return fmt.Errorf("parse input file: %w", err)
			}
			checked, err := pc.checkResourceInputs(ctx, urn, res, nil, inputs)
			if err != nil {
				return err
			}
			summary, err := formatCreateSummary(res, checked, pc.showSecrets)
			if err != nil {
				return err
			}
			if err := pc.confirm(cmd, summary, yes); err != nil {
				return err
			}
			response, err := pc.provider.Create(ctx, plugin.CreateRequest{
				URN:        urn,
				Name:       urn.Name(),
				Type:       urn.Type(),
				Properties: checked,
				Preview:    pc.dryrun,
			})
			if err != nil {
				return err
			}
			if response.ID == "" {
				response.ID = resource.ID("[unknown]")
			}
			return pc.printResourceResult(cmd, response.ID, response.Properties, res)
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "pcl", "Input file format")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	return cmd
}

func (pc *packageCommand) newResourceReadCommand(res *schema.Resource) *cobra.Command {
	return &cobra.Command{
		Use:   "read <id>",
		Short: "Read a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := pc.configureProvider(ctx); err != nil {
				return err
			}
			urn := resourceURN(res)
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
			return pc.printResourceResult(cmd, id, response.Outputs, res)
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
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := pc.configureProvider(ctx); err != nil {
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
				ctx, inputFile, "input", inputFormat, res, pc.evalContext,
				pc.converter, pc.loaderTarget, pc.packageDescriptor, pcl.AllowMissingProperties)
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
			if err := pc.confirm(cmd, formatPatchSummary(res, id, diff), yes); err != nil {
				return err
			}

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
				return err
			}
			return pc.printResourceResult(cmd, id, response.Properties, res)
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "pcl", "Input file format")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource inputs")
	cmd.Flags().BoolVar(&yes, "yes", false,
		"Automatically approve and perform the operation without a confirmation prompt")
	return cmd
}

func (pc *packageCommand) newResourceDeleteCommand(res *schema.Resource) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a resource",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := pc.requireYesIfNonInteractive(yes); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := pc.configureProvider(ctx); err != nil {
				return err
			}
			urn := resourceURN(res)
			id := resource.ID(args[0])
			if err := pc.confirm(cmd, formatDeleteSummary(res, id), yes); err != nil {
				return err
			}
			_, err := pc.provider.Delete(ctx, plugin.DeleteRequest{
				URN:     urn,
				Name:    urn.Name(),
				Type:    urn.Type(),
				ID:      id,
				Inputs:  resource.PropertyMap{},
				Outputs: resource.PropertyMap{},
			})
			return err
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
			if err := pc.configureProvider(ctx); err != nil {
				return err
			}

			query, err := evaluateResourceListFile(
				ctx, inputFile, "input", inputFormat, res, pc.evalContext,
				pc.converter, pc.loaderTarget, pc.packageDescriptor)
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

			return pc.printListResults(cmd, results)
		},
	}
	cmd.Flags().StringVar(&inputFormat, "input", "pcl", "Input file format")
	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing resource list inputs")
	cmd.Flags().BoolVar(&all, "all", false, "Enumerate all matching resources")
	cmd.Flags().Int64Var(&count, "count", 0, "Enumerate up to count matching resources")
	return cmd
}

func evaluateResourceListFile(
	ctx context.Context, path, fileType, inputFormat string, res *schema.Resource, evalContext functionEvalContext,
	loadConverter func(string) (plugin.Converter, error), loaderTarget string,
	packageDescriptor *codegenrpc.GetSchemaRequest,
) (resource.PropertyMap, error) {
	contract.Assertf(res.ListInputs != nil, "should not call evaluateResourceListFile for resources without list inputs")

	bind := func(file *hclsyntax.File) ([]*model.Attribute, model.Type, []*schema.Property, hcl.Diagnostics) {
		attrs, inputType, diags := pcl.BindResourceList(file, res)
		return attrs, inputType, res.ListInputs.Properties, diags
	}
	return evaluateFile(
		ctx, path, fileType, inputFormat, res.Token, bind, loadConverter, loaderTarget, packageDescriptor, evalContext,
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

func (pc *packageCommand) printResourceResult(
	cmd *cobra.Command, id resource.ID, outputs resource.PropertyMap, res *schema.Resource,
) error {
	contract.Requiref(id != "", "id", "id should not be blank")

	if res.Properties != nil {
		outputs = filterOutputs(outputs, res.Properties)
	}
	result := resource.PropertyMap{
		"id":         resource.NewProperty(string(id)),
		"properties": resource.NewProperty(outputs),
	}
	output, err := jsonifyProperty(resource.NewProperty(result), pc.showSecrets)
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

func formatCreateSummary(res *schema.Resource, inputs resource.PropertyMap, showSecrets bool) (string, error) {
	body, err := jsonifyProperty(resource.NewProperty(inputs), showSecrets)
	if err != nil {
		return "", fmt.Errorf("format inputs: %w", err)
	}
	return fmt.Sprintf("This will create %s with the following inputs:\n%s", res.Token, body), nil
}

func formatDeleteSummary(res *schema.Resource, id resource.ID) string {
	return fmt.Sprintf("This will delete %s %q.", res.Token, id)
}

// formatPatchSummary renders a human-readable summary of the changes a patch will apply, using the provider's
// Diff response. If the provider returns no DetailedDiff we fall back to a short "no detailed diff available"
// note — the operation can still proceed since Update is what actually applies the change.
func formatPatchSummary(res *schema.Resource, id resource.ID, diff plugin.DiffResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This will update %s %q.", res.Token, id)
	if diff.Changes == plugin.DiffNone {
		b.WriteString("\nNo changes.")
		return b.String()
	}
	if len(diff.DetailedDiff) == 0 {
		if len(diff.ChangedKeys) > 0 {
			b.WriteString("\nChanged properties:")
			for _, k := range diff.ChangedKeys {
				fmt.Fprintf(&b, "\n  %s", k)
			}
		} else {
			b.WriteString("\nProvider reported changes but no detailed diff is available.")
		}
		return b.String()
	}
	b.WriteString("\nChanges:")
	// Stable order so the prompt is deterministic.
	paths := make([]string, 0, len(diff.DetailedDiff))
	for k := range diff.DetailedDiff {
		paths = append(paths, k)
	}
	sort.Strings(paths)
	for _, p := range paths {
		d := diff.DetailedDiff[p]
		marker := diffKindMarker(d.Kind)
		fmt.Fprintf(&b, "\n  %s %s", marker, p)
	}
	return b.String()
}

func diffKindMarker(k plugin.DiffKind) string {
	switch k {
	case plugin.DiffAdd, plugin.DiffAddReplace:
		return "+"
	case plugin.DiffDelete, plugin.DiffDeleteReplace:
		return "-"
	case plugin.DiffUpdate, plugin.DiffUpdateReplace:
		return "~"
	default:
		return "?"
	}
}
