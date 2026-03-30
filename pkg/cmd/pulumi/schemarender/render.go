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

package schemarender

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// CleanDescription strips HTML tags (especially pulumi-lang spans) from schema descriptions
// and cleans up the result for terminal or markdown display.
func CleanDescription(desc string) string {
	return htmlTagRe.ReplaceAllString(desc, "")
}

// GetType resolves a TypeSpec to a human-readable type string.
func GetType(spec *schema.PackageSpec, prop schema.TypeSpec) (string, error) {
	typ := prop.Type
	if typ != "" && typ != "object" && typ != "array" && prop.Ref == "" {
		return typ, nil
	}
	if prop.Type == "array" {
		if prop.Items == nil {
			return "[]unknown", nil
		}
		typ, err := GetType(spec, *prop.Items)
		if err != nil {
			return "", err
		}
		return "[]" + typ, nil
	}
	if prop.Type == "object" {
		if prop.AdditionalProperties == nil {
			return "object", nil
		}
		typ, err := GetType(spec, *prop.AdditionalProperties)
		if err != nil {
			return "", err
		}
		return "map[string]" + typ, nil
	}
	if prop.Ref != "" {
		if strings.HasPrefix(prop.Ref, "#/types/") {
			ref := strings.TrimPrefix(prop.Ref, "#/types/")
			ref = strings.ReplaceAll(ref, "%2F", "/")
			if typeSpec, ok := spec.Types[ref]; ok {
				if len(typeSpec.Enum) > 0 {
					return fmt.Sprintf("enum(%s){%s}",
						typeSpec.Type, FormatEnumValues(typeSpec.Enum)), nil
				}
				simplifiedName, err := SimplifyModuleName("type", ref)
				if err != nil {
					return "", err
				}
				split := strings.Split(simplifiedName, ":")
				return split[2], nil
			}
		}
		return prop.Ref, nil
	}
	return "unknown", nil
}

// SummaryFromDescription extracts the first paragraph from a markdown description,
// stripping HTML tags for clean display.
func SummaryFromDescription(description string) string {
	description = CleanDescription(description)
	var summary strings.Builder
	for _, line := range strings.Split(description, "\n") {
		if strings.TrimSpace(line) == "" {
			break
		}
		summary.WriteString(line + " ")
	}
	return strings.TrimSpace(summary.String())
}

// SimplifyModuleName simplifies "aws:ec2/instance:Instance" to "aws:ec2:Instance".
func SimplifyModuleName(typ string, name string) (string, error) {
	split := strings.Split(name, ":")
	if len(split) < 3 {
		return "", fmt.Errorf("invalid %s name %q", typ, name)
	}
	moduleSplit := strings.Split(split[1], "/")
	return split[0] + ":" + moduleSplit[0] + ":" + split[2], nil
}

// Bold wraps text in terminal bold escape codes.
func Bold(s string) string {
	return colors.Always.Colorize(colors.Bold + s + colors.Reset)
}

// Underline wraps text in terminal underline escape codes.
func Underline(s string) string {
	return colors.Always.Colorize(colors.Underline + s + colors.Reset)
}

// FormatEnumValues formats enum values as a comma-separated string.
func FormatEnumValues(enum []schema.EnumValueSpec) string {
	var values []string
	for _, v := range enum {
		if v.Name != "" {
			values = append(values, v.Name)
		} else if v.Value != nil {
			values = append(values, fmt.Sprintf("%v", v.Value))
		}
	}
	return strings.Join(values, ", ")
}
