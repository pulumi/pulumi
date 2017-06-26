// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

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
