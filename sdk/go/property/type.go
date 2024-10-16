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

import "fmt"

// typeString provides a type name for v.
//
// typeString should be used only to generate error messages. Do not rely on typeString
// providing stable output.
func typeString(v Value) string {
	switch {
	case v.IsArchive():
		return "archive"
	case v.IsArray():
		return "array"
	case v.IsAsset():
		return "asset"
	case v.IsBool():
		return "bool"
	case v.IsComputed():
		return "computed"
	case v.IsMap():
		return "map"
	case v.IsNull():
		return "null"
	case v.IsNumber():
		return "number"
	case v.IsResourceReference():
		return "resource reference"
	case v.IsString():
		return "string"
	default:
		panic(fmt.Sprintf("unknown type %T", v.v))
	}
}
