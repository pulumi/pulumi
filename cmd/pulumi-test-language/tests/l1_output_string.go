// Copyright 2024, Pulumi Corporation.
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

package tests

import (
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
)

func init() {
	LanguageTests["l1-output-string"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L,
					projectDirectory string, err error,
					snap *deploy.Snapshot, changes display.ResourceChanges,
				) {
					RequireStackResource(l, err, changes)
					stack := RequireSingleResource(l, snap.Resources, "pulumi:pulumi:Stack")

					outputs := stack.Outputs

					assert.Len(l, outputs, 6, "expected 6 outputs")
					AssertPropertyMapMember(l, outputs, "empty", resource.NewStringProperty(""))
					AssertPropertyMapMember(l, outputs, "small", resource.NewStringProperty("Hello world!"))
					AssertPropertyMapMember(l, outputs, "emoji", resource.NewStringProperty("👋 \"Hello \U0001019b!\" 😊"))
					AssertPropertyMapMember(l, outputs, "escape", resource.NewStringProperty(
						"Some ${common} \"characters\" 'that' need escaping: "+
							"\\ (backslash), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"))
					AssertPropertyMapMember(l, outputs, "escapeNewline", resource.NewStringProperty(
						"Some ${common} \"characters\" 'that' need escaping: "+
							"\\ (backslash), \n (newline), \t (tab), \u001b (escape), \u0007 (bell), \u0000 (null), \U000e0021 (tag space)"))

					large := strings.Repeat(lorem+"\n", 150)
					AssertPropertyMapMember(l, outputs, "large", resource.NewStringProperty(large))
				},
			},
		},
	}
}
