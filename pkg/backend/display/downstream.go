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

package display

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

const maxDownstreamStackLines = 5

// FormatDownstreamStacks writes the "Downstream Stacks" summary section to w, colorized per
// opts.Color. Nothing is written when refs is empty. consoleURL builds a console URL from path
// segments.
func FormatDownstreamStacks(
	w io.Writer, refs []apitype.StackReference, consoleURL func(...string) string, opts Options,
) {
	if len(refs) == 0 {
		return
	}
	noun := "Downstream Stacks"
	if len(refs) == 1 {
		noun = "Downstream Stack"
	}
	fmt.Fprint(w, opts.Color.Colorize(fmt.Sprintf("%s%s: (%d)%s\n", colors.SpecHeadline, noun, len(refs), colors.Reset)))
	downstreamStackLines(w, refs, consoleURL, opts.Color, opts.IsInteractive)
}

// downstreamStackLines writes the section body. Interactive, colorized sessions get stacks
// grouped per project, hyperlinked into the console; other sessions get one console URL per line
// so the link survives in logs.
func downstreamStackLines(
	w io.Writer, refs []apitype.StackReference, consoleURL func(...string) string,
	color colors.Colorization, interactive bool,
) {
	slices.SortFunc(refs, func(a, b apitype.StackReference) int {
		return cmp.Or(
			cmp.Compare(a.Organization, b.Organization),
			cmp.Compare(a.RoutingProject, b.RoutingProject),
			cmp.Compare(a.Name, b.Name),
		)
	})

	written := 0
	line := func(s string) {
		fmt.Fprint(w, color.Colorize(s+"\n"))
		written++
	}
	if interactive && color != colors.Never {
		for i := 0; i < len(refs); {
			if written == maxDownstreamStackLines {
				line("    ...")
				return
			}
			org, project := refs[i].Organization, refs[i].RoutingProject
			var stacks []string
			for ; i < len(refs) && refs[i].Organization == org && refs[i].RoutingProject == project; i++ {
				stacks = append(stacks, color.Hyperlink(consoleURL(org, project, refs[i].Name), refs[i].Name))
			}
			line("    " + color.Hyperlink(consoleURL(org, project), org+"/"+project) + ": " + strings.Join(stacks, ", "))
		}
	} else {
		for _, ref := range refs {
			if written == maxDownstreamStackLines {
				line("    ...")
				return
			}
			url := consoleURL(ref.Organization, ref.RoutingProject, ref.Name)
			line("    " + colors.Underline + colors.BrightBlue + url + colors.Reset)
		}
	}
}
