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

package registry

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/blang/semver"
	cmdcmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/schemarender"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	commonregistry "github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/spf13/cobra"
)

type exampleDetailJSON struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	Language string `json:"language"`
	Code     string `json:"code"`
}

var codeChooserRe = regexp.MustCompile(`(?s)<!--Start PulumiCodeChooser -->(.*?)<!--End PulumiCodeChooser -->`)
var codeFenceRe = regexp.MustCompile("(?s)```(\\w+)\n(.*?)```")
var markdownHeadingRe = regexp.MustCompile(`^#{1,4}\s*`)
var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// commentPrefix returns the single-line comment syntax for a language.
func commentPrefix(language string) string {
	switch language {
	case "python", "yaml":
		return "#"
	default:
		return "//"
	}
}

func newRegistryExampleGetCmd() *cobra.Command {
	var jsonOut bool
	var versionStr string
	var language string

	cmd := &cobra.Command{
		Use:   "get <package-or-token> <index> --language <lang>",
		Short: "Get a code example",
		Long: `Get a code example for a package, resource, or function in a given language.

Pass a package name (e.g., aws) for package-level examples, or a type token
(e.g., aws:ec2/instance:Instance) for resource or function examples.

Use 'registry example ls' to see available examples and their indices.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if language == "" {
				return fmt.Errorf("--language is required (e.g. --language typescript)")
			}

			ctx := cmd.Context()
			reg := cmdcmd.NewDefaultRegistry(ctx, pkgWorkspace.Instance, nil, cmdutil.Diag(), env.Global())

			target := args[0]
			exampleIndex, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid example index %q: must be a number", args[1])
			}

			var version *semver.Version
			if versionStr != "" {
				v, err := semver.Parse(versionStr)
				if err != nil {
					return fmt.Errorf("invalid version %q: %w", versionStr, err)
				}
				version = &v
			}

			examples, err := resolveExamples(ctx, reg, target, version)
			if len(examples) == 0 {
				return fmt.Errorf("no code examples found for %q", target)
			}

			if exampleIndex >= len(examples) {
				return fmt.Errorf("example index %d out of range (have %d examples)", exampleIndex, len(examples))
			}

			ex := examples[exampleIndex]
			code, ok := ex.codeByLang[language]
			if !ok {
				return fmt.Errorf("language %q not found in example %d; available: %s",
					language, exampleIndex, strings.Join(ex.languages, ", "))
			}

			if jsonOut {
				return ui.PrintJSON(exampleDetailJSON{
					Index:    exampleIndex,
					Title:    ex.title,
					Language: language,
					Code:     code,
				})
			}
			fmt.Printf("%s %s\n", commentPrefix(language), ex.title)
			fmt.Print(code)
			if !strings.HasSuffix(code, "\n") {
				fmt.Println()
			}
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{
			{Name: "package-or-token"},
			{Name: "index", Usage: "<example-index>"},
		},
		Required: 2,
	})

	cmd.PersistentFlags().BoolVarP(&jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVar(&versionStr, "version", "", "Specific package version")
	cmd.PersistentFlags().StringVarP(&language, "language", "l", "",
		"Language to show (typescript, python, go, csharp, java, yaml)")

	return cmd
}

// resolveExamples resolves a package name or type token to a list of examples.
// For type tokens, it returns examples from that resource/function's description.
// For package names, it aggregates examples from all resources and functions in the schema.
func resolveExamples(
	ctx context.Context,
	reg commonregistry.Registry,
	target string,
	version *semver.Version,
) ([]parsedExample, error) {
	if strings.Contains(target, ":") {
		// It's a type token — could be a resource or function.
		packageName, err := parsePackageFromToken(target)
		if err != nil {
			return nil, err
		}
		spec, err := loadSchemaForPackage(ctx, reg, packageName, version)
		if err != nil {
			return nil, err
		}
		if _, res, err := findResource(spec, target); err == nil {
			return parseExamples(res.Description), nil
		}
		if _, fn, err := findFunction(spec, target); err == nil {
			return parseExamples(fn.Description), nil
		}
		return nil, fmt.Errorf("resource or function %q not found", target)
	}

	// It's a package name — aggregate examples from all resources and functions.
	spec, err := loadSchemaForPackage(ctx, reg, target, version)
	if err != nil {
		return nil, err
	}

	var all []parsedExample
	for token, res := range spec.Resources {
		examples := parseExamples(res.Description)
		for i := range examples {
			// Prefix the title with the resource token for context.
			simplified, sErr := schemarender.SimplifyModuleName("resource", token)
			if sErr != nil {
				simplified = token
			}
			if examples[i].title == "Example" || examples[i].title == "" {
				examples[i].title = simplified
			} else {
				examples[i].title = simplified + ": " + examples[i].title
			}
		}
		all = append(all, examples...)
	}
	for token, fn := range spec.Functions {
		examples := parseExamples(fn.Description)
		for i := range examples {
			simplified, sErr := schemarender.SimplifyModuleName("function", token)
			if sErr != nil {
				simplified = token
			}
			if examples[i].title == "Example" || examples[i].title == "" {
				examples[i].title = simplified
			} else {
				examples[i].title = simplified + ": " + examples[i].title
			}
		}
		all = append(all, examples...)
	}

	// Sort by title for consistent output.
	sort.Slice(all, func(i, j int) bool {
		return all[i].title < all[j].title
	})

	return all, nil
}

// descriptionBeforeExamples returns only the prose text before the first
// code example block, stripping HTML tags and trailing "Example Usage" headings.
func descriptionBeforeExamples(description string) string {
	idx := strings.Index(description, "<!--Start PulumiCodeChooser -->")
	if idx >= 0 {
		description = strings.TrimSpace(description[:idx])
	}
	// Strip trailing "## Example Usage" or similar headings that precede code blocks.
	lines := strings.Split(description, "\n")
	for len(lines) > 0 {
		last := strings.TrimSpace(lines[len(lines)-1])
		if last == "" || strings.HasPrefix(last, "## Example") || strings.HasPrefix(last, "### Example") {
			lines = lines[:len(lines)-1]
		} else {
			break
		}
	}
	description = strings.TrimSpace(strings.Join(lines, "\n"))
	return schemarender.CleanDescription(description)
}

// firstExampleCode extracts the first code example in the given language
// from a description string, or empty string if none found.
func firstExampleCode(description, language string) string {
	examples := parseExamples(description)
	if len(examples) == 0 {
		return ""
	}
	if code, ok := examples[0].codeByLang[language]; ok {
		return code
	}
	// Try any available language.
	for _, lang := range examples[0].languages {
		if code, ok := examples[0].codeByLang[lang]; ok {
			return code
		}
	}
	return ""
}

type parsedExample struct {
	title      string
	languages  []string
	codeByLang map[string]string
}

func parseExamples(description string) []parsedExample {
	// Split on the start marker to get the text before each code block.
	parts := strings.Split(description, "<!--Start PulumiCodeChooser -->")
	if len(parts) < 2 {
		return nil
	}

	chooserMatches := codeChooserRe.FindAllStringSubmatch(description, -1)
	if len(chooserMatches) == 0 {
		return nil
	}

	var examples []parsedExample
	for i, match := range chooserMatches {
		block := match[1]
		ex := parsedExample{
			title:      extractTitle(parts[i]),
			codeByLang: make(map[string]string),
		}

		fenceMatches := codeFenceRe.FindAllStringSubmatch(block, -1)
		for _, fm := range fenceMatches {
			lang := fm[1]
			code := strings.TrimRight(fm[2], "\n")
			if lang == "sh" || lang == "" {
				continue
			}
			ex.languages = append(ex.languages, lang)
			ex.codeByLang[lang] = code
		}

		if len(ex.languages) > 0 {
			examples = append(examples, ex)
		}
	}
	return examples
}

// extractTitle finds a heading or short description before a code block to use as a title.
func extractTitle(textBefore string) string {
	lines := strings.Split(strings.TrimSpace(textBefore), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, ">") {
			continue
		}
		line = markdownHeadingRe.ReplaceAllString(line, "")
		line = htmlTagRe.ReplaceAllString(line, "")
		line = strings.ReplaceAll(line, "`", "")
		line = strings.ReplaceAll(line, "**", "")
		line = strings.TrimSpace(line)
		if len(line) > 80 {
			line = line[:77] + "..."
		}
		if line != "" {
			return line
		}
	}
	return "Example"
}

