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

// LogEncryptionSessionInitRequest defines the request body for initializing an encryption session.
type LogEncryptionSessionInitRequest struct {
	// SessionKeyType is the type of session key to create. We currently only support "plog_v1"
	SessionKeyType string `json:"sessionKeyType"`
}

// EncryptionSessionInitResponse is the response from initializing an encryption session.
type LogEncryptionSessionInitResponse struct {
	// SessionID is the unique identifier for the encryption session.
	SessionID string `json:"sessionID"`
	// SessionKeyType is the type of session key that was created. Should always be the same as requested.
	SessionKeyType string `json:"sessionKeyType"`
	// SessionKey is the base64-encoded session key bytes.
	SessionKey string `json:"sessionKey"`
}
