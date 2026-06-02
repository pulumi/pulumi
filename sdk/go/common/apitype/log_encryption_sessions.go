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

// SessionKeyType represents the type of session key used for log encryption.
type SessionKeyType string

const (
	// SessionKeyTypePlogV1 is the plog_v1 session key type (AES-256-GCM).
	SessionKeyTypePlogV1 SessionKeyType = "plog_v1"
)

// LogEncryptionSessionInitRequest defines the request body for initializing an encryption session.
type LogEncryptionSessionInitRequest struct {
	SessionKeyType SessionKeyType `json:"sessionKeyType"`
}

// LogEncryptionSessionInitResponse is the response from initializing an encryption session.
type LogEncryptionSessionInitResponse struct {
	SessionID      string         `json:"sessionID"`
	SessionKeyType SessionKeyType `json:"sessionKeyType"`
	SessionKey     string         `json:"sessionKey"`
}
