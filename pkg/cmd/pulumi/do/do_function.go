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
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func functionSchemaHelp(fn *schema.Function) string {
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
				fmt.Fprintf(&b, " - %s", strings.ReplaceAll(property.Comment, "\n", " "))
			}
			b.WriteString("\n")
		}
	}

	if fn.Inputs != nil {
		writeProperties("Inputs", fn.Inputs.Properties, true)
	}
	if fn.Outputs != nil {
		writeProperties("Outputs", fn.Outputs.Properties, true)
	} else if fn.ReturnType != nil {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "Outputs:\n  result (%s)\n", unwrapType(fn.ReturnType))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (pc *packageCommand) newFunctionCommand(fn *schema.Function) *cobra.Command {
	_, _, name, diags := pcl.DecomposeToken(fn.Token, hcl.Range{})
	contract.Assertf(!diags.HasErrors(), "token should decompose")

	shorthelp := fmt.Sprintf("Invoke the %s function", name)
	longhelp := shorthelp + "."
	if fn.Comment != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, fn.Comment)
	}
	if schemaHelp := functionSchemaHelp(fn); schemaHelp != "" {
		longhelp = fmt.Sprintf("%s\n\n%s", longhelp, schemaHelp)
	}

	var inputFile string

	cmd := &cobra.Command{
		Use:   name,
		Short: shorthelp,
		Long:  longhelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := pc.configureProvider(cmd, ctx); err != nil {
				return err
			}

			var inputProperties []*schema.Property
			if fn.Inputs != nil {
				inputProperties = fn.Inputs.Properties
			}
			inputs, err := evaluateFunctionFile(
				ctx, inputFile, "input", pc.format, fn, pc.evalContext(),
				pc.converter, pc.loaderTarget, pc.packageDescriptor,
				collectInputFlags(cmd, "input", inputProperties))
			if err != nil {
				return fmt.Errorf("parse input file: %w", err)
			}

			response, err := pc.provider.Invoke(ctx, plugin.InvokeRequest{
				Tok:     tokens.ModuleMember(fn.Token),
				Args:    inputs,
				Preview: pc.dryrun,
			})
			if err != nil {
				return err
			}

			var result resource.PropertyValue
			if fn.Outputs != nil {
				result = resource.NewProperty(filterOutputs(response.Properties, fn.Outputs.Properties))
			} else if fn.ReturnType != nil {
				if len(response.Properties) != 1 {
					return fmt.Errorf("expected exactly one return value from function but got %d", len(response.Properties))
				}

				for _, value := range response.Properties {
					result = filterOutput(value, fn.ReturnType)
					break
				}
			}

			// Print the response as JSON to stdout.
			outputs, err := jsonifyProperty(result, pc.showSecrets)
			if err != nil {
				return fmt.Errorf("failed to convert outputs to JSON: %w", err)
			}

			fmt.Fprint(cmd.OutOrStdout(), outputs)
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "input-file", "", "Path to a file containing function inputs")
	cmd.Flags().StringVar(&pc.providerFile, "provider-file", "",
		"Path to a file containing provider configuration")
	cmd.Flags().StringVar(&pc.format, "input", "pcl",
		"Format of the configuration files")
	cmd.Flags().StringVar(&pc.providerURN, "provider", "",
		"The URN of a provider resource in the current stack whose inputs to use as the "+
			"base of the provider configuration (requires a stack context)")

	addInputFlags(cmd, pc.spec.Name(), pc.providerDef.InputProperties)
	if fn.Inputs != nil {
		addInputFlags(cmd, "input", fn.Inputs.Properties)
	}

	return cmd
}
