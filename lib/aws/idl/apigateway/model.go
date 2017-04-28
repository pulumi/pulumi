// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// The Model resource defines the structure of a request or response payload for an Amazon API Gateway method.
type Model struct {
	idl.NamedResource
	// The content type for the model.
	ContentType string `coco:"contentType,replaces"`
	// The REST API with which to associate this model.
	RestAPI *RestAPI `coco:"restAPI,replaces"`
	// The schema to use to transform data to one or more output formats. Specify null (`{}`) if you don't want to
	// specify a schema.
	Schema interface{} `coco:"schema"`
	// A name for the model.  If you don't specify a name, a unique physical ID is generated and used.
	ModelName *string `coco:"modelName,replaces,optional"`
	// A description that identifies this model.
	Description *string `coco:"description,optional"`
}
