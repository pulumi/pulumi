// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/coconut/pkg/resource/idl"
)

// The UsagePlan resource specifies a usage plan for deployed Amazon API Gateway (API Gateway) APIs.  A
// usage plan enforces throttling and quota limits on individual client API keys. For more information, see
// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-api-usage-plans.html.
type UsagePlan struct {
	idl.NamedResource
	APIStages     *[]APIStage       `coco:"apiStages,optional"`
	Description   *string           `coco:"description,optional"`
	Quota         *QuotaSettings    `coco:"quota,optional"`
	Throttle      *ThrottleSettings `coco:"throttle,optional"`
	UsagePlanName *string           `coco:"usagePlanName,optional"`
}

// APIStage specifies which Amazon API Gateway (API Gateway) stage and API to associate with a usage plan.
type APIStage struct {
	// The API you want to associate with the usage plan.
	API *RestAPI `coco:"api,optional"`
	// The Stage you want to associate with the usage plan.
	Stage *Stage `coco:"stage,optional"`
}

// QuotaSettings specifies the maximum number of requests users can make to your Amazon API Gateway (API Gateway) APIs.
type QuotaSettings struct {
	// The maximum number of requests that users can make within the specified time period.
	Limit *float64 `coco:"limit,optional"`
	// For the initial time period, the number of requests to subtract from the specified limit.  When you first
	// implement a usage plan, the plan might start in the middle of the week or month.  With this property, you can
	// decrease the limit for this initial time period.
	Offset *float64 `coco:"offset,optional"`
	// The time period for which the maximum limit of requests applies.
	Period *QuotaPeriod `coco:"period,optional"`
}

// The time period in which a quota limit applies.
type QuotaPeriod string

const (
	QuotaDayPeriod   QuotaPeriod = "DAY"
	QuotaWeekPeriod  QuotaPeriod = "WEEK"
	QuotaMonthPeriod QuotaPeriod = "MONTH"
)

// ThrottleSettings specifies the overall request rate (average requests per second) and burst capacity when users call
// your Amazon API Gateway (API Gateway) APIs.
type ThrottleSettings struct {
	// The maximum API request rate limit over a time ranging from one to a few seconds.  The maximum API request rate
	// limit depends on whether the underlying token bucket is at its full capacity.  For more information about request
	// throttling, see http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
	BurstRateLimit *float64 `coco:"burstRateLimit,optional"`
	// The API request steady-state rate limit (average requests per second over an extended period of time). For more
	// information about request throttling, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
	RateLimit *float64 `coco:"rateLimit,optional"`
}
