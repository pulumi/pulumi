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
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func TestFormatDownstreamStacks(t *testing.T) {
	t.Parallel()

	consoleURL := func(paths ...string) string { return "https://console/" + strings.Join(paths, "/") }
	format := func(refs []apitype.StackReference, opts Options) string {
		var buf bytes.Buffer
		FormatDownstreamStacks(&buf, refs, consoleURL, opts)
		return buf.String()
	}
	ref := func(org, project, name string) apitype.StackReference {
		return apitype.StackReference{Organization: org, RoutingProject: project, Name: name}
	}
	// Raw keeps color directives verbatim, so expectations stay readable.
	raw := func(interactive bool) Options { return Options{Color: colors.Raw, IsInteractive: interactive} }
	header := func(s string) string { return colors.SpecHeadline + s + colors.Reset + "\n" }
	urlLine := func(s string) string {
		return "    " + colors.Underline + colors.BrightBlue + "https://console/" + s + colors.Reset + "\n"
	}
	link := func(path, text string) string { return colors.Raw.Hyperlink("https://console/"+path, text) }

	assert.Equal(t, "", format(nil, raw(true)))

	// colors.Never renders a plain-text section, as used by `pulumi stack`.
	assert.Equal(t,
		"Downstream Stacks: (2)\n"+
			"    https://console/org/net/dev\n"+
			"    https://console/org/net/prod\n",
		format([]apitype.StackReference{ref("org", "net", "prod"), ref("org", "net", "dev")},
			Options{Color: colors.Never, IsInteractive: true}))

	// Non-interactive output lists one console URL per stack.
	assert.Equal(t,
		header("Downstream Stack: (1)")+urlLine("org/net/dev"),
		format([]apitype.StackReference{ref("org", "net", "dev")}, raw(false)))
	assert.Equal(t,
		header("Downstream Stacks: (6)")+
			urlLine("org/net/a")+urlLine("org/net/b")+urlLine("org/net/c")+urlLine("org/net/d")+urlLine("org/net/e")+
			"    ...\n",
		format([]apitype.StackReference{
			ref("org", "net", "a"), ref("org", "net", "b"), ref("org", "net", "c"),
			ref("org", "net", "d"), ref("org", "net", "e"), ref("org", "net", "f"),
		}, raw(false)))

	// Interactive output groups by org/project, sorted, with hyperlinked names.
	assert.Equal(t,
		header("Downstream Stacks: (4)")+
			"    "+link("org/app", "org/app")+": "+link("org/app/dev", "dev")+"\n"+
			"    "+link("org/net", "org/net")+": "+link("org/net/dev", "dev")+", "+link("org/net/prod", "prod")+"\n"+
			"    "+link("other/svc", "other/svc")+": "+link("other/svc/prod", "prod")+"\n",
		format([]apitype.StackReference{
			ref("org", "net", "prod"), ref("other", "svc", "prod"), ref("org", "net", "dev"), ref("org", "app", "dev"),
		}, raw(true)))

	// Interactive output truncates by project line.
	assert.Equal(t,
		header("Downstream Stacks: (6)")+
			"    "+link("org/p1", "org/p1")+": "+link("org/p1/dev", "dev")+"\n"+
			"    "+link("org/p2", "org/p2")+": "+link("org/p2/dev", "dev")+"\n"+
			"    "+link("org/p3", "org/p3")+": "+link("org/p3/dev", "dev")+"\n"+
			"    "+link("org/p4", "org/p4")+": "+link("org/p4/dev", "dev")+"\n"+
			"    "+link("org/p5", "org/p5")+": "+link("org/p5/dev", "dev")+"\n"+
			"    ...\n",
		format([]apitype.StackReference{
			ref("org", "p1", "dev"), ref("org", "p2", "dev"), ref("org", "p3", "dev"),
			ref("org", "p4", "dev"), ref("org", "p5", "dev"), ref("org", "p6", "dev"),
		}, raw(true)))
}
