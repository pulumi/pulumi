// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// The UsagePlanKey resource associates an Amazon API Gateway API key with an API Gateway usage plan.  This association
// determines which user the usage plan is applied to.
type UsagePlanKey struct {
	idl.NamedResource
	// The API key for the API resource to associate with a usage plan.
	Key *APIKey `coco:"key,replaces"`
	// The usage plan.
	UsagePlan *UsagePlan `coco:"usagePlan,replaces"`
}
