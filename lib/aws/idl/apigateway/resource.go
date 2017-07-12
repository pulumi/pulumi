// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// An Amazon API Gateway (API Gateway) API resource.
type Resource struct {
	idl.NamedResource
	// If you want to create a child resource, the parent resource.  For resources without a parent, specify
	// the RestAPI's root resource.
	Parent *Resource `lumi:"parent,replaces"`
	// A path name for the resource.
	PathPart string `lumi:"pathPart,replaces"`
	// The RestAPI resource in which you want to create this resource.
	RestAPI *RestAPI `lumi:"restAPI,replaces"`
}
