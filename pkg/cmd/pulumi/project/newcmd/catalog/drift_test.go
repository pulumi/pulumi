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

package catalog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/project/newcmd/catalog"
	cmdTemplates "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/templates"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func TestCuratedTemplatesExistUpstream(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipped in short test run: clones pulumi/templates")
	}

	source := cmdTemplates.New(t.Context(), "", cmdTemplates.ScopeLocal, workspace.TemplateKindPulumiProject, env.Global())
	defer contract.IgnoreClose(source)

	templates, err := source.Templates()
	require.NoError(t, err)
	require.NotEmpty(t, templates, "no templates fetched; cannot validate the catalog")

	upstream := make(map[string]bool, len(templates))
	for _, tmpl := range templates {
		upstream[tmpl.Name()] = true
	}

	all := append(catalog.Featured(), catalog.Others()...)
	for _, p := range all {
		for _, l := range p.Languages {
			name, ok := catalog.Resolve(p.ID, l.ID)
			require.True(t, ok, "Resolve(%q, %q) failed for a language the catalog lists", p.ID, l.ID)
			assert.True(t, upstream[name],
				"catalog lists %q but pulumi/templates has no such template; "+
					"the template was renamed or removed and the catalog is stale", name)
		}
	}
}
