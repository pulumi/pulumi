// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"github.com/marapongo/mu/pkg/ast"
	"github.com/marapongo/mu/pkg/compiler/backends/clouds"
	"github.com/marapongo/mu/pkg/diag"
)

// New returns a fresh instance of an AWS Cloud implementation.  This targets "native AWS" for the code-gen outputs.
// This primarily means CloudFormation as the stack templating output, and idiomatic AWS services like S3, DynamoDB,
// Lambda, and so on, for the actual services in those stack templates.
//
// For more details, see: https://github.com/marapongo/mu/blob/master/docs/targets.md#amazon-web-services-aws
func New() clouds.Cloud {
	return &awsCloud{}
}

type awsCloud struct {
	clouds.Cloud
}

func (c *awsCloud) CodeGen(target *ast.Target, doc *diag.Document, stack *ast.Stack) {
}
