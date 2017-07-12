// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Deployment resource deploys an Amazon API Gateway (API Gateway) RestAPI resource to a stage so
// that clients can call the API over the Internet.  The stage acts as an environment.
type Deployment struct {
	idl.NamedResource
	// restAPI is the RestAPI resource to deploy.
	RestAPI *RestAPI `lumi:"restAPI,replaces"`
	// description is a description of the purpose of the API Gateway deployment.
	Description *string `lumi:"description,optional"`

	// The identifier for the deployment resource.
	ID string `lumi:"id,out"`
	// The date and time that the deployment resource was created.
	CreatedDate string `lumi:"createdDate,out"`
}
