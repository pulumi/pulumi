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

// Package schemainfo renders schema resource and function inputs/outputs for CLI help. It is shared
// by `pulumi package info` and `pulumi do` so both display property information the same way.
package schemainfo

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

// Kind distinguishes an inputs section from an outputs section, which changes the footnote wording.
type Kind int

const (
	// Inputs is a section of input properties; required members are annotated with '*'.
	Inputs Kind = iota
	// Outputs is a section of output properties; always-present members are annotated with '*'.
	Outputs
	// ListInputs is a section of list-operation input properties; required members are annotated
	// with '*'.
	ListInputs
)

// Property is a single input or output row to render. Type is the already-rendered type string;
// Description is the raw schema description, which is summarized on render.
type Property struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// BoundProperties adapts bound schema properties to the rendering model, rendering each type with
// TypeString.
func BoundProperties(properties []*schema.Property) []Property {
	result := make([]Property, 0, len(properties))
	for _, prop := range properties {
		result = append(result, Property{
			Name:        prop.Name,
			Type:        TypeString(prop.Type),
			Required:    prop.IsRequired(),
			Description: prop.Comment,
		})
	}
	return result
}

// TypeString renders a bound schema.Type for display, eliding the outer Optional and Input wrappers
// (the property's own optionality is conveyed by the '*' marker) and otherwise using the type's
// canonical String form, e.g. "Array<Input<string>>" or "pkg:mod:Foo•Input".
func TypeString(typ schema.Type) string {
	for {
		switch t := typ.(type) {
		case *schema.OptionalType:
			typ = t.ElementType
		case *schema.InputType:
			typ = t.ElementType
		default:
			return typ.String()
		}
	}
}

// Bold renders s in bold. Colorization is always enabled so the output is consistent regardless of
// whether it is written to a terminal.
func Bold(s string) string {
	return colors.Always.Colorize(colors.Bold + s + colors.Reset)
}

// Underline renders s underlined.
func Underline(s string) string {
	return colors.Always.Colorize(colors.Underline + s + colors.Reset)
}

var langChoiceSpanRegexp = regexp.MustCompile(`(?s)<span\b[^>]*>(.*?)</span>`)

// Summarize returns the first paragraph of a schema description for use as a one-line summary. It
// resolves the language-choice `<span>` markup that bridged-provider descriptions embed to its
// canonical text and collapses the paragraph's internal newlines into spaces.
func Summarize(description string) string {
	description = langChoiceSpanRegexp.ReplaceAllString(description, "$1")

	var summary strings.Builder
	started := false
	for _, line := range strings.Split(description, "\n") {
		if strings.TrimSpace(line) == "" {
			if started {
				// A blank line ends the first paragraph.
				break
			}
			// Skip leading blank lines.
			continue
		}
		started = true
		summary.WriteString(line + " ")
	}
	return strings.TrimSpace(summary.String())
}

// WriteProperties writes a titled, alphabetically-sorted property section. Each property renders as
//
//   - <bold name> (<underline type>[<underline *>]): <summary>
//
// and, when any property is marked, a footnote explaining the '*' is appended. The section title is
// always written, even when there are no properties.
func WriteProperties(w io.Writer, title string, props []Property, kind Kind) {
	fmt.Fprintln(w, Bold(title)+":")

	sorted := make([]Property, len(props))
	copy(sorted, props)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	marked := false
	for _, prop := range sorted {
		marker := ""
		if prop.Required {
			marker = "*"
			marked = true
		}
		fmt.Fprintf(w, " - %s (%s%s)", Bold(prop.Name), Underline(prop.Type), Underline(marker))
		if summary := Summarize(prop.Description); summary != "" {
			fmt.Fprintf(w, ": %s", summary)
		}
		fmt.Fprintln(w)
	}

	if marked {
		switch kind {
		case Inputs:
			fmt.Fprintln(w, "Inputs marked with '*' are required")
		case ListInputs:
			fmt.Fprintln(w, "List inputs marked with '*' are required")
		case Outputs:
			fmt.Fprintln(w, "Outputs marked with '*' are always present")
		}
	}
}
