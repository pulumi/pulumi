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

package plugin

import (
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/propertyrpc"
)

// The analyzer protocol has no output values: policy packs read and write an unknown value as a sentinel string,
// such as sig.UnknownStringValue. propertyrpc instead encodes an unknown as an output value without a value, so the
// analyzer converts between the two encodings.

// marshalAnalyzerProperties marshals properties for a policy pack, with unknowns as sentinel strings.
func marshalAnalyzerProperties(m property.Map) *structpb.Struct {
	return rewriteStruct(propertyrpc.Marshal(m), unknownToSentinel)
}

// unmarshalAnalyzerProperties unmarshals properties from a policy pack, reading sentinel strings as unknowns.
func unmarshalAnalyzerProperties(s *structpb.Struct) (property.Map, error) {
	return propertyrpc.Unmarshal(rewriteStruct(s, sentinelToUnknown))
}

// unknownToSentinel replaces an output value without a value with the unknown string sentinel, keeping it secret if
// the output value was. Analyzer properties never carry dependencies, so nothing else is lost.
func unknownToSentinel(v *structpb.Value) (*structpb.Value, bool) {
	fields := v.GetStructValue().GetFields()
	if fields[sig.Key].GetStringValue() != sig.OutputValue || fields["value"] != nil {
		return nil, false
	}
	unknown := structpb.NewStringValue(sig.UnknownStringValue)
	if !fields["secret"].GetBoolValue() {
		return unknown, true
	}
	return structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
		sig.Key: structpb.NewStringValue(sig.Secret),
		"value": unknown,
	}}), true
}

// sentinelToUnknown replaces an unknown sentinel string with an output value without a value.
func sentinelToUnknown(v *structpb.Value) (*structpb.Value, bool) {
	if !sig.IsUnknown(v.GetStringValue()) {
		return nil, false
	}
	return structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
		sig.Key: structpb.NewStringValue(sig.OutputValue),
	}}), true
}

// rewriteStruct returns a copy of s in which f replaces values. f sees each value before the values nested in it,
// and a replacement isn't descended into.
func rewriteStruct(s *structpb.Struct, f func(*structpb.Value) (*structpb.Value, bool)) *structpb.Struct {
	fields := make(map[string]*structpb.Value, len(s.GetFields()))
	for k, v := range s.GetFields() {
		fields[k] = rewriteValue(v, f)
	}
	return &structpb.Struct{Fields: fields}
}

func rewriteValue(v *structpb.Value, f func(*structpb.Value) (*structpb.Value, bool)) *structpb.Value {
	if replacement, ok := f(v); ok {
		return replacement
	}
	switch kind := v.GetKind().(type) {
	case *structpb.Value_StructValue:
		return structpb.NewStructValue(rewriteStruct(kind.StructValue, f))
	case *structpb.Value_ListValue:
		values := make([]*structpb.Value, len(kind.ListValue.GetValues()))
		for i, e := range kind.ListValue.GetValues() {
			values[i] = rewriteValue(e, f)
		}
		return structpb.NewListValue(&structpb.ListValue{Values: values})
	default:
		return v
	}
}
