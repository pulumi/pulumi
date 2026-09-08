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

package propertyrpc

import (
	"encoding/base64"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func Marshal(m property.Map) *structpb.Struct {
	fields := make(map[string]*structpb.Value, m.Len())
	for k, v := range m.All {
		fields[k] = MarshalValue(v)
	}
	return &structpb.Struct{Fields: fields}
}

func MarshalValue(v property.Value) *structpb.Value {
	// If we have dependencies, this must be an output
	if deps := v.Dependencies(); len(deps) > 0 || (v.Secret() && v.IsComputed()) {
		fields := map[string]*structpb.Value{
			sig.Key: structpb.NewStringValue(sig.OutputValue),
		}
		if !v.IsComputed() {
			fields["value"] = marshalPlainValue(v)
		}
		if v.Secret() {
			fields["secret"] = trueValue
		}
		if len(deps) > 0 {
			urns := make([]*structpb.Value, len(deps))
			for i, urn := range deps {
				urns[i] = structpb.NewStringValue(string(urn))
			}
			fields["dependencies"] = structpb.NewListValue(&structpb.ListValue{Values: urns})
		}
		return structpb.NewStructValue(&structpb.Struct{Fields: fields})
	}

	if v.Secret() {
		return structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
			sig.Key: structpb.NewStringValue(sig.Secret),
			"value": marshalPlainValue(v),
		}})
	}

	return marshalPlainValue(v)
}

func marshalPlainValue(v property.Value) *structpb.Value {
	switch {
	case v.IsBool():
		return structpb.NewBoolValue(v.AsBool())
	case v.IsNumber():
		return structpb.NewNumberValue(v.AsNumber())
	case v.IsString():
		return marshalString(v.AsString())
	case v.IsArray():
		return structpb.NewListValue(marshalArray(v.AsArray()))
	case v.IsMap():
		return structpb.NewStructValue(Marshal(v.AsMap()))
	case v.IsAsset():
		return marshalSerialized(v.AsAsset().Serialize())
	case v.IsArchive():
		return marshalSerialized(v.AsArchive().Serialize())
	case v.IsResourceReference():
		return marshalResourceReference(v.AsResourceReference())
	case v.IsNull():
		return structpb.NewNullValue()
	case v.IsComputed():
		return computedValue
	default:
		contract.Failf("%T of unknown type %#v", v, v)
		return nil
	}
}

// marshalString marshals a string. A protobuf string field must contain valid UTF-8, so a string
// with other bytes is marshalled as the base64 encoding of its bytes.
func marshalString(s string) *structpb.Value {
	if utf8.ValidString(s) {
		return structpb.NewStringValue(s)
	}
	return structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
		sig.Key: structpb.NewStringValue(sig.ByteString),
		"value": structpb.NewStringValue(base64.StdEncoding.EncodeToString([]byte(s))),
	}})
}

// marshalSerialized marshals the weakly typed map produced by [asset.Asset.Serialize] and
// [archive.Archive.Serialize]. Those maps contain only strings and nested serialized maps.
func marshalSerialized(m map[string]any) *structpb.Value {
	s, err := structpb.NewStruct(m)
	contract.AssertNoErrorf(err, "failed to marshal %#v", m)
	return structpb.NewStructValue(s)
}

func marshalResourceReference(ref property.ResourceReference) *structpb.Value {
	fields := map[string]*structpb.Value{
		sig.Key: structpb.NewStringValue(sig.ResourceReference),
		"urn":   structpb.NewStringValue(string(ref.URN)),
	}
	if ref.Name != "" {
		fields["name"] = structpb.NewStringValue(ref.Name)
	}
	if ref.Type != "" {
		fields["type"] = structpb.NewStringValue(ref.Type)
	}
	if id, hasID := ref.IDString(); hasID {
		fields["id"] = structpb.NewStringValue(id)
	}
	if ref.PackageVersion != "" {
		fields["packageVersion"] = structpb.NewStringValue(ref.PackageVersion)
	}
	return structpb.NewStructValue(&structpb.Struct{Fields: fields})
}

var computedValue = structpb.NewStructValue(&structpb.Struct{
	Fields: map[string]*structpb.Value{
		sig.Key: structpb.NewStringValue(sig.OutputValue),
	},
})

var trueValue = structpb.NewBoolValue(true)

func marshalArray(array property.Array) *structpb.ListValue {
	values := make([]*structpb.Value, array.Len())
	for i, v := range array.All {
		values[i] = MarshalValue(v)
	}
	return &structpb.ListValue{Values: values}
}
