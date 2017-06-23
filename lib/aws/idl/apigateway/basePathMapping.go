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

// The BasePathMapping resource creates a base path that clients who call your Amazon API Gateway API
// must use in the invocation URL.
type BasePathMapping struct {
	idl.NamedResource
	// DomainName is the domain name for the base path mapping.
	DomainName string `lumi:"domainName"`
	// RestAPI is the API to map.
	RestAPI *RestAPI `lumi:"restAPI"`
	// BasePath is the base path that callers of the API must provider in the URL after the domain name.
	BasePath *string `lumi:"basePath,optional"`
	// Stage is the mapping's API stage.
	Stage *Stage `lumi:"stage,optional"`
}
