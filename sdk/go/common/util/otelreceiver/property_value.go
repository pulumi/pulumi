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
	"encoding/binary"
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/sig"
)

// PropertyValueMagic is the fixed64 value that identifies an
// encoded property value.
const PropertyValueMagic = sig.PropertyValueLogMagic

func EncodePropertyValue(s *structpb.Struct) ([]byte, error) {
	sv := &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: s}}
	valBytes, err := proto.Marshal(sv)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 8+len(valBytes))
	binary.LittleEndian.PutUint64(buf[:8], PropertyValueMagic)
	copy(buf[8:], valBytes)
	return buf, nil
}

func DecodePropertyValue(data []byte) (*structpb.Struct, error) {
	if len(data) < 8 {
		return nil, errors.New("not a property value: too short")
	}
	if binary.LittleEndian.Uint64(data[:8]) != PropertyValueMagic {
		return nil, errors.New("not a property value: magic mismatch")
	}
	val := &structpb.Value{}
	if err := proto.Unmarshal(data[8:], val); err != nil {
		return nil, err
	}
	s := val.GetStructValue()
	if s == nil {
		return nil, errors.New("not a property value: not a struct")
	}
	return s, nil
}
