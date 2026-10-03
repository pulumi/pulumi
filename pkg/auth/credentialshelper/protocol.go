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

// Package credentialshelper runs the credentials helper protocol independently of backend selection and storage.
package credentialshelper

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
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
	if r == nil {
		return false
	}
	if r.AccessToken != "" {
		return true
	}
	for _, values := range r.Headers {
		if len(values) != 0 {
			return true
		}
	}
	return false
}

func (r Request) validate(previous *Response) error {
	switch r.Reason {
	case Initial:
		return nil
	case Expired, Rejected:
		if r.SelectedBackendURL == "" {
			return errors.New("credential refresh requires a selected backend URL")
		}
		if !previous.HasHTTPCredentials() {
			return errors.New("credential refresh requires previous helper HTTP credentials")
		}
		return nil
	default:
		return errors.New("invalid credentials helper invocation reason")
	}
}

func decodeResponse(data []byte, request Request, previous *Response) (*Response, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, errors.New("response must contain a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("response must contain exactly one JSON object")
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("response must be a JSON object")
	}
	if len(fields) == 0 {
		if request.Reason != Initial {
			return nil, errors.New("helper must supply HTTP credentials on refresh")
		}
		return nil, nil
	}
	version, ok := fields["version"].(json.Number)
	if !ok || version != "1" {
		return nil, errors.New("response must specify version 1")
	}
	if len(fields) == 1 && request.Reason == Initial {
		return nil, nil
	}

	var response Response
	for name, value := range fields {
		switch name {
		case "version":
		case "backendUrl":
			if request.Reason != Initial || request.SelectedBackendURL != "" {
				return nil, errors.New("backendUrl is only allowed on an initial response without a selected backend")
			}
			backendURL, ok := value.(string)
			if !ok || !validBackendURL(backendURL) {
				return nil, errors.New("backendUrl must be a supported backend URL")
			}
			response.BackendURL = backendURL
		case "accessToken":
			token, ok := value.(string)
			if !ok || !httpguts.ValidHeaderFieldValue("token "+token) {
				return nil, errors.New("accessToken must be a string valid for an HTTP authorization header")
			}
			response.AccessToken = token
		case "headers":
			headers, err := decodeHeaders(value)
			if err != nil {
				return nil, err
			}
			response.Headers = headers
		case "env":
			if request.Reason != Initial {
				return nil, errors.New("env is not allowed on a refresh response")
			}
			environment, err := decodeEnvironment(value)
			if err != nil {
				return nil, err
			}
			response.Env = environment
		case "expiresAt":
			expiresAt, ok := value.(string)
			if !ok {
				return nil, errors.New("expiresAt must be an RFC 3339 timestamp")
			}
			expiry, err := time.Parse(time.RFC3339, expiresAt)
			if err != nil {
				return nil, errors.New("expiresAt must be an RFC 3339 timestamp")
			}
			response.ExpiresAt = &expiry
		default:
			return nil, errors.New("response contains an unknown field")
		}
	}
	if request.Reason != Initial {
		if !response.HasHTTPCredentials() {
			return nil, errors.New("helper must supply HTTP credentials on refresh")
		}
		if previous != nil && previous.AccessToken != "" && response.AccessToken == "" {
			return nil, errors.New("helper must continue supplying an accessToken on refresh")
		}
	}
	return &response, nil
}

func validBackendURL(raw string) bool {
	backend, err := url.Parse(raw)
	if err != nil || backend.Opaque != "" || !strings.HasPrefix(raw, backend.Scheme+"://") {
		return false
	}
	switch backend.Scheme {
	case "file":
		return backend.Host != "" || backend.Path != ""
	case "postgres":
		return true
	case "http", "https", "s3", "azblob", "gs":
		return backend.Hostname() != ""
	default:
		return false
	}
}

func decodeHeaders(value any) (http.Header, error) {
	fields, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("headers must be an object of string arrays")
	}
	headers := make(http.Header, len(fields))
	for name, value := range fields {
		if !httpguts.ValidHeaderFieldName(name) {
			return nil, errors.New("headers contains an invalid HTTP header name")
		}
		switch strings.ToLower(name) {
		case "authorization", "connection", "content-length", "host", "keep-alive", "proxy-connection",
			"te", "trailer", "transfer-encoding", "upgrade":
			return nil, errors.New("headers contains a forbidden HTTP header")
		}
		values, ok := value.([]any)
		if !ok {
			return nil, errors.New("headers must be an object of string arrays")
		}
		for _, value := range values {
			text, ok := value.(string)
			if !ok || !httpguts.ValidHeaderFieldValue(text) {
				return nil, errors.New("headers contains an invalid HTTP header value")
			}
			headers.Add(name, text)
		}
	}
	return headers, nil
}

func decodeEnvironment(value any) (map[string]string, error) {
	fields, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("env must be an object of strings")
	}
	environment := make(map[string]string, len(fields))
	for name, value := range fields {
		if name == "" || strings.ContainsAny(name, "=\x00") {
			return nil, errors.New("env contains an invalid environment variable name")
		}
		switch strings.ToUpper(name) {
		case "PULUMI_ACCESS_TOKEN", "PULUMI_BACKEND_URL", "PULUMI_API", "PULUMI_CREDENTIAL_HELPER":
			return nil, errors.New("env contains a forbidden environment variable")
		}
		text, ok := value.(string)
		if !ok || strings.ContainsRune(text, '\x00') {
			return nil, errors.New("env must contain valid environment variable strings")
		}
		environment[name] = text
	}
	return environment, nil
}
