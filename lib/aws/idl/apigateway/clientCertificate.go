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

// The ClientCertificate resource creates a client certificate that Amazon API Gateway (API Gateway)
// uses to configure client-side SSL authentication for sending requests to the integration endpoint.
type ClientCertificate struct {
	idl.NamedResource
	// Description is a description of the client certificate.
	Description *string `lumi:"description,optional"`
}
