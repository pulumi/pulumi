// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Model resource defines the structure of a request or response payload for an Amazon API Gateway method.
type Model struct {
	idl.NamedResource
	// The content type for the model.
	ContentType string `lumi:"contentType,replaces"`
	// The REST API with which to associate this model.
	RestAPI *RestAPI `lumi:"restAPI,replaces"`
	// The schema to use to transform data to one or more output formats. Specify null (`{}`) if you don't want to
	// specify a schema.
	Schema interface{} `lumi:"schema"`
	// A name for the model.  If you don't specify a name, a unique physical ID is generated and used.
	ModelName *string `lumi:"modelName,replaces,optional"`
	// A description that identifies this model.
	Description *string `lumi:"description,optional"`
}
