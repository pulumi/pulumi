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
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func TestDownstreamStackLines(t *testing.T) {
	t.Parallel()

	consoleURL := func(paths ...string) string { return "https://console/" + strings.Join(paths, "/") }
	ref := func(org, project, name string) apitype.StackReference {
		return apitype.StackReference{Organization: org, RoutingProject: project, Name: name}
	}
	lines := func(refs []apitype.StackReference, color colors.Colorization, interactive bool) []string {
		return slices.Collect(DownstreamStackLines(refs, consoleURL, color, interactive))
	}
	urlLine := func(s string) string {
		return "    " + colors.Underline + colors.BrightBlue + "https://console/" + s + colors.Reset
	}
	link := func(path, text string) string { return colors.Always.Hyperlink("https://console/"+path, text) }

	assert.Empty(t, lines(nil, colors.Always, true))

	// Non-interactive output lists one console URL per stack.
	assert.Equal(t, []string{urlLine("org/net/dev")},
		lines([]apitype.StackReference{ref("org", "net", "dev")}, colors.Always, false))
	assert.Equal(t,
		[]string{
			urlLine("org/net/a"), urlLine("org/net/b"), urlLine("org/net/c"), urlLine("org/net/d"), urlLine("org/net/e"),
			"    ...",
		},
		lines([]apitype.StackReference{
			ref("org", "net", "a"), ref("org", "net", "b"), ref("org", "net", "c"),
			ref("org", "net", "d"), ref("org", "net", "e"), ref("org", "net", "f"),
		}, colors.Always, false))

	// Colors off while interactive: still the URL list, since hyperlinks would be dropped.
	assert.Equal(t, []string{urlLine("org/net/dev")},
		lines([]apitype.StackReference{ref("org", "net", "dev")}, colors.Never, true))

	// Interactive output groups by org/project, sorted, with hyperlinked names.
	assert.Equal(t,
		[]string{
			"    " + link("org/app", "org/app") + ": " + link("org/app/dev", "dev"),
			"    " + link("org/net", "org/net") + ": " + link("org/net/dev", "dev") + ", " + link("org/net/prod", "prod"),
			"    " + link("other/svc", "other/svc") + ": " + link("other/svc/prod", "prod"),
		},
		lines([]apitype.StackReference{
			ref("org", "net", "prod"), ref("other", "svc", "prod"), ref("org", "net", "dev"), ref("org", "app", "dev"),
		}, colors.Always, true))

	// Interactive output truncates by project line.
	assert.Equal(t,
		[]string{
			"    " + link("org/p1", "org/p1") + ": " + link("org/p1/dev", "dev"),
			"    " + link("org/p2", "org/p2") + ": " + link("org/p2/dev", "dev"),
			"    " + link("org/p3", "org/p3") + ": " + link("org/p3/dev", "dev"),
			"    " + link("org/p4", "org/p4") + ": " + link("org/p4/dev", "dev"),
			"    " + link("org/p5", "org/p5") + ": " + link("org/p5/dev", "dev"),
			"    ...",
		},
		lines([]apitype.StackReference{
			ref("org", "p1", "dev"), ref("org", "p2", "dev"), ref("org", "p3", "dev"),
			ref("org", "p4", "dev"), ref("org", "p5", "dev"), ref("org", "p6", "dev"),
		}, colors.Always, true))
}
