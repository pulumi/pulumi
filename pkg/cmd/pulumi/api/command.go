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
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/spf13/cobra"
)

// buildOperationCmd creates a cobra.Command from an OperationSpec.
func buildOperationCmd(spec OperationSpec) *cobra.Command {
	// Categorize path params:
	// - orgName: resolved via --org flag
	// - Params with Values (synthetic enums) or !Required (variant-specific): flags
	// - All other required path params: positional args
	var positionalArgs []ParamSpec
	var pathParamFlags []ParamSpec
	needsOrg := false
	for _, p := range spec.Params {
		if p.In == "path" {
			if p.Name == "orgName" {
				needsOrg = true
			} else if len(p.Values) > 0 || !p.Required {
				pathParamFlags = append(pathParamFlags, p)
			} else {
				positionalArgs = append(positionalArgs, p)
			}
		}
	}

	// Determine query params (for flags)
	var queryParams []ParamSpec
	for _, p := range spec.Params {
		if p.In == "query" {
			queryParams = append(queryParams, p)
		}
	}

	// Build short description from the first sentence of Description,
	// falling back to Summary.
	short := spec.Description
	if short == "" {
		short = spec.Summary
	}
	if idx := strings.Index(short, ". "); idx > 0 {
		short = short[:idx+1]
	} else if idx := strings.Index(short, ".\n"); idx > 0 {
		short = short[:idx+1]
	}

	// Build long description
	long := spec.Description
	if long == "" {
		long = spec.Summary
	}
	if spec.BodySchemaText != "" {
		long += "\n\n" + spec.BodySchemaText
	}
	if spec.ResponseSchemaText != "" {
		long += "\n\n" + spec.ResponseSchemaText
	}

	var orgFlag string
	var jsonOutput bool
	var bodyFlag string
	var bodyStdin bool

	// Storage for flag values
	pathFlagVals := make(map[string]*string)
	queryStringVals := make(map[string]*string)
	queryIntVals := make(map[string]*int)
	queryBoolVals := make(map[string]*bool)

	cmd := &cobra.Command{
		Use:   spec.CommandName,
		Short: short,
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Resolve context
			resolved, err := ResolveContext(orgFlag, needsOrg)
			if err != nil {
				return err
			}

			// Build map of all path param values.
			paramVals := make(map[string]string)
			if needsOrg {
				paramVals["orgName"] = resolved.OrgName
			}
			argIdx := 0
			for _, p := range positionalArgs {
				if argIdx < len(args) {
					paramVals[p.Name] = args[argIdx]
					argIdx++
				}
			}
			for name, val := range pathFlagVals {
				if val != nil && *val != "" {
					paramVals[name] = *val
				}
			}

			// Select path and content types.
			var path, bodyCT, respCT string
			if len(spec.PathVariants) > 0 {
				variant, varErr := selectVariant(spec.PathVariants, paramVals)
				if varErr != nil {
					return varErr
				}
				path = variant.Path
				bodyCT = variant.BodyContentType
				respCT = variant.ResponseContentType
			} else {
				path = spec.Path
				bodyCT = spec.BodyContentType
				respCT = spec.ResponseContentType
			}

			// Substitute all param values into path.
			for name, val := range paramVals {
				path = strings.Replace(path, "{"+name+"}", url.PathEscape(val), 1)
			}

			// Build query string
			query := url.Values{}
			for name, val := range queryStringVals {
				if val != nil && *val != "" {
					query.Set(name, *val)
				}
			}
			for name, val := range queryIntVals {
				if val != nil && *val != 0 {
					query.Set(name, strconv.Itoa(*val))
				}
			}
			for name, val := range queryBoolVals {
				if val != nil && *val {
					query.Set(name, "true")
				}
			}

			// Handle body
			var body io.Reader
			if spec.HasBody {
				if bodyStdin {
					body = os.Stdin
				} else if bodyFlag != "" {
					if strings.HasPrefix(bodyFlag, "@") {
						f, err := os.Open(bodyFlag[1:])
						if err != nil {
							return fmt.Errorf("opening body file: %w", err)
						}
						defer f.Close()
						body = f
					} else {
						body = strings.NewReader(bodyFlag)
					}
				}
			}

			// Make the request
			apiClient := NewAPIClient(resolved.CloudURL, resolved.Token)
			resp, err := apiClient.Do(ctx, spec.Method, path, query, body, bodyCT, respCT)
			if err != nil {
				return fmt.Errorf("API request failed: %w", err)
			}

			return FormatResponse(resp, jsonOutput)
		},
	}

	// Set up constrictor arguments for positional args
	if len(positionalArgs) > 0 {
		constrictorArgs := make([]constrictor.Argument, len(positionalArgs))
		for i, p := range positionalArgs {
			constrictorArgs[i] = constrictor.Argument{
				Name: p.Name,
			}
		}
		constrictor.AttachArguments(cmd, &constrictor.Arguments{
			Arguments: constrictorArgs,
			Required:  len(positionalArgs),
		})
	} else {
		constrictor.AttachArguments(cmd, constrictor.NoArgs)
	}

	// Add standard flags
	cmd.Flags().StringVar(&orgFlag, "org", "", "Override the default organization")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output raw JSON response")

	if spec.HasBody {
		bodyDesc := "Request body JSON (or @filename to read from file)"
		if spec.BodyContentType == "application/x-yaml" {
			bodyDesc = "Request body YAML (or @filename to read from file)"
		} else if spec.BodyContentType != "" && spec.BodyContentType != "application/json" {
			bodyDesc = fmt.Sprintf("Request body %s (or @filename to read from file)", spec.BodyContentType)
		}
		cmd.Flags().StringVar(&bodyFlag, "body", "", bodyDesc)
		cmd.Flags().BoolVar(&bodyStdin, "body-stdin", false, "Read request body from stdin")
	}

	// Add path param flags (enum params and variant-specific optional params)
	for _, p := range pathParamFlags {
		val := new(string)
		pathFlagVals[p.Name] = val
		flagName := toKebab(p.Name)
		desc := p.Description
		if len(p.Values) > 0 {
			desc = fmt.Sprintf("One of: %s", strings.Join(p.Values, ", "))
		}
		cmd.Flags().StringVar(val, flagName, "", desc)
		if len(p.Values) > 0 {
			cmd.MarkFlagRequired(flagName) //nolint:errcheck
		}
	}

	// Add query param flags
	for _, p := range queryParams {
		switch p.Type {
		case "integer", "number":
			val := new(int)
			queryIntVals[p.Name] = val
			cmd.Flags().IntVar(val, p.Name, 0, p.Description)
		case "boolean":
			val := new(bool)
			queryBoolVals[p.Name] = val
			cmd.Flags().BoolVar(val, p.Name, false, p.Description)
		default: // "string" and anything else
			val := new(string)
			queryStringVals[p.Name] = val
			cmd.Flags().StringVar(val, p.Name, "", p.Description)
		}
	}

	return cmd
}

// selectVariant picks the PathVariant whose path template can be fully resolved
// with the provided param values. Variants are tried in order (longest first).
func selectVariant(variants []PathVariant, paramVals map[string]string) (*PathVariant, error) {
	for i := range variants {
		path := variants[i].Path
		resolved := true
		for _, seg := range strings.Split(path, "/") {
			if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
				paramName := seg[1 : len(seg)-1]
				if _, ok := paramVals[paramName]; !ok {
					resolved = false
					break
				}
			}
		}
		if resolved {
			return &variants[i], nil
		}
	}

	// Build a helpful error listing the optional flags.
	var optionalParams []string
	for _, v := range variants {
		for _, seg := range strings.Split(v.Path, "/") {
			if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
				name := seg[1 : len(seg)-1]
				if _, ok := paramVals[name]; !ok {
					optionalParams = append(optionalParams, "--"+toKebab(name))
				}
			}
		}
	}
	return nil, fmt.Errorf("provide one of: %s", strings.Join(optionalParams, ", "))
}
