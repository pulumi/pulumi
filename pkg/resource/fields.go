// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"reflect"

	"github.com/pulumi/lumi/pkg/util/mapper"
)

// NewErrors creates a new error list pertaining to a resource.  Note that it just turns around and defers to
// the same mapping infrastructure used for serialization and deserialization, but it presents a nicer interface.
func NewErrors(errs []error) error {
	return mapper.NewMappingError(errs)
}

// NewFieldError creates a new error pertaining to a resource's field.  Note that it just turns around and defers to
// the same mapping infrastructure used for serialization and deserialization, but it presents a nicer interface.
func NewFieldError(ty reflect.Type, fld string, err error) error {
	return mapper.NewFieldError(ty, fld, err)
}
