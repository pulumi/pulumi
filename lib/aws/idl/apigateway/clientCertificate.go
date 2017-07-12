// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The ClientCertificate resource creates a client certificate that Amazon API Gateway (API Gateway)
// uses to configure client-side SSL authentication for sending requests to the integration endpoint.
type ClientCertificate struct {
	idl.NamedResource
	// Description is a description of the client certificate.
	Description *string `lumi:"description,optional"`
}
