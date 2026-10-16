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

// Webhook describes a webhook returned by the Pulumi Cloud REST API.
type Webhook struct {
	OrganizationName string   `json:"organizationName"`
	ProjectName      *string  `json:"projectName,omitempty"`
	StackName        *string  `json:"stackName,omitempty"`
	EnvName          *string  `json:"envName,omitempty"`
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	PayloadURL       string   `json:"payloadUrl"`
	Active           bool     `json:"active"`
	Format           *string  `json:"format,omitempty"`
	Filters          []string `json:"filters,omitempty"`
	Groups           []string `json:"groups,omitempty"`
	Secret           string   `json:"secret,omitempty"`
	HasSecret        bool     `json:"hasSecret"`
	SecretCiphertext string   `json:"secretCiphertext"`
}

// WebhookDelivery describes the result of delivering a webhook event,
// returned by the ping and delivery endpoints.
type WebhookDelivery struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Payload         string `json:"payload"`
	Timestamp       int64  `json:"timestamp"`
	Duration        int    `json:"duration"`
	RequestURL      string `json:"requestUrl"`
	RequestHeaders  string `json:"requestHeaders"`
	ResponseCode    int    `json:"responseCode"`
	ResponseHeaders string `json:"responseHeaders"`
	ResponseBody    string `json:"responseBody"`
}
