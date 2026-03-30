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

package docs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHrefToPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		href string
		want string
	}{
		{name: "docs prefix stripped", href: "/docs/iac/concepts/stacks/", want: "iac/concepts/stacks"},
		{name: "registry prefix kept", href: "/registry/packages/aws/", want: "registry/packages/aws"},
		{name: "query params stripped", href: "/docs/install?ref=nav", want: "install"},
		{name: "fragment stripped", href: "/docs/install#linux", want: "install"},
		{name: "both query and fragment", href: "/docs/install?a=1#top", want: "install"},
		{name: "bare path", href: "/docs/", want: "docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, hrefToPath(tt.href))
		})
	}
}
