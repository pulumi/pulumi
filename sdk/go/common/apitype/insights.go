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

import (
	"encoding/json"
	"time"
)

// InsightsResourceWithVersion is a single discovered resource as returned by the
// Pulumi Insights ReadResource endpoint. The shape mirrors the OpenAPI schema of
// the same name in the Pulumi Cloud REST API.
type InsightsResourceWithVersion struct {
	// Account is the name of the Insights account the resource was discovered in.
	Account string `json:"account"`
	// Type is the Pulumi resource type token (e.g. `aws:s3/bucket:Bucket`).
	Type string `json:"type"`
	// ID is the cloud-provider-assigned identifier for the resource.
	ID string `json:"id"`
	// Version is the monotonically-increasing version number for this resource.
	Version int64 `json:"version"`
	// Modified is the time at which the resource was last modified on the cloud
	// provider side, as recorded by Insights.
	Modified time.Time `json:"modified"`
	// State is the raw resource state as captured by the scan. The shape is
	// resource-type-specific and is passed through as JSON.
	State json.RawMessage `json:"state,omitempty"`
	// PolicyState is the evaluation state for any policies that ran against the
	// resource. Empty when no policies were evaluated.
	PolicyState string `json:"policyState,omitempty"`
}
