// Copyright 2016-2026, Pulumi Corporation.
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

package restgateway

import (
	"strings"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/structpb"
)

// JSONToStruct converts a map[string]interface{} (from JSON) to a protobuf Struct.
func JSONToStruct(m map[string]interface{}) (*structpb.Struct, error) {
	if m == nil {
		return nil, nil
	}
	return structpb.NewStruct(m)
}

// StructToJSON converts a protobuf Struct to a map[string]interface{} (for JSON).
func StructToJSON(s *structpb.Struct) map[string]interface{} {
	if s == nil {
		return nil
	}
	return s.AsMap()
}

// SeverityToProto converts a string severity level to the protobuf enum.
func SeverityToProto(s string) pulumirpc.LogSeverity {
	switch strings.ToLower(s) {
	case "debug":
		return pulumirpc.LogSeverity_DEBUG
	case "info":
		return pulumirpc.LogSeverity_INFO
	case "warning":
		return pulumirpc.LogSeverity_WARNING
	case "error":
		return pulumirpc.LogSeverity_ERROR
	default:
		return pulumirpc.LogSeverity_INFO
	}
}
