// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package apitype

import (
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/tokens"
)

// Resource describes a Cloud resource constructed by Pulumi.
type Resource struct {
	Type     string                 `json:"type"`
	URN      string                 `json:"urn"`
	Custom   bool                   `json:"custom"`
	ID       string                 `json:"id"`
	Inputs   map[string]interface{} `json:"inputs"`
	Defaults map[string]interface{} `json:"defaults"` // TODO: extra in pulumi
	Outputs  map[string]interface{} `json:"outputs"`
	Parent   string                 `json:"parent"`
	Protect  bool                   `json:"protect"` // TODO: extra in pulumi
}

// Stack describes a Stack running on a Pulumi Cloud.
type Stack struct {
	CloudName string `json:"cloudName"`
	OrgName   string `json:"orgName"`

	RepoName    string       `json:"repoName"`
	ProjectName string       `json:"projName"`
	StackName   tokens.QName `json:"stackName"`

	ActiveUpdate string     `json:"activeUpdate"`
	Resources    []Resource `json:"resources,omitempty"`

	Version int `json:"version"`
}

// CreateStackRequest defines the request body for creating a new Stack
type CreateStackRequest struct {
	// If empty means use the default cloud.
	CloudName string `json:"cloudName"`
	// The rest of the StackIdentifier (repo, project) is in the URL.
	StackName string `json:"stackName"`
}

// CreateStackResponse is the response from a create Stack request.
type CreateStackResponse struct {
	// The name of the cloud used if the default was sent.
	CloudName string `json:"cloudName"`
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

// StackExport describes an exported stack.
type StackExport struct {
	// The opaque Pulumi deployment.
	Deployment json.RawMessage `json:"deployment,omitempty"`
}

// ExportStackResponse defines the response body for exporting a Stack.
type ExportStackResponse StackExport

// ImportStackRequest defines the request body for importing a Stack.
type ImportStackRequest StackExport

// ImportStackResponse defines the response body for importing a Stack.
type ImportStackResponse struct {
	UpdateID string `json:"updateId"`
}
