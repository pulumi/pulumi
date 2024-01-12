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

type valueType uint8

const (
	typeUnknown           valueType = 0
	typeBool              valueType = iota
	typeNumber            valueType = iota
	typeString            valueType = iota
	typeArray             valueType = iota
	typeMap               valueType = iota
	typeAsset             valueType = iota
	typeArchive           valueType = iota
	typeResourceReference valueType = iota
)

func (v Value) typ() valueType {
	switch {
	case v.IsBool():
		return typeBool
	case v.IsNumber():
		return typeNumber
	case v.IsString():
		return typeString
	case v.IsArray():
		return typeArray
	case v.IsMap():
		return typeMap
	case v.IsAsset():
		return typeAsset
	case v.IsArchive():
		return typeArchive
	case v.IsResourceReference():
		return typeResourceReference
	default:
		return typeUnknown
	}
}
