// Copyright 2016 Marapongo, Inc. All rights reserved.

package awsctx

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/marapongo/mu/pkg/util/contract"
)

// Context represents state shared amongst all parties in this process.  In particular, it wraps an AWS session
// object and offers convenient wrappers for creating connections to the various sub-services (EC2, S3, etc).
type Context struct {
	sess *session.Session
	ec2  *ec2.EC2
}

func New() (*Context, error) {
	// Create an AWS session; note that this is safe to share among many operations.
	// TODO: consider verifying credentials, region, etc. here.
	// TODO: currently we just inherit the standard AWS SDK credentials logic; eventually we will want more
	//     flexibility, I assume, including possibly reading from configuration dynamically.
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	contract.Assert(sess != nil)

	// Allocate a new global context with this session; note that all other connections are lazily allocated.
	return &Context{
		sess: sess,
	}, nil
}

func (ctx *Context) EC2() *ec2.EC2 {
	contract.Assert(ctx.sess != nil)
	if ctx.ec2 == nil {
		ctx.ec2 = ec2.New(ctx.sess)
	}
	return ctx.ec2
}
