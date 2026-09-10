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
	"fmt"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/asset"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// UnmarshalError describes why a value could not be unmarshalled, and where in the value
// the problem was found.
type UnmarshalError struct {
	Path   property.Path
	Reason error
}

func (e UnmarshalError) Error() string {
	var path string
	if (e.Path == property.Path{}) {
		path = "<root>"
	} else if p, err := e.Path.MarshalText(); err != nil {
		path = "PathError(" + err.Error() + ")"
	} else {
		path = string(p)
	}
	return fmt.Sprintf("%s: %s", path, e.Reason)
}

func (e UnmarshalError) Unwrap() error { return e.Reason }

func Unmarshal(s *structpb.Struct) (property.Map, error) {
	return unmarshalStruct(s, nil)
}

func UnmarshalValue(v *structpb.Value) (property.Value, error) {
	return unmarshalValue(v, nil)
}

// path is the location of the value that is currently unmarshalled, relative to the value
// that [Unmarshal] or [UnmarshalValue] received.
type path = []property.PathSegment

func errAt(p path, msg string, args ...any) error {
	return wrapAt(p, fmt.Errorf(msg, args...))
}

func wrapAt(p path, err error) error {
	return UnmarshalError{Path: property.PathFromSegments(p...), Reason: err}
}

func unmarshalStruct(s *structpb.Struct, p path) (property.Map, error) {
	fields := make(map[string]property.Value, len(s.GetFields()))
	for k, v := range s.GetFields() {
		v, err := unmarshalValue(v, append(p, property.NewSegment(k)))
		if err != nil {
			return property.Map{}, err
		}
		fields[k] = v
	}
	return property.NewMap(fields), nil
}

func unmarshalValue(v *structpb.Value, p path) (property.Value, error) {
	switch v := v.GetKind().(type) {
	case nil, *structpb.Value_NullValue:
		return property.Value{}, nil
	case *structpb.Value_BoolValue:
		return property.New(v.BoolValue), nil
	case *structpb.Value_NumberValue:
		return property.New(v.NumberValue), nil
	case *structpb.Value_StringValue:
		if sig.IsUnknown(v.StringValue) {
			return property.New(property.Computed), nil
		}
		return property.New(v.StringValue), nil
	case *structpb.Value_ListValue:
		return unmarshalArray(v.ListValue, p)
	case *structpb.Value_StructValue:
		return unmarshalStructValue(v.StructValue, p)
	default:
		return property.Value{}, errAt(p, "unknown value kind %T", v)
	}
}

func unmarshalArray(list *structpb.ListValue, p path) (property.Value, error) {
	values := make([]property.Value, len(list.GetValues()))
	for i, v := range list.GetValues() {
		v, err := unmarshalValue(v, append(p, property.NewSegment(i)))
		if err != nil {
			return property.Value{}, err
		}
		values[i] = v
	}
	return property.New(values), nil
}

// unmarshalStructValue unmarshals a struct, which is either a plain map or one of the
// types that [sig.Key] describes.
func unmarshalStructValue(s *structpb.Struct, p path) (property.Value, error) {
	signature, hasSignature := s.GetFields()[sig.Key]
	if !hasSignature {
		m, err := unmarshalStruct(s, p)
		if err != nil {
			return property.Value{}, err
		}
		return property.New(m), nil
	}

	switch signature := signature.GetStringValue(); signature {
	case sig.Secret:
		value, ok := s.GetFields()["value"]
		if !ok {
			return property.Value{}, errAt(p, "malformed secret: missing value")
		}
		// The secret marker is not part of the path, so p is not extended here.
		v, err := unmarshalValue(value, p)
		if err != nil {
			return property.Value{}, err
		}
		return v.WithSecret(true), nil
	case sig.OutputValue:
		return unmarshalOutputValue(s, p)
	case sig.ResourceReference:
		return unmarshalResourceReference(s, p)
	case sig.ByteString:
		return unmarshalByteString(s, p)
	case asset.AssetSig:
		a, _, err := asset.Deserialize(s.AsMap())
		if err != nil {
			return property.Value{}, wrapAt(p, err)
		}
		return property.New(a), nil
	case archive.ArchiveSig:
		a, _, err := archive.Deserialize(s.AsMap())
		if err != nil {
			return property.Value{}, wrapAt(p, err)
		}
		return property.New(a), nil
	default:
		return property.Value{}, errAt(p, "unknown signature %q", signature)
	}
}

