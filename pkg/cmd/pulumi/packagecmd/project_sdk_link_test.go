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

package packagecmd

import (
	"bytes"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/nodejs"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/stretchr/testify/assert"
)

func TestPrintNodeJsImportInstructions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		pkg            *schema.Package
		options        map[string]interface{}
		wantImportLine string
	}{
		{
			name: "uses package info name when available",
			pkg: &schema.Package{
				Name: "aws-native",
				Language: map[string]interface{}{
					"nodejs": nodejs.NodePackageInfo{
						PackageName: "awsnative",
					},
				},
			},
			options:        map[string]interface{}{},
			wantImportLine: "import * as awsnative from \"@pulumi/aws-native\";\n",
		},
		{
			name: "falls back to camelCase when no package info",
			pkg: &schema.Package{
				Name: "aws-native",
			},
			options:        map[string]interface{}{},
			wantImportLine: "import * as awsNative from \"@pulumi/aws-native\";\n",
		},
		{
			name: "respects typescript option",
			pkg: &schema.Package{
				Name: "aws-native",
			},
			options: map[string]interface{}{
				"typescript": false,
			},
			wantImportLine: "  const awsNative = require(\"@pulumi/aws-native\");\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			err := printNodeJsImportInstructions(&buf, tt.pkg, tt.options)
			assert.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, tt.wantImportLine, "output should contain the import line")
		})
	}
}
