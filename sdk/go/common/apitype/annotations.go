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

package apitype

import "encoding/json"

// BulkResourceAnnotation is a single annotation returned from the bulk list endpoint.
type BulkResourceAnnotation struct {
	Urn      string          `json:"urn"`
	Source   string          `json:"source"`
	Kind     string          `json:"kind"`
	Data     json.RawMessage `json:"data"`
	Created  string          `json:"created,omitempty"`
	Modified string          `json:"modified,omitempty"`
}

// ListBulkResourceAnnotationsResponse is the response from the bulk annotations list endpoint.
type ListBulkResourceAnnotationsResponse struct {
	Annotations       []BulkResourceAnnotation `json:"annotations"`
	ContinuationToken *string                  `json:"continuationToken,omitempty"`
}

// BatchAnnotationOperation is a single write operation in a batch annotations request.
type BatchAnnotationOperation struct {
	Urn  string          `json:"urn"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data,omitempty"`
}

// BatchWriteAnnotationsRequest is the request body for the batch annotations write endpoint.
type BatchWriteAnnotationsRequest struct {
	Operations []BatchAnnotationOperation `json:"operations"`
}
