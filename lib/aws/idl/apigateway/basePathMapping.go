// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// The BasePathMapping resource creates a base path that clients who call your Amazon API Gateway API
// must use in the invocation URL.
type BasePathMapping struct {
	idl.NamedResource
	// DomainName is the domain name for the base path mapping.
	DomainName string `coco:"domainName"`
	// RestAPI is the API to map.
	RestAPI *RestAPI `coco:"restAPI"`
	// BasePath is the base path that callers of the API must provider in the URL after the domain name.
	BasePath *string `coco:"basePath,optional"`
	// Stage is the mapping's API stage.
	Stage *Stage `coco:"stage,optional"`
}
