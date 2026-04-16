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
	"encoding/binary"
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// PropertyValueLogMagic is the magic prefix written before the
// serialized property value in the encoder functions below.  It is
// the ASCII string "pulumiPv" interpreted as a little-endian uint64.
const PropertyValueLogMagic uint64 = 0x7650696d756c7570

const propertyValueLogMagicSize = 8

// encodeStructValue prepends the magic prefix to the protobuf-encoded
// structpb.Value.
func encodeStructValue(val *structpb.Value) ([]byte, error) {
	valBytes, err := proto.Marshal(val)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, propertyValueLogMagicSize+len(valBytes))
	binary.LittleEndian.PutUint64(buf[:propertyValueLogMagicSize], PropertyValueLogMagic)
	copy(buf[propertyValueLogMagicSize:], valBytes)
	return buf, nil
}

// decodeStructValue verifies the magic prefix and returns the inner
// protobuf-encoded structpb.Value.
func decodeStructValue(data []byte) (*structpb.Value, error) {
	if len(data) < propertyValueLogMagicSize {
		return nil, errors.New("not a property value log: too short")
	}
	if binary.LittleEndian.Uint64(data[:propertyValueLogMagicSize]) != PropertyValueLogMagic {
		return nil, errors.New("not a property value log: magic mismatch")
	}
	val := &structpb.Value{}
	if err := proto.Unmarshal(data[propertyValueLogMagicSize:], val); err != nil {
		return nil, err
	}
	return val, nil
}

// EncodePropertyValueForLog serializes a resource.PropertyValue for
// inclusion as a BytesValue attribute in an OTLP log record. The
// output is the magic prefix followed by the protobuf-encoded
// structpb.Value produced by MarshalPropertyValue with KeepSecrets
// and KeepUnknowns enabled. The receiver uses DecodePropertyValueFromLog
// to identify and recover the original property value.
func EncodePropertyValueForLog(v resource.PropertyValue) ([]byte, error) {
	val, err := MarshalPropertyValue("", v, MarshalOptions{
		KeepSecrets:  true,
		KeepUnknowns: true,
	})
	if err != nil {
		return nil, err
	}
	return encodeStructValue(val)
}

// DecodePropertyValueFromLog is the inverse of EncodePropertyValueForLog.
// Returns an error if the bytes do not start with the magic prefix.
func DecodePropertyValueFromLog(data []byte) (resource.PropertyValue, error) {
	val, err := decodeStructValue(data)
	if err != nil {
		return resource.PropertyValue{}, err
	}
	v, err := UnmarshalPropertyValue("", val, MarshalOptions{
		KeepSecrets:  true,
		KeepUnknowns: true,
	})
	if err != nil {
		return resource.PropertyValue{}, err
	}
	if v == nil {
		return resource.PropertyValue{}, nil
	}
	return *v, nil
}

// EncodeValueForLog is the property.Value variant of
// EncodePropertyValueForLog. The wire format is identical, so a value
// encoded by either function can be decoded by either DecodeValueFromLog
// or DecodePropertyValueFromLog.
func EncodeValueForLog(v property.Value) ([]byte, error) {
	return EncodePropertyValueForLog(resource.ToResourcePropertyValue(v))
}

// DecodeValueFromLog is the property.Value variant of
// DecodePropertyValueFromLog.
func DecodeValueFromLog(data []byte) (property.Value, error) {
	v, err := DecodePropertyValueFromLog(data)
	if err != nil {
		return property.Value{}, err
	}
	return resource.FromResourcePropertyValue(v), nil
}
