// Copyright 2022-2024, Pulumi Corporation.
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

package report_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/dotnet"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/report"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func TestReportExample(t *testing.T) {
	t.Parallel()

	reporter := report.New("example", version.Version)
	defer reporter.Close()

	examples := []struct {
		title string
		body  string
	}{
		{"Our basic bucket", `resource bucket "aws:s3:BucketV2" {  }`},
		{"A resource group", `resource group "azure:core:ResourceGroup" { location: "westus2" }`},
		{"Might not bind", `resource foo "not:a:Resource" { foo: "bar" }`},
	}
	for _, example := range examples {
		parser := syntax.NewParser()
		err := parser.ParseFile(bytes.NewReader([]byte(example.body)), example.title)
		require.NoError(t, err, "parse failed")
		program, diags, err := pcl.BindProgram(parser.Files, pcl.PluginHost(utils.NewHost(testdataPath)))
		if err != nil || diags.HasErrors() {
			reporter.Report(example.title, "", parser.Files, diags, err)
			continue
		}

		langs := []string{"dotnet", "nodejs"}
		for i, genFn := range []report.GenerateProgramFn{dotnet.GenerateProgram, nodejs.GenerateProgram} {
			program, diags, err := report.WrapGen(reporter, example.title, langs[i], parser.Files, genFn)(program)
			handleAsNormal(program, diags, err)
		}
	}

	assert.Equal(t, report.Summary{
		Name: "example",
		Stats: report.Stats{
			NumConversions: 5,
			Successes:      4,
		},
		Languages: map[string]*report.Language{
			"": {
				Stats: report.Stats{
					NumConversions: 1,
					Successes:      0,
				},
				GoErrors: map[string]string{
					"Might not bind": "error: could not locate a compatible plugin in " +
						"deploytest, the makefile and & constructor of the plugin host " +
						"must define the location of the schema",
				},
				Files: map[string][]report.File{
					"Might not bind": {{Name: "Might not bind", Body: "resource foo \"not:a:Resource\" { foo: \"bar\" }"}},
				},
			},
			"dotnet": {
				Stats: report.Stats{
					NumConversions: 2,
					Successes:      2,
				},
			},
			"nodejs": {
				Stats: report.Stats{
					NumConversions: 2,
					Successes:      2,
				},
			},
		},
	}, reporter.Summary())
}

func handleAsNormal(args ...interface{}) {}
