// Copyright 2016-2020, Pulumi Corporation.
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
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
)

var (
	// ArchiveType represents the set of Pulumi Archive values.
	ArchiveType model.Type = model.NewOpaqueType("Archive")
	// AssetType represents the set of Pulumi Asset values.
	AssetType model.Type = model.NewOpaqueType("Asset")
	// ResourcePropertyType represents a resource property reference.
	ResourcePropertyType model.Type = model.NewOpaqueType("Property")
	// AssetOrArchiveType represents the set of Pulumi Archive values.
	AssetOrArchiveType model.Type = model.NewUnionType(ArchiveType, AssetType)
	// AliasType represents a type for the alias resource option. Aliases are either a string (single URN) or an object
	// (with "name" and "parent", etc fields).
	AliasType = model.NewUnionType(model.StringType, model.NewObjectType(map[string]model.Type{
		"name":     model.NewOptionalType(model.StringType),
		"noParent": model.NewOptionalType(model.BoolType),
		"parent":   model.NewOptionalType(model.DynamicType),
	}))
	// CustomTimeoutsType represents the type for custom timeouts resource option.
	CustomTimeoutsType = model.NewObjectType(map[string]model.Type{
		"create": model.NewOptionalType(model.StringType),
		"update": model.NewOptionalType(model.StringType),
		"delete": model.NewOptionalType(model.StringType),
	})
)
