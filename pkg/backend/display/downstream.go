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
	"iter"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

// maxDownstreamStackLines caps how many lines DownstreamStackLines renders before truncating.
const maxDownstreamStackLines = 5

// DownstreamStackLines renders the stacks that reference another stack as the body of a summary
// section, sorted by organization, project, and stack name and indented to match other sections.
// In interactive, colorized sessions stacks are grouped per project on one line, with each project
// and stack name a terminal hyperlink into the console. Otherwise each stack's console URL is
// listed on its own line so the link survives in logs. Output is truncated to
// maxDownstreamStackLines followed by an ellipsis line. consoleURL builds a console URL from path
// segments. The lines carry color directives and must be colorized before printing.
func DownstreamStackLines(
	refs []apitype.StackReference, consoleURL func(...string) string, color colors.Colorization, interactive bool,
) iter.Seq[string] {
	slices.SortFunc(refs, func(a, b apitype.StackReference) int {
		return cmp.Or(
			cmp.Compare(a.Organization, b.Organization),
			cmp.Compare(a.RoutingProject, b.RoutingProject),
			cmp.Compare(a.Name, b.Name),
		)
	})

	var lines []string
	if interactive && color != colors.Never {
		for i := 0; i < len(refs); {
			org, project := refs[i].Organization, refs[i].RoutingProject
			var stacks []string
			for ; i < len(refs) && refs[i].Organization == org && refs[i].RoutingProject == project; i++ {
				stacks = append(stacks, color.Hyperlink(consoleURL(org, project, refs[i].Name), refs[i].Name))
			}
			lines = append(lines,
				"    "+color.Hyperlink(consoleURL(org, project), org+"/"+project)+": "+strings.Join(stacks, ", "))
		}
	} else {
		for _, ref := range refs {
			url := consoleURL(ref.Organization, ref.RoutingProject, ref.Name)
			lines = append(lines, "    "+colors.Underline+colors.BrightBlue+url+colors.Reset)
		}
	}

	if len(lines) > maxDownstreamStackLines {
		lines = append(lines[:maxDownstreamStackLines], "    ...")
	}
	return slices.Values(lines)
}
