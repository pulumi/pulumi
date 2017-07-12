// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package apigateway

import (
	"github.com/pulumi/lumi/pkg/resource/idl"

	"github.com/pulumi/lumi/lib/aws/idl/iam"
)

// The Account resource specifies the AWS Identity and Access Management (IAM) role that Amazon API
// Gateway (API Gateway) uses to write API logs to Amazon CloudWatch Logs (CloudWatch Logs).
type Account struct {
	idl.NamedResource
	// CloudWatchRole is the IAM role that has write access to CloudWatch Logs in your account.
	CloudWatchRole *iam.Role `lumi:"cloudWatchRole,optional"`
}
