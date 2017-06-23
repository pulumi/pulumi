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
