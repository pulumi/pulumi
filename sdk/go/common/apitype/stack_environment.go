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

// StackEnvironmentSyncRequest publishes the inline environment definition from a stack's
// configuration file to the ESC environment managed by that stack.
type StackEnvironmentSyncRequest struct {
	// Yaml is the environment definition, as written in the `environment` block of the stack's
	// configuration file.
	Yaml string `json:"yaml"`
	// ExpectedRevision, when set, is the revision the caller last observed. The request fails with a
	// conflict if the environment has moved past it.
	ExpectedRevision *int `json:"expectedRevision,omitempty"`
	// DryRun reports how the definition differs from the published one without publishing or opening.
	DryRun bool `json:"dryRun,omitempty"`
	// OpenDuration is how long the opened environment stays valid, as a duration string such as "2h".
	OpenDuration string `json:"openDuration,omitempty"`
}

// StackEnvironmentSyncResponse is the outcome of a StackEnvironmentSyncRequest.
type StackEnvironmentSyncResponse struct {
	// Environment is the environment managed by the stack, as "project/name".
	Environment string `json:"environment"`
	// PreviousRevision is the environment's latest revision before the request (0 if it did not exist).
	PreviousRevision int `json:"previousRevision"`
	// Revision holds the stack's definition after the request.
	Revision int `json:"revision"`
	// Created reports that the environment was created, or would be created on a dry run.
	Created bool `json:"created"`
	// Changed reports that the submitted definition differs from the published one.
	Changed bool `json:"changed"`
	// CurrentYaml is the definition published before the request, with secrets encrypted.
	CurrentYaml string `json:"currentYaml,omitempty"`
	// OpenSessionID is the open session for Revision. Empty on a dry run or on errors.
	OpenSessionID string `json:"openSessionId,omitempty"`
	// Diagnostics carries the errors that stopped the definition from being published or opened.
	Diagnostics EnvironmentDiagnostics `json:"diagnostics,omitempty"`
}

// StackEnvironmentSecretRequest asks the service to encrypt a value with the key of the environment
// managed by a stack.
type StackEnvironmentSecretRequest struct {
	// Plaintext is the value to encrypt.
	Plaintext string `json:"plaintext"`
}

// StackEnvironmentSecretResponse carries a value encrypted for the environment managed by a stack.
type StackEnvironmentSecretResponse struct {
	// Environment is the environment managed by the stack, as "project/name".
	Environment string `json:"environment"`
	// Ciphertext is the base64 ciphertext to store as `fn::secret: {ciphertext: ...}`.
	Ciphertext string `json:"ciphertext"`
}
