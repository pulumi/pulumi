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

package cloudsetup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOIDCClaims(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		env            string
		wantAudience   string
		wantSubjectEnv string
	}{
		{name: "project", env: "aws-login/prod", wantAudience: "aws:acme", wantSubjectEnv: "aws-login/prod"},
		// Environments in the legacy default project present an unprefixed audience and drop
		// the project from the subject.
		{name: "default project", env: "default/prod", wantAudience: "acme", wantSubjectEnv: "prod"},
		// A bare name carries no project, so it is not the default-project shape.
		{name: "no project", env: "prod", wantAudience: "aws:acme", wantSubjectEnv: "prod"},
		{name: "no environment", env: "", wantAudience: "aws:acme", wantSubjectEnv: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			audience, subjectEnv := OIDCClaims("aws", "acme", c.env)
			assert.Equal(t, c.wantAudience, audience)
			assert.Equal(t, c.wantSubjectEnv, subjectEnv)
		})
	}
}
