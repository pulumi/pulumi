// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"
)

// The Stage resource specifies the AWS Identity and Access Management (IAM) role that Amazon API
// Gateway (API Gateway) uses to write API logs to Amazon CloudWatch Logs (CloudWatch Logs).
type Stage struct {
	idl.NamedResource
	// The RestAPI resource that you're deploying with this stage.
	RestAPI *RestAPI `lumi:"restAPI,replaces"`
	// The name of the stage, which API Gateway uses as the first path segment in the invoke URI.
	StageName string `lumi:"stageName,replaces"`
	// The deployment that the stage points to.
	Deployment *Deployment `lumi:"deployment"`
	// Indicates whether cache clustering is enabled for the stage.
	CacheClusterEnabled *bool `lumi:"cacheClusterEnabled,optional"`
	// The stage's cache cluster size.
	CacheClusterSize *string `lumi:"cacheClusterSize,optional"`
	// The identifier of the client certificate that API Gateway uses to call your integration endpoints in the stage.
	ClientCertificate *ClientCertificate `lumi:"clientCertificate,optional"`
	// A description of the stage's purpose.
	Description *string `lumi:"description,optional"`
	// Settings for all methods in the stage.
	MethodSettings *[]MethodSetting `lumi:"methodSettings,optional"`
	// A map (string to string map) that defines the stage variables, where the variable name is the key and the
	// variable value is the value. Variable names are limited to alphanumeric characters. Values must match the
	// following regular expression: `[A-Za-z0-9-._~:/?#&=,]+`.
	Variables *map[string]string `lumi:"variables,optional"`
}
