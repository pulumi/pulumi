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

package property

import "github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"

// ResourceReference is a property value that represents a reference to a Resource. The reference captures the
// resource's URN, ID, and the version of its containing package. Note that there are several cases to consider with
// respect to the ID:
//
//   - The reference may not contain an ID if the referenced resource is a component resource. In this case, the ID will
//     be Null.
//   - The ID may be unknown (in which case it will be the Computed property value)
//   - Otherwise, the ID must be a string.
type ResourceReference struct {
	URN            urn.URN
	ID             Value
	PackageVersion string
}

func (ref ResourceReference) IDString() (value string, hasID bool) {
	switch {
	case ref.ID.IsComputed():
		return "", true
	case ref.ID.IsString():
		return ref.ID.AsString(), true
	default:
		return "", false
	}
}

func (ref ResourceReference) Equal(other ResourceReference) bool {
	if ref.URN != other.URN {
		return false
	}
	if ref.PackageVersion != other.PackageVersion {
		return false
	}

	vid, oid := ref.ID, other.ID
	return vid.Equals(oid, EqualRelaxComputed)
}
