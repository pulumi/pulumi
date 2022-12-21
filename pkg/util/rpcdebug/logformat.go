// Copyright 2016-2022, Pulumi Corporation.
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

package rpcdebug

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// JSON format for tracking gRPC conversations. Normal methods have one entry for each req-resp conversation, streaming
// methods have one entry per each request or response over the stream.
type debugInterceptorLogEntry struct {
	Method   string          `json:"method"`
	Request  json.RawMessage `json:"request,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
	Errors   []string        `json:"errors,omitempty"`
	Metadata interface{}     `json:"metadata,omitempty"`
}

// Convert protobuf messages to JSON used in Request, Response fields of debugInterceptorLogEntry.
func transcode(obj interface{}) (json.RawMessage, error) {
	if obj == nil {
		return json.RawMessage("null"), nil
	}

	m, ok := obj.(proto.Message)
	if !ok {
		return json.RawMessage("null"),
			fmt.Errorf("Failed to decode, expecting proto.Message, got %v",
				reflect.TypeOf(obj))
	}

	jsonSer := jsonpb.Marshaler{}
	buf := bytes.Buffer{}
	if err := jsonSer.Marshal(&buf, m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func transcodeBack(raw json.RawMessage, dest interface{}) error {
	jsonSer := &jsonpb.Unmarshaler{}
	destmsg := dest.(proto.Message)
	return jsonSer.Unmarshal(bytes.NewReader(raw), destmsg)
}
