// Copyright 2016-2023, Pulumi Corporation.
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

package tokens

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateProjectName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc string
		give string

		// Expect success if wantErr is empty.
		wantErr string
	}{
		{desc: "valid", give: "foo"},
		{desc: "empty", wantErr: "project names may not be empty"},
		{
			desc:    "too long",
			give:    strings.Repeat("a", 101),
			wantErr: "project names are limited to 100 characters",
		},
		{
			desc:    "not a name",
			give:    "foo bar",
			wantErr: "project names may only contain alphanumerics, hyphens, underscores, and periods",
		},
		{
			desc:    "Correct Project Name",
			give:    "SampleProject",
			wantErr: "",
		},
		{
			desc:    "Project Name with unsupported punctuation",
			give:    "SampleProject!",
			wantErr: "project names may only contain alphanumerics, hyphens, underscores, and periods",
		},
		{
			desc:    "Project Name starting with the word Pulumi",
			give:    "PulumiProject",
			wantErr: "",
		},
		{
			desc:    "Project Name greater than 100 characters",
			give:    "cZClTe6xrjgKzH5QS8rFEPqYK1z4bbMeMr6n89n87djq9emSAlznQXXkkCEpBBCaZAFNlCvbfqVcqoifYlfPl11hvekIDjXVIY7m1",
			wantErr: "project names are limited to 100 characters",
		},
		{
			desc:    "Empty Project Name",
			give:    "",
			wantErr: "project names may not be empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			err := ValidateProjectName(tt.give)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
