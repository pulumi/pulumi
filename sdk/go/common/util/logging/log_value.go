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

package logging

import (
	"encoding/binary"
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// PropertyValueLogMagic is the magic prefix written before the
// serialized property value in the encoder functions below.  It is
// the ASCII string "pulumiPv" interpreted as a little-endian uint64.
const PropertyValueLogMagic uint64 = 0x7650696d756c7570

const propertyValueLogMagicSize = 8

// EncodeStructValueForLog prepends the magic prefix to the
// protobuf-encoded structpb.Value.  Callers in the plugin package
// wrap this to accept resource.PropertyValue and property.Value.
func EncodeStructValueForLog(val *structpb.Value) ([]byte, error) {
	valBytes, err := proto.Marshal(val)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, propertyValueLogMagicSize+len(valBytes))
	binary.LittleEndian.PutUint64(buf[:propertyValueLogMagicSize], PropertyValueLogMagic)
	copy(buf[propertyValueLogMagicSize:], valBytes)
	return buf, nil
}

// decodeStructValueFromLog verifies the magic prefix and returns the
// inner protobuf-encoded structpb.Value.
func decodeStructValueFromLog(data []byte) (*structpb.Value, error) {
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
