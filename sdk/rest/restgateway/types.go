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

// CreateSessionRequest is the body for POST /sessions.
type CreateSessionRequest struct {
	ProjectName string `json:"projectName"`
	Stack       string `json:"stack"`
	Preview     bool   `json:"preview,omitempty"`
}

// CreateSessionResponse is returned by POST /sessions.
type CreateSessionResponse struct {
	ID       string `json:"id"`
	StackURN string `json:"stackUrn"`
}

// RegisterResourceRequest is the body for POST /sessions/:id/resources.
type RegisterResourceRequest struct {
	Type                    string                 `json:"type"`
	Name                    string                 `json:"name"`
	Custom                  bool                   `json:"custom"`
	Parent                  string                 `json:"parent,omitempty"`
	Properties              map[string]interface{} `json:"properties,omitempty"`
	Dependencies            []string               `json:"dependencies,omitempty"`
	Provider                string                 `json:"provider,omitempty"`
	Version                 string                 `json:"version,omitempty"`
	Protect                 *bool                  `json:"protect,omitempty"`
	ImportID                string                 `json:"importId,omitempty"`
	DeleteBeforeReplace     bool                   `json:"deleteBeforeReplace,omitempty"`
	IgnoreChanges           []string               `json:"ignoreChanges,omitempty"`
	AdditionalSecretOutputs []string               `json:"additionalSecretOutputs,omitempty"`
	ReplaceOnChanges        []string               `json:"replaceOnChanges,omitempty"`
	RetainOnDelete          *bool                  `json:"retainOnDelete,omitempty"`
	PluginDownloadURL       string                 `json:"pluginDownloadURL,omitempty"`
}

// RegisterResourceResponse is returned by POST /sessions/:id/resources.
type RegisterResourceResponse struct {
	URN        string                 `json:"urn"`
	ID         string                 `json:"id,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Stable     bool                   `json:"stable"`
}

// InvokeRequest is the body for POST /sessions/:id/invoke.
type InvokeRequest struct {
	Token             string                 `json:"token"`
	Args              map[string]interface{} `json:"args,omitempty"`
	Provider          string                 `json:"provider,omitempty"`
	Version           string                 `json:"version,omitempty"`
	PluginDownloadURL string                 `json:"pluginDownloadURL,omitempty"`
}

// InvokeResponse is returned by POST /sessions/:id/invoke.
type InvokeResponse struct {
	Return   map[string]interface{} `json:"return,omitempty"`
	Failures []CheckFailure         `json:"failures,omitempty"`
}

// CheckFailure represents a single property check failure.
type CheckFailure struct {
	Property string `json:"property"`
	Reason   string `json:"reason"`
}

// LogRequest is the body for POST /sessions/:id/logs.
type LogRequest struct {
	Severity string `json:"severity"` // debug, info, warning, error
	Message  string `json:"message"`
	URN      string `json:"urn,omitempty"`
}

// DeleteSessionRequest is the optional body for DELETE /sessions/:id.
type DeleteSessionRequest struct {
	Exports map[string]interface{} `json:"exports,omitempty"`
}

// ErrorResponse is returned when an error occurs.
type ErrorResponse struct {
	Error string `json:"error"`
}
