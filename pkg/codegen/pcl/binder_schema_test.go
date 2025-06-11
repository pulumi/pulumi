// Copyright 2020-2024, Pulumi Corporation.
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

package pcl

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/assert"
	"github.com/zclconf/go-cty/cty"
)

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

func BenchmarkLoadPackage(b *testing.B) {
	loader := schema.NewPluginLoader(utils.NewHost(testdataPath))

	for n := 0; n < b.N; n++ {
		_, err := NewPackageCache().loadPackageSchema(context.Background(), loader, "aws", "", "")
		if err != nil {
			b.Fatalf("failed to load package schema: %v", err)
		}
	}
}

func TestGenEnum(t *testing.T) {
	t.Parallel()
	enum := model.NewEnumType(
		"my.enum", model.StringType,
		[]cty.Value{
			cty.StringVal("foo"),
			cty.StringVal("bar"),
		},
		enumSchemaType{
			Type: &schema.EnumType{Elements: []*schema.Enum{{Value: "foo"}, {Value: "bar"}}},
		},
	)
	safeEnumFunc := func(member *schema.Enum) {}
	unsafeEnumFunc := func(from model.Expression) {}

	d := GenEnum(enum, &model.LiteralValueExpression{
		Value: cty.StringVal("foo"),
	}, safeEnumFunc, unsafeEnumFunc)
	assert.Nil(t, d)

	d = GenEnum(enum, &model.LiteralValueExpression{
		Value: cty.StringVal("Bar"),
	}, safeEnumFunc, unsafeEnumFunc)
	assert.Equal(t, d.Summary, `"Bar" is not a valid value of the enum "my:enum"`)
	assert.Equal(t, d.Detail, `Valid members are "foo", "bar"`)
}
