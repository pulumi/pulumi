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

package deploy

import (
	"reflect"

	"github.com/mitchellh/copystructure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func init() {
	// [property.Glob] and [resource.BackCompatPropertyPath] wrap an internal pathRepr which holds
	// its data in an unexported embedded string. reflect-based deep copy cannot set that field,
	// so we register value-copy functions that return the input unchanged (both types are
	// effectively immutable).
	copystructure.Copiers[reflect.TypeFor[property.Glob]()] = func(v any) (any, error) {
		return v.(property.Glob), nil
	}
	copystructure.Copiers[reflect.TypeFor[property.Path]()] = func(v any) (any, error) {
		return v.(property.Path), nil
	}
	copystructure.Copiers[reflect.TypeFor[resource.BackCompatPropertyPath]()] = func(v any) (any, error) {
		return v.(resource.BackCompatPropertyPath), nil
	}
}