// unmarshalByteString unmarshals a string that contains bytes which are not valid UTF-8. Such
// strings are transported as the base64 encoding of their bytes.
func unmarshalByteString(s *structpb.Struct, p path) (property.Value, error) {
	value, ok := s.GetFields()["value"]
	if !ok {
		return property.Value{}, errAt(p, "malformed byte string: missing value")
	}
	str, ok := value.GetKind().(*structpb.Value_StringValue)
	if !ok {
		return property.Value{}, errAt(p, "malformed byte string: value is not a string")
	}
	decoded, err := base64.StdEncoding.DecodeString(str.StringValue)
	if err != nil {
		return property.Value{}, errAt(p, "malformed byte string: value is not valid base64: %s", err)
	}
	return property.New(string(decoded)), nil
}

func unmarshalOutputValue(s *structpb.Struct, p path) (property.Value, error) {
	fields := s.GetFields()

	// An output value without a value is unknown.
	v := property.New(property.Computed)
	if value, known := fields["value"]; known {
		var err error
		// The output marker is not part of the path, so p is not extended here.
		v, err = unmarshalValue(value, p)
		if err != nil {
			return property.Value{}, err
		}
	}

	if secret, ok := fields["secret"]; ok {
		b, ok := secret.GetKind().(*structpb.Value_BoolValue)
		if !ok {
			return property.Value{}, errAt(p, "malformed output value: secret is not a bool")
		}
		v = v.WithSecret(b.BoolValue)
	}

	if dependencies, ok := fields["dependencies"]; ok {
		list, ok := dependencies.GetKind().(*structpb.Value_ListValue)
		if !ok {
			return property.Value{}, errAt(p, "malformed output value: dependencies is not an array")
		}
		urns := make([]urn.URN, len(list.ListValue.GetValues()))
		for i, dependency := range list.ListValue.GetValues() {
			d, ok := dependency.GetKind().(*structpb.Value_StringValue)
			if !ok {
				return property.Value{}, errAt(p, "malformed output value: dependency %d is not a string", i)
			}
			urns[i] = urn.URN(d.StringValue)
		}
		if len(urns) > 0 {
			v = v.WithDependencies(urns)
		}
	}

	return v, nil
}

func unmarshalResourceReference(s *structpb.Struct, p path) (property.Value, error) {
	var err error
	// field returns the value of a string field, and if the field was present.
	field := func(name string) (string, bool) {
		if err != nil {
			return "", false
		}
		v, ok := s.GetFields()[name]
		if !ok {
			return "", false
		}
		str, ok := v.GetKind().(*structpb.Value_StringValue)
		if !ok {
			err = errAt(p, "malformed resource reference: %s is not a string", name)
			return "", false
		}
		return str.StringValue, true
	}

	resourceURN, hasURN := field("urn")
	name, _ := field("name")
	typ, _ := field("type")
	id, hasID := field("id")
	packageVersion, _ := field("packageVersion")
	if err != nil {
		return property.Value{}, err
	}
	if !hasURN {
		return property.Value{}, errAt(p, "malformed resource reference: missing urn")
	}

	ref := property.ResourceReference{
		URN:            urn.URN(resourceURN),
		Name:           name,
		Type:           typ,
		PackageVersion: packageVersion,
	}
	switch {
	case !hasID:
		// A reference to a component resource has no ID, so ref.ID stays null.
	case id == "":
		// An empty ID marks a resource whose ID is not yet known.
		ref.ID = property.New(property.Computed)
	default:
		ref.ID = property.New(id)
	}
	return property.New(ref), nil
}
