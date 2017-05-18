// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Deployment resource deploys an Amazon API Gateway (API Gateway) RestAPI resource to a stage so
// that clients can call the API over the Internet.  The stage acts as an environment.
type Deployment struct {
	idl.NamedResource
	// restAPI is the RestAPI resource to deploy.
	RestAPI *RestAPI `lumi:"restAPI"`
	// description is a description of the purpose of the API Gateway deployment.
	Description *string `lumi:"description,optional"`
	// stageDescription configures the stage that API Gateway creates with this deployment.
	StageDescription *StageDescription `lumi:"stageDescription,optional"`
	// stageName is a name for the stage that API Gateway creates with this deployment.  Use only alphanumeric
	// characters.
	StageName *string `lumi:"stageName,optional"`
}

type StageDescription struct {
	// Indicates whether cache clustering is enabled for the stage.
	CacheClusterEnabled *bool `lumi:"cacheClusterEnabled,optional"`
	// The size of the stage's cache cluster.
	CacheClusterSize *string `lumi:"cacheClusterSize,optional"`
	// Indicates whether the cached responses are encrypted.
	CacheDataEncrypted *bool `lumi:"cacheDataEncrypted,optional"`
	// The time-to-live (TTL) period, in seconds, that specifies how long API Gateway caches responses.
	CacheTTLInSeconds *float64 `lumi:"cacheTTLInSeconds,optional"`
	// Indicates whether responses are cached and returned for requests. You must enable a cache cluster on the stage
	// to cache responses. For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-caching.html.
	CachingEnabled *bool `lumi:"cachingEnabled,optional"`
	// The client certificate that API Gateway uses to call your integration endpoints in the stage.
	ClientCertificate *ClientCertificate `lumi:"clientCertificate,optional"`
	// Indicates whether data trace logging is enabled for methods in the stage. API Gateway pushes these logs to Amazon
	// CloudWatch Logs.
	DataTraceEnabled *bool `lumi:"dataTraceEnabled,optional"`
	// A description of the purpose of the stage.
	Description *string `lumi:"description,optional"`
	// The logging level for this method.
	LoggingLevel *LoggingLevel `lumi:"loggingLevel,optional"`
	// Configures settings for all of the stage's methods.
	MethodSettings *[]MethodSetting `lumi:"methodSettings,optional"`
	// Indicates whether Amazon CloudWatch metrics are enabled for methods in the stage.
	MetricsEnabled *bool `lumi:"metricsEnabled,optional"`
	// The name of the stage, which API Gateway uses as the first path segment in the invoke URI.
	StageName *string `lumi:"stageName,optional"`
	// The number of burst requests per second that API Gateway permits across all APIs, stages, and methods in your
	// AWS account. For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
	ThrottlingBurstLimit *float64 `lumi:"throttlingBurstLimit,optional"`
	// The number of steady-state requests per second that API Gateway permits across all APIs, stages, and methods in
	// your AWS account. For more information, see
	// http://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-request-throttling.html.
	ThrottlingRateLimit *float64 `lumi:"throttlingRateLimit,optional"`
	// A map that defines the stage variables.  Variable names must consist of alphanumeric characters, and the values
	// must match the following regular expression: `[A-Za-z0-9-._~:/?#&=,]+`.
	Variables *map[string]string `lumi:"variables,optional"`
}
