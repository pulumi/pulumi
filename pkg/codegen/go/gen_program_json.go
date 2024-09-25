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

package gen

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

type jsonSpiller struct{}

func (g *generator) rewriteToJSON(x model.Expression) (model.Expression, []*spillTemp, hcl.Diagnostics) {
	return g.rewriteSpills(x, func(x model.Expression) (string, model.Expression, bool) {
		if call, ok := x.(*model.FunctionCallExpression); ok && call.Name == "toJSON" {
			return "json", x, true
		}
		return "", nil, false
	})
}
