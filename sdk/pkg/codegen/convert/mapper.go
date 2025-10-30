// Copyright 2016-2025, Pulumi Corporation.
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

package convert

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Mapper provides methods for retrieving mappings that describe how to map names in some source "provider" (e.g. a
// Terraform provider, if we are converting from Terraform) to names in appropriate Pulumi packages. So when converting
// a Terraform program containing code like `resource "aws_s3_bucket" "b" {}`, for instance, we need to know (among
// other things) that the `aws_s3_bucket` Terraform resource type corresponds to the Pulumi type `aws:s3/bucket:Bucket`,
// and thus lives in the `aws` package. This is the kind of information that a Mapper provides.
type Mapper interface {
	// GetMapping returns any available mapping data for the given source provider name (so again, this is e.g. the name
	// of a Terraform provider if converting from Terraform). Callers may pass a "hint" parameter that describes a Pulumi
	// package that is expected to provide the mapping and satisfy the request, which implementations may use to optimise
	// their efforts to return the best possible mapping. If no matching mapping exists, implementations should return an
	// empty byte array result.
	GetMapping(ctx context.Context, provider string, hint *MapperPackageHint) ([]byte, error)
}

// MapperPackageHint is the type of hints that may be passed to GetMapping to help guide implementations to picking
// appropriate Pulumi packages to satisfy mapping requests.
type MapperPackageHint struct {
	// The name of the Pulumi plugin that is expected to provide the mapping.
	PluginName string

	// An optional parameterization that should be used on the named plugin before asking it for mappings. E.g. in the
	// case of a dynamically bridged Terraform provider, callers may wish to express that a mapping is most likely offered
	// by the "terraform-provider" plugin, but only when it is parameterized with the appropriate Terraform provider
	// information.
	Parameterization *workspace.Parameterization
}
