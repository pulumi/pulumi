// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"

	"github.com/pulumi/lumi/lib/aws/idl/s3"
)

// The RestAPI resource contains a collection of Amazon API Gateway (API Gateway) resources and methods that can be
// invoked through HTTPS endpoints.
type RestAPI struct {
	idl.NamedResource
	// An OpenAPI specification that defines a set of RESTful APIs in the JSON format.
	Body *interface{} `lumi:"body,optional"`
	// The Amazon Simple Storage Service (Amazon S3) location that points to a OpenAPI file, which defines a set of
	// RESTful APIs in JSON or YAML format.
	BodyS3Location *S3Location `lumi:"bodyS3Location,optional"`
	// Another API Gateway RestAPI resource that you want to clone.
	CloneFrom *RestAPI `lumi:"cloneFrom,optional"`
	// A description of the purpose of this API Gateway RestAPI resource.
	Description *string `lumi:"description,optional"`
	// If a warning occurs while API Gateway is creating the RestAPI resource, indicates whether to roll back the
	// resource.
	FailOnWarnings *bool `lumi:"failOnWarnings,optional"`
	// A name for the API Gateway RestApi resource.  Required if you don't specify an OpenAPI definition.
	APIName *string `lumi:"apiName,optional"`
	// Custom header parameters for the request.
	Parameters *[]string `lumi:"parameters,optional"`
}

// S3Location is a property of the RestAPI resource that specifies the Amazon Simple Storage Service (Amazon S3)
// location of a OpenAPI (formerly Swagger) file that defines a set of RESTful APIs in JSON or YAML.
type S3Location struct {
	// The S3 object corresponding to the OpenAPI file.
	Object *s3.Object `lumi:"object"`
	// The Amazon S3 ETag (a file checksum) of the OpenAPI file.  If you don't specify a value, API Gateway skips ETag
	// validation of your OpenAPI file.
	ETag *string `lumi:"etag,optional"`
	// For versioning-enabled buckets, a specific version of the OpenAPI file.
	Version *string `lumi:"version,optional"`
}
