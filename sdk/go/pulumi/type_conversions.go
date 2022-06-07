// Copyright 2016-2022, Pulumi Corporation.
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

package pulumi

// StringRef returns a pointer to its argument, used in cases where a parameter requires an optional
// string.
func StringRef(v string) *string {
	return &v
}

// BoolRef returns a pointer to its argument, used in cases where a parameter requires an optional
// bool.
func BoolRef(v bool) *bool {
	return &v
}

// IntRef returns a pointer to its argument, used in cases where a parameter requires an optional
// int.
func IntRef(v int) *int {
	return &v
}

// Float64Ref returns a pointer to its argument, used in cases where a parameter requires an optional
// float64.
func Float64Ref(v float64) *float64 {
	return &v
}
