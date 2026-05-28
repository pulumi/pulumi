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

package otelreceiver

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// PropertyValueMagic is the fixed64 value that identifies a LogPropertyValue.
// It is the ASCII string "pulumiPv" interpreted as a little-endian uint64.
const PropertyValueMagic uint64 = 0x7650696d756c7570 // "pulumiPv" LE

// EncodePropertyValue wraps a google.protobuf.Struct in the LogPropertyValue
// wire format (see proto/pulumi/log.proto).  The result is suitable for use
// as a BytesValue attribute in an OTLP log record.
func EncodePropertyValue(s *structpb.Struct) ([]byte, error) {
	return proto.Marshal(&pulumirpc.LogPropertyValue{
		Magic: PropertyValueMagic,
		Value: s,
	})
}

// DecodePropertyValue attempts to decode bytes as a LogPropertyValue.
// Returns the inner Struct if the magic matches, or an error if the bytes
// are not a valid LogPropertyValue.
func DecodePropertyValue(data []byte) (*structpb.Struct, error) {
	msg := &pulumirpc.LogPropertyValue{}
	if err := proto.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	if msg.Magic != PropertyValueMagic {
		return nil, errors.New("not a LogPropertyValue: magic mismatch")
	}
	if msg.Value == nil {
		return nil, errors.New("not a LogPropertyValue: missing value")
	}
	return msg.Value, nil
}
