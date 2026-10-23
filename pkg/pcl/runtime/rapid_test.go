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

package runtime

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/pcl/rapidpcl"
)

// TestRapidPrograms checks that every generated program binds with the types the generator
// assigned and evaluates without error.
func TestRapidPrograms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	program := rapidpcl.Program()
	rapid.Check(t, func(t *rapid.T) {
		sample := program.Draw(t, "program")
		parser := syntax.NewParser()
		require.NoError(t, parser.ParseFile(strings.NewReader(sample.Source), "main.pp"))
		require.False(t, parser.Diagnostics.HasErrors(), "%s\n%s", parser.Diagnostics.Error(), sample.Source)
		program, diags, err := pcl.BindProgram(parser.Files, schema.NewNullLoader())
		require.NoError(t, err)
		require.False(t, diags.HasErrors(), "%s\n%s", diags.Error(), sample.Source)

		for _, node := range program.Nodes {
			output, ok := node.(*pcl.OutputVariable)
			if !ok {
				continue
			}
			// Tuples of different lengths unify to a tuple with optional trailing elements, which
			// converts to the generator's list type only unsafely.
			want, bound := sample.Outputs[output.Name()], model.ResolveOutputs(output.Value.Type())
			require.True(t, want.ConversionFrom(bound).Exists(),
				"output %s: generator type %v, binder type %v\n%s", output.Name(), want, bound, sample.Source)
		}

		i := &Interpreter{
			program: program,
			info: RunInfo{
				Project:       "project",
				Stack:         "stack",
				Organization:  "organization",
				WorkingDir:    dir,
				RootDirectory: dir,
				Parallel:      4,
			},
			packageRefs: map[string]string{},
		}
		i.evalContext = NewEvalContext(
			i.info.WorkingDir, i.info.RootDirectory, i.info.Organization, i.info.Project, i.info.Stack,
			i.lookupResource, i.lookupFunction, i.getResource, i.invoke, i.call,
		)
		outputs, err := i.executeProgramNodes(t.Context())
		require.NoError(t, err, sample.Source)
		require.Equal(t, len(sample.Outputs), outputs.Len(), sample.Source)
	})
}
