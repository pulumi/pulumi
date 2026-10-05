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

// Package credentialhelper invokes external credential helpers and validates their responses.
package credentialhelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/http/httpguts"
)

// Reason identifies why the CLI is requesting credentials.
type Reason string

const (
	Initial  Reason = "initial"
	Expired  Reason = "expired"
	Rejected Reason = "rejected"
)

// Request describes the backend selection and the reason for an invocation.
type Request struct {
	Version            int    `json:"version"`
	Reason             Reason `json:"reason"`
	SelectedBackendURL string `json:"selectedBackendUrl,omitempty"`
}

// Response contains the backend selection and credentials returned by a helper.
type Response struct {
	BackendURL  string
	AccessToken string
	Headers     http.Header
	Env         map[string]string
	ExpiresAt   *time.Time
}

// HasHTTPCredentials reports whether the response supplies a token or header values.
func (r *Response) HasHTTPCredentials() bool {
	return r != nil && (r.AccessToken != "" || len(r.Headers) != 0)
}

type wireResponse struct {
	Version     *int                `json:"version"`
	BackendURL  *string             `json:"backendUrl"`
	AccessToken string              `json:"accessToken"`
	Headers     map[string][]string `json:"headers"`
	Env         map[string]string   `json:"env"`
	ExpiresAt   *string             `json:"expiresAt"`
}

// decodeResponse validates a helper's output. A nil response means the helper declined.
func decodeResponse(data []byte, request Request) (*Response, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire *wireResponse
	if err := decoder.Decode(&wire); err != nil {
		if typeErr, ok := errors.AsType[*json.UnmarshalTypeError](err); ok && typeErr.Field != "" {
			return nil, fmt.Errorf("%s has the wrong type", typeErr.Field)
		}
		return nil, errors.New("response must be a JSON object containing only known fields")
	}
	if _, err := decoder.Token(); err != io.EOF || wire == nil {
		return nil, errors.New("response must contain exactly one JSON object")
	}
	// encoding/json decodes null as a zero value, which would turn a wrong type into an omitted field.
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil || containsNull(raw) {
		return nil, errors.New("response must not contain null values")
	}
	refresh := request.Reason != Initial
	empty := wire.BackendURL == nil && wire.AccessToken == "" && wire.Headers == nil && wire.Env == nil &&
		wire.ExpiresAt == nil
	// Only an empty response, which declines, may omit the version.
	if (wire.Version == nil && !empty) || (wire.Version != nil && *wire.Version != 1) {
		return nil, errors.New("response must specify version 1")
	}
	if empty && !refresh {
		return nil, nil
	}

	response := &Response{AccessToken: wire.AccessToken, Env: wire.Env}
	if wire.BackendURL != nil {
		if refresh || request.SelectedBackendURL != "" {
			return nil, errors.New("backendUrl is only allowed on an initial response without a selected backend")
		}
		if *wire.BackendURL == "" {
			return nil, errors.New("backendUrl must not be empty")
		}
		response.BackendURL = *wire.BackendURL
	}
	if !httpguts.ValidHeaderFieldValue("token " + wire.AccessToken) {
		return nil, errors.New("accessToken must be valid for an HTTP authorization header")
	}
	if wire.Headers != nil {
		response.Headers = http.Header{}
	}
	for name, values := range wire.Headers {
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, errors.New("headers contains an invalid HTTP header name")
		}
		switch strings.ToLower(name) {
		case "authorization", "connection", "content-length", "host", "keep-alive", "proxy-connection",
			"te", "trailer", "transfer-encoding", "upgrade":
			return nil, fmt.Errorf("headers contains a forbidden HTTP header %q", name)
		}
		for _, value := range values {
			if !httpguts.ValidHeaderFieldValue(value) {
				return nil, errors.New("headers contains an invalid HTTP header value")
			}
			response.Headers.Add(name, value)
		}
	}
	if wire.Env != nil && refresh {
		return nil, errors.New("env is not allowed on a refresh response")
	}
	for name, value := range wire.Env {
		if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, '\x00') {
			return nil, errors.New("env contains an invalid environment variable")
		}
		switch strings.ToUpper(name) {
		case "PULUMI_ACCESS_TOKEN", "PULUMI_BACKEND_URL", "PULUMI_API", "PULUMI_CREDENTIAL_HELPER":
			return nil, errors.New("env contains a forbidden environment variable")
		}
	}
	if wire.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *wire.ExpiresAt)
		if err != nil {
			return nil, errors.New("expiresAt must be an RFC 3339 timestamp")
		}
		response.ExpiresAt = &expiresAt
	}
	if refresh && !response.HasHTTPCredentials() {
		return nil, errors.New("helper must supply HTTP credentials on refresh")
	}
	return response, nil
}

func containsNull(value any) bool {
	switch value := value.(type) {
	case nil:
		return true
	case map[string]any:
		for _, element := range value {
			if containsNull(element) {
				return true
			}
		}
	case []any:
		return slices.ContainsFunc(value, containsNull)
	}
	return false
}
