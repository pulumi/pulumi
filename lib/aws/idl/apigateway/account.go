// Copyright 2017 Pulumi, Inc. All rights reserved.

package apigateway

import (
	"github.com/pulumi/coconut/pkg/resource/idl"

	"github.com/pulumi/coconut/lib/aws/idl/iam"
)

// The Account resource specifies the AWS Identity and Access Management (IAM) role that Amazon API
// Gateway (API Gateway) uses to write API logs to Amazon CloudWatch Logs (CloudWatch Logs).
type Account struct {
	idl.NamedResource
	// CloudWatchRole is the IAM role that has write access to CloudWatch Logs in your account.
	CloudWatchRole *iam.Role `coco:"cloudWatchRole,optional"`
}
