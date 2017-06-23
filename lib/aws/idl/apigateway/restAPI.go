// Copyright 2016-2017, Pulumi Corporation
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

	// The API's identifier. This identifier is unique across all of your APIs in Amazon API Gateway.
	ID string `lumi:"id,out"`
	// The timestamp when the API was created.
	CreatedDate string `lumi:"createdDate,out"`
	// A version identifier for the API.
	Version string `lumi:"version,out"`
	// The warning messages reported when failonwarnings is turned on during API import.
	Warnings []string `lumi:"warnings,out"`
	// The list of binary media types supported by the RestApi. By default, the RestApi supports only UTF-8-encoded
	// text payloads.
	BinaryMediaTypes []string `lumi:"binaryMediaTypes,out"`
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
