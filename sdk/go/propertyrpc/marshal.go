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

// MarshalOptions controls how [MarshalWithOptions] and [MarshalValueWithOptions] serialize values.
type MarshalOptions struct {
	// PreferOutputValues causes secrets and computed (unknown) values to be emitted as OutputValue
	// envelopes (with empty dependencies) rather than Secret / unknown sentinels. Values that
	// already carry dependencies are always emitted as OutputValue envelopes regardless of this
	// setting. Use this when sending to a peer that negotiated OutputValue support and expects to
	// receive outputs on the wire, so that it can round-trip the "this value was/is an output"
	// signal rather than observing a bare secret.
	PreferOutputValues bool
}

func Marshal(m property.Map) *structpb.Struct {
	return MarshalWithOptions(m, MarshalOptions{})
}

func MarshalWithOptions(m property.Map, opts MarshalOptions) *structpb.Struct {
	fields := make(map[string]*structpb.Value, m.Len())
	for k, v := range m.All {
		fields[k] = MarshalValueWithOptions(v, opts)
	}
	return &structpb.Struct{Fields: fields}
}

func MarshalValue(v property.Value) *structpb.Value {
	return MarshalValueWithOptions(v, MarshalOptions{})
}

func MarshalValueWithOptions(v property.Value, opts MarshalOptions) *structpb.Value {
	// If we have dependencies, this must be an output. When PreferOutputValues is set, also wrap
	// bare secrets and computed values as OutputValue envelopes so the peer sees them as outputs.
	if deps := v.Dependencies(); len(deps) > 0 ||
		(opts.PreferOutputValues && (v.Secret() || v.IsComputed())) {
		fields := map[string]*structpb.Value{
			sig.Key: structpb.NewStringValue(sig.OutputValue),
		}
		if !v.IsComputed() {
			fields["value"] = marshalPlainValue(v, opts)
		}
		if v.Secret() {
			fields["secret"] = trueValue
		}

		urns := make([]*structpb.Value, len(deps))
		for i, urn := range deps {
			urns[i] = structpb.NewStringValue(string(urn))
		}
		fields["dependencies"] = structpb.NewListValue(&structpb.ListValue{Values: urns})

		return structpb.NewStructValue(&structpb.Struct{Fields: fields})
	}

	if v.Secret() {
		return structpb.NewStructValue(&structpb.Struct{Fields: map[string]*structpb.Value{
			sig.Key: structpb.NewStringValue(sig.Secret),
			"value": marshalPlainValue(v, opts),
		}})
	}

	return marshalPlainValue(v, opts)
}

func marshalPlainValue(v property.Value, opts MarshalOptions) *structpb.Value {
	switch {
	case v.IsBool():
		return structpb.NewBoolValue(v.AsBool())
	case v.IsNumber():
		return structpb.NewNumberValue(v.AsNumber())
	case v.IsString():
		return marshalString(v.AsString())
	case v.IsArray():
		return structpb.NewListValue(marshalArray(v.AsArray(), opts))
	case v.IsMap():
		return structpb.NewStructValue(MarshalWithOptions(v.AsMap(), opts))
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

// N.B. While there are similar sentinel values for other types of unknown values in practice we only ever use the
// string one, and the non-Go SDKs have only _ever_ sent the unknown string sentinel.
var computedValue = structpb.NewStringValue(sig.UnknownStringValue)

var trueValue = structpb.NewBoolValue(true)

func marshalArray(array property.Array, opts MarshalOptions) *structpb.ListValue {
	values := make([]*structpb.Value, array.Len())
	for i, v := range array.All {
		values[i] = MarshalValueWithOptions(v, opts)
	}
	return &structpb.ListValue{Values: values}
}
