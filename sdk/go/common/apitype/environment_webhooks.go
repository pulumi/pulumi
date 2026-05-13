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

// EnvironmentWebhook is the response shape for a Pulumi ESC environment
// webhook. It mirrors the OpenAPI `WebhookResponse` schema for the env
// endpoints — distinct from the stack-shaped `Webhook` defined in webhooks.go.
type EnvironmentWebhook struct {
	OrganizationName string   `json:"organizationName"`
	ProjectName      string   `json:"projectName,omitempty"`
	EnvName          string   `json:"envName,omitempty"`
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	PayloadURL       string   `json:"payloadUrl"`
	Active           bool     `json:"active"`
	Format           string   `json:"format,omitempty"`
	Filters          []string `json:"filters,omitempty"`
	Groups           []string `json:"groups,omitempty"`
	HasSecret        bool     `json:"hasSecret,omitempty"`
	SecretCiphertext string   `json:"secretCiphertext,omitempty"`
}

// CreateEnvironmentWebhookRequest is the body for the create endpoint.
// Required: Name, DisplayName, PayloadURL, Active.
type CreateEnvironmentWebhookRequest struct {
	OrganizationName string   `json:"organizationName,omitempty"`
	ProjectName      string   `json:"projectName,omitempty"`
	EnvName          string   `json:"envName,omitempty"`
	Name             string   `json:"name"`
	DisplayName      string   `json:"displayName"`
	PayloadURL       string   `json:"payloadUrl"`
	Active           bool     `json:"active"`
	Format           string   `json:"format,omitempty"`
	Filters          []string `json:"filters,omitempty"`
	Secret           string   `json:"secret,omitempty"`
}

// UpdateEnvironmentWebhookRequest is the PATCH body for edit. All fields are
// pointers so callers can express "leave unchanged" by passing nil. Lists
// replace the entire stored list; for partial edits do a GET first and merge.
type UpdateEnvironmentWebhookRequest struct {
	DisplayName *string   `json:"displayName,omitempty"`
	PayloadURL  *string   `json:"payloadUrl,omitempty"`
	Active      *bool     `json:"active,omitempty"`
	Format      *string   `json:"format,omitempty"`
	Filters     *[]string `json:"filters,omitempty"`
	Secret      *string   `json:"secret,omitempty"`
}

// EnvironmentWebhookDelivery describes one delivery attempt for an environment
// webhook. Mirrors the OpenAPI `WebhookDelivery` schema.
type EnvironmentWebhookDelivery struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Timestamp       int64  `json:"timestamp"`
	Duration        int64  `json:"duration"`
	Payload         string `json:"payload"`
	RequestURL      string `json:"requestUrl"`
	RequestHeaders  string `json:"requestHeaders"`
	ResponseCode    int64  `json:"responseCode"`
	ResponseHeaders string `json:"responseHeaders"`
	ResponseBody    string `json:"responseBody"`
}
