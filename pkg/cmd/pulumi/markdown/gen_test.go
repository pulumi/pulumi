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

package markdown

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestAliasRedirectPaths(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "pulumi"}
	stack := &cobra.Command{Use: "stack"}
	remove := &cobra.Command{Use: "remove", Aliases: []string{"rm"}}
	noAlias := &cobra.Command{Use: "get"}
	multi := &cobra.Command{Use: "destroy", Aliases: []string{"down", "dn"}}
	root.AddCommand(stack)
	stack.AddCommand(remove, noAlias)
	root.AddCommand(multi)

	// A command emits redirects for its aliases under both prefixes
	assert.Equal(t, []string{
		"/docs/iac/cli/commands/pulumi_stack_rm/",
		"/docs/reference/cli/pulumi_stack_rm/",
	}, aliasRedirectPaths(remove))

	// Multiple aliases each get a pair of redirects
	assert.Equal(t, []string{
		"/docs/iac/cli/commands/pulumi_down/",
		"/docs/reference/cli/pulumi_down/",
		"/docs/iac/cli/commands/pulumi_dn/",
		"/docs/reference/cli/pulumi_dn/",
	}, aliasRedirectPaths(multi))

	// Commands without aliases (and nil) produce nothing.
	assert.Nil(t, aliasRedirectPaths(noAlias))
	assert.Nil(t, aliasRedirectPaths(nil))
}
