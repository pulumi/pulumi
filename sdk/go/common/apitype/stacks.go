// Copyright 2016-2018, Pulumi Corporation.
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

// StackSummary describes the state of a stack, without including its specific resources, etc.
type StackSummary struct {
	// ID is the logical ID of the stack.
	//
	// For maintainers of the Pulumi service:
	// ID corresponds to the Program ID, not the Stack ID inside the Pulumi service.
	ID string `json:"id"`
	// OrgName is the organization name the stack is found in.
	OrgName string `json:"orgName"`
	// ProjectName is the name of the project the stack is associated with.
	ProjectName string `json:"projectName"`
	// StackName is the name of the stack.
	StackName string `json:"stackName"`

	// LastUpdate is a Unix timestamp of the start time of the stack's last update, as applicable.
	LastUpdate *int64 `json:"lastUpdate,omitempty"`

	// ResourceCount is the number of resources associated with this stack, as applicable.
	ResourceCount *int `json:"resourceCount,omitempty"`

	// Links to the stack in the Pulumi Console
	Links StackLinks `json:"links,omitempty"`
}

type StackLinks struct {
	Self string `json:"self"`
}

// ListStacksResponse returns a set of stack summaries. This call is designed to be inexpensive.
type ListStacksResponse struct {
	Stacks []StackSummary `json:"stacks"`

	// ContinuationToken is an opaque value used to mark the end of the all stacks. If non-nil,
	// pass it into a subsequent call in order to get the next batch of results.
	//
	// A value of nil means that all stacks have been returned.
	ContinuationToken *string `json:"continuationToken,omitempty"`
}

// CreateStackRequest defines the request body for creating a new Stack
type CreateStackRequest struct {
	// The rest of the StackIdentifier (e.g. organization, project) is in the URL.
	StackName string `json:"stackName"`

	// An optional set of tags to apply to the stack.
	Tags map[StackTagName]string `json:"tags,omitempty"`

	// An optional set of teams to assign to the stack.
	Teams []string `json:"teams,omitempty"`

	// An optional state to initialize the stack with.
	State *UntypedDeployment `json:"state,omitempty"`

	Config *StackConfig `json:"config,omitempty"`
}

// CreateStackResponse is the response from a create Stack request.
type CreateStackResponse struct{}

// EncryptValueRequest defines the request body for encrypting a value.
type EncryptValueRequest struct {
	// The value to encrypt.
	Plaintext []byte `json:"plaintext"`
}

// EncryptValueResponse defines the response body for an encrypted value.
type EncryptValueResponse struct {
	// The encrypted value.
	Ciphertext []byte `json:"ciphertext"`
}

// BatchEncryptRequest defines the request body for encrypting multiple values.
type BatchEncryptRequest struct {
	// The values to encrypt.
	Plaintexts [][]byte `json:"plaintexts"`
}

// BatchEncryptResponse defines the response body for multiple encrypted values.
type BatchEncryptResponse struct {
	// The encrypted values, in order of the plaintexts from the request.
	Ciphertexts [][]byte `json:"ciphertexts"`
}

// DecryptValueRequest defines the request body for decrypting a value.
type DecryptValueRequest struct {
	// The value to decrypt.
	Ciphertext []byte `json:"ciphertext"`
}

// DecryptValueResponse defines the response body for a decrypted value.
type DecryptValueResponse struct {
	// The decrypted value.
	Plaintext []byte `json:"plaintext"`
}

// Log3rdPartyDecryptionEvent defines the request body for logging a 3rd party secrets provider decryption event.
type Log3rdPartyDecryptionEvent struct {
	SecretName  string `json:"secretName,omitempty"`
	CommandName string `json:"commandName,omitempty"`
}

// BatchDecryptRequest defines the request body for batch decrypting secret values.
type BatchDecryptRequest struct {
	Ciphertexts [][]byte `json:"ciphertexts"`
}

// BatchDecryptResponse defines the response body for batch decrypted secret values. The key in
// the map is the base64 encoding of the ciphertext.
type BatchDecryptResponse struct {
	Plaintexts map[string][]byte `json:"plaintexts"`
}

// ExportStackResponse defines the response body for exporting a Stack.
type ExportStackResponse UntypedDeployment

// ImportStackRequest defines the request body for importing a Stack.
type ImportStackRequest UntypedDeployment

// ImportStackResponse defines the response body for importing a Stack.
type ImportStackResponse struct {
	UpdateID string `json:"updateId"`
}
