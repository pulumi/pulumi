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

// The UsagePlanKey resource associates an Amazon API Gateway API key with an API Gateway usage plan.  This association
// determines which user the usage plan is applied to.
type UsagePlanKey struct {
	idl.NamedResource
	// The API key for the API resource to associate with a usage plan.
	Key *APIKey `lumi:"key,replaces"`
	// The usage plan.
	UsagePlan *UsagePlan `lumi:"usagePlan,replaces"`
}
