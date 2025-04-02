// Copyright 2016-2024, Pulumi Corporation.
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

// Visit each Value in the within v.
//
// Parents are visited before their children.
func (v Value) visit(f func(Value) (continueWalking bool)) bool {
	cont := f(v)
	if !cont {
		return false
	}
	switch {
	case v.IsArray():
		for _, v := range v.AsArray().arr {
			if !v.visit(f) {
				return false
			}
		}
	case v.IsMap():
		for _, v := range v.AsMap().m {
			if !v.visit(f) {
				return false
			}
		}
	}
	return true
}
