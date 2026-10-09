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

package rapidpcl

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
)

func TestQuote(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		e, diags := hclsyntax.ParseExpression([]byte(Quote(s)), "test.pp", hcl.InitialPos)
		require.False(t, diags.HasErrors(), diags.Error())
		v, diags := e.Value(nil)
		require.False(t, diags.HasErrors(), diags.Error())
		// HCL normalizes string literals to NFC.
		require.Equal(t, norm.NFC.String(s), v.AsString())
	})
}

func TestProgramBinds(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		sample := Program().Draw(t, "program")
		parser := syntax.NewParser()
		require.NoError(t, parser.ParseFile(strings.NewReader(sample.Source), "main.pp"))
		require.False(t, parser.Diagnostics.HasErrors(), "%s\n%s", parser.Diagnostics.Error(), sample.Source)
		_, diags, err := pcl.BindProgram(parser.Files, schema.NewNullLoader())
		require.NoError(t, err)
		require.False(t, diags.HasErrors(), "%s\n%s", diags.Error(), sample.Source)
	})
}
