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

package tests

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/require"
)

func init() {
	LanguageTests["l1-builtin-file"] = LanguageTest{
		Runs: []TestRun{
			{
				Assert: func(l *L, res AssertArgs) {
					RequireStackResource(l, res.Err, res.Changes)
					stack := RequireSingleResource(l, res.Snap.Resources, "pulumi:pulumi:Stack")

					outputs := resource.FromResourcePropertyMap(stack.Outputs)
					require.Equal(l, 3, outputs.Len(), "expected 3 outputs")

					AssertPropertyMapMember(l, outputs, "fileContent",
						property.New("The quick brown fox jumps over the lazy dog"))

					AssertPropertyMapMember(l, outputs, "fileB64",
						property.New("VGhlIHF1aWNrIGJyb3duIGZveCBqdW1wcyBvdmVyIHRoZSBsYXp5IGRvZw=="))

					AssertPropertyMapMember(l, outputs, "fileSha",
						property.New("16j7swfXgJRpypq8sAguT41WUeRtPNt2LQLQvzfJ5ZI="))
				},
			},
		},
	}
}
