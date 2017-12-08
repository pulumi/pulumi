// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apitype

import "github.com/pulumi/pulumi/pkg/tokens"

// Resource describes a Cloud resource constructed by Pulumi.
type Resource struct {
	Type     string                 `json:"type"`
	URN      string                 `json:"urn"`
	Custom   bool                   `json:"custom"`
	ID       string                 `json:"id"`
	Inputs   map[string]interface{} `json:"inputs"`
	Defaults map[string]interface{} `json:"defaults"`
	Outputs  map[string]interface{} `json:"outputs"`
	Parent   string                 `json:"parent"`
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
	CloudName string `json:"cloudName"`
	StackName string `json:"stackName"`
}

// CreateStackResponse is the response from a create Stack request.
type CreateStackResponse struct {
	// The name of the cloud used if the default was sent.
	CloudName string `json:"cloudName"`
}
