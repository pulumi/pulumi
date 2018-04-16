// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package apitype

// StackSummary presents an overview of a particular stack without enumerating its current resource set.
type StackSummary struct {
	// ID is the unique identifier for a stack in the context of its PPC.
	ID string `json:"id"`

	// ActiveUpdate is the unique identifier for the stack's active update. This may be empty if no update has
	// been applied.
	ActiveUpdate string `json:"activeUpdate"`

	// ResourceCount is the number of resources associated with this stack. Note that this is currently unimplemented.
	ResourceCount int `json:"resourceCount"`
}

// ListStacksResponse describes the data returned by the `GET /stacks` endpoint of the PPC API.
type ListStacksResponse struct {
	// Stacks contains a list of summaries for each stack that currently exists in the PPC.
	Stacks []StackSummary `json:"stacks"`
}

// CreateStackResponseByID describes the data returned by the `POST /stacks` endpoint of the PPC API.
type CreateStackResponseByID struct {
	// ID is the unique identifier for the newly-created stack.
	ID string `json:"id"`
}

// CreateStackRequest defines the request body for creating a new Stack
type CreateStackRequest struct {
	// If empty means use the default cloud.
	CloudName string `json:"cloudName"`
	// The rest of the StackIdentifier (repo, project) is in the URL.
	StackName string `json:"stackName"`
	// An optional set of tags to apply to the stack.
	Tags map[StackTagName]string `json:"tags,omitEmpty"`
}

// CreateStackResponseByName is the response from a create Stack request.
type CreateStackResponseByName struct {
	// The name of the cloud used if the default was sent.
	CloudName string `json:"cloudName"`
}

// GetStackResponse describes the data returned by the `/GET /stack/{stackID}` endpoint of the PPC API. If the
// `deployment` query parameter is set to `true`, `Deployment` will be set and `Resources will be empty.
type GetStackResponse struct {
	// ID is the unique identifier for a stack in the context of its PPC.
	ID string `json:"id"`

	// ActiveUpdate is the unique identifier for the stack's active update. This may be empty if no update has
	// been applied.
	ActiveUpdate string `json:"activeUpdate"`

	// UnknownState indicates whether or not the contents of the resources array contained in the response is
	// known to accurately represent the cloud resources managed by this stack. A stack that is in an unknown
	// state cannot be updated.
	// TODO: [pulumi/pulumi-ppc#29]: make this state recoverable. This could be as simple as import/export.
	UnknownState bool `json:"unknownState"`

	// Resources provides the list of cloud resources managed by this stack.
	Resources []ResourceV1 `json:"resources"`

	// Manifest is the Manifest from the last rendered checkpoint.
	Manifest ManifestV1 `json:"manifest"`

	// Deployment provides a view of the stack as an opaque Pulumi deployment.
	Deployment *DeploymentV1 `json:"deployment,omitempty"`
}

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

// ExportStackResponse defines the response body for exporting a Stack.
type ExportStackResponse UntypedDeployment

// ImportStackRequest defines the request body for importing a Stack.
type ImportStackRequest UntypedDeployment

// ImportStackResponse defines the response body for importing a Stack.
type ImportStackResponse struct {
	UpdateID string `json:"updateId"`
}
