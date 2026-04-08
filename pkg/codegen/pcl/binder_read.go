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

package pcl

import (
	"context"
	"fmt"
	"slices"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/maputil"
	"github.com/zclconf/go-cty/cty"
)

func (b *binder) bindRead(ctx context.Context, node *Read) hcl.Diagnostics {
	var diagnostics hcl.Diagnostics

	typeDiags := b.bindResourceTypes(ctx, node)
	diagnostics = append(diagnostics, typeDiags...)

	bodyDiags := b.bindResourceBody(node)
	diagnostics = append(diagnostics, bodyDiags...)

	return diagnostics
}
