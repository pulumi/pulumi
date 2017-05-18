// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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
